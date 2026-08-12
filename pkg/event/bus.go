package event

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// maxSubscribersPerEvent 每个事件的最大订阅者数量
	maxSubscribersPerEvent = 1000
)

type subscriber struct {
	handler  EventHandler
	async    bool
	priority HandlerPriority
}

// bus 是 EventBus 的默认实现。
type bus struct {
	mu          sync.RWMutex
	subscribers map[string][]*subscriber
	patterns    map[string][]*subscriber // 通配符订阅
	wg          sync.WaitGroup
	closed      bool
	logger      *zap.Logger
}

// NewEventBus 创建新的事件总线。
func NewEventBus() EventBus {
	return NewEventBusWithLogger(zap.NewNop())
}

// NewEventBusWithLogger 创建新的事件总线，使用指定的 logger。
func NewEventBusWithLogger(logger *zap.Logger) EventBus {
	return &bus{
		subscribers: make(map[string][]*subscriber),
		patterns:    make(map[string][]*subscriber),
		logger:      logger,
	}
}

// Publish 发布事件。
//
// 锁内仅收集订阅者快照并对异步订阅者登记 WaitGroup，锁外执行 handler，
// 避免 handler 内的订阅/取消订阅造成死锁，也避免长耗时 handler 阻塞
// 其他发布与订阅；同时保证 Close 能等待所有已入队的异步任务。
func (b *bus) Publish(ctx context.Context, event *Event) error {
	// 时间戳未显式设置时统一由总线填充
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return fmt.Errorf("publish %q: %w", event.Name, ErrClosed)
	}
	targets := make([]*subscriber, 0)
	if subs, ok := b.subscribers[event.Name]; ok {
		targets = append(targets, subs...)
	}
	// 通配符匹配订阅者
	for pattern, subs := range b.patterns {
		if b.matchPatternLocked(pattern, event.Name) {
			targets = append(targets, subs...)
		}
	}
	// 在持锁阶段登记异步任务，确保 Close 的 Wait 覆盖这些任务
	for _, sub := range targets {
		if sub.async {
			b.wg.Add(1)
		}
	}
	b.mu.RUnlock()

	for _, sub := range targets {
		b.dispatch(ctx, event, sub)
	}
	return nil
}

// dispatch 分发事件到单个订阅者。异步订阅者的 WaitGroup 已在 Publish 持锁
// 阶段登记，此处仅启动 goroutine。
func (b *bus) dispatch(ctx context.Context, event *Event, sub *subscriber) {
	if !sub.async {
		b.runHandler(ctx, event, sub.handler)
		return
	}
	go func() {
		defer b.wg.Done()
		b.runHandler(ctx, event, sub.handler)
	}()
}

// runHandler 执行 handler，捕获 panic 并记录错误但不中断其他 handler。
func (b *bus) runHandler(ctx context.Context, event *Event, handler EventHandler) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Warn("event handler panic", zap.String("event", event.Name), zap.Any("recover", r))
		}
	}()
	if err := handler(ctx, event); err != nil {
		b.logger.Warn("event handler error", zap.String("event", event.Name), zap.Error(err))
	}
}

// resolveOptions 应用订阅选项，未显式设置时使用默认值。
//
// 零值 SubscriptionOptions 即同步 + 普通优先级，可直接使用。
func resolveOptions(opts ...SubscriptionOptions) SubscriptionOptions {
	if len(opts) > 0 {
		return opts[0]
	}
	return SubscriptionOptions{}
}

// Subscribe 订阅事件，返回取消订阅函数。
func (b *bus) Subscribe(eventName string, handler EventHandler, opts ...SubscriptionOptions) (func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, fmt.Errorf("subscribe %q: %w", eventName, ErrClosed)
	}

	// 检查订阅者数量限制
	if len(b.subscribers[eventName]) >= maxSubscribersPerEvent {
		return nil, fmt.Errorf("subscribe %q: %w", eventName, ErrMaxSubscribers)
	}

	opt := resolveOptions(opts...)
	sub := &subscriber{
		handler:  handler,
		async:    opt.Async,
		priority: opt.Priority,
	}

	b.subscribers[eventName] = append(b.subscribers[eventName], sub)
	// 稳定排序（降序）：高优先级先执行，同优先级保持注册顺序
	sort.SliceStable(b.subscribers[eventName], func(i, j int) bool {
		return b.subscribers[eventName][i].priority > b.subscribers[eventName][j].priority
	})

	return b.unsubscribeFunc(eventName, sub), nil
}

// unsubscribeFunc 返回移除指定订阅者的闭包。
func (b *bus) unsubscribeFunc(eventName string, sub *subscriber) func() {
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.subscribers[eventName]
		for i, s := range subs {
			if s == sub {
				b.subscribers[eventName] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}
}

// SubscribeWildcard 通配符订阅（支持 "user.*" 匹配 "user.created"），返回取消订阅函数。
func (b *bus) SubscribeWildcard(pattern string, handler EventHandler, opts ...SubscriptionOptions) (func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, fmt.Errorf("subscribe wildcard %q: %w", pattern, ErrClosed)
	}

	// 验证模式有效性
	if _, err := filepath.Match(pattern, "test"); err != nil {
		return nil, fmt.Errorf("subscribe wildcard %q: %w: %v", pattern, ErrInvalidPattern, err)
	}

	opt := resolveOptions(opts...)
	sub := &subscriber{
		handler:  handler,
		async:    opt.Async,
		priority: opt.Priority,
	}

	b.patterns[pattern] = append(b.patterns[pattern], sub)
	// 稳定排序（降序）：高优先级先执行，同优先级保持注册顺序
	sort.SliceStable(b.patterns[pattern], func(i, j int) bool {
		return b.patterns[pattern][i].priority > b.patterns[pattern][j].priority
	})

	return b.unsubscribeWildcardFunc(pattern, sub), nil
}

// unsubscribeWildcardFunc 返回移除指定通配符订阅者的闭包。
func (b *bus) unsubscribeWildcardFunc(pattern string, sub *subscriber) func() {
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.patterns[pattern]
		for i, s := range subs {
			if s == sub {
				b.patterns[pattern] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}
}

// matchPatternLocked 检查事件名是否匹配模式（调用方需持有读锁）。
func (b *bus) matchPatternLocked(pattern, eventName string) bool {
	// 使用 filepath.Match 支持通配符
	matched, err := filepath.Match(pattern, eventName)
	if err != nil {
		b.logger.Warn("invalid pattern", zap.String("pattern", pattern), zap.Error(err))
		return false
	}
	return matched
}

// Close 关闭事件总线，等待所有已入队的异步任务完成。
func (b *bus) Close() error {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()

	b.wg.Wait()
	return nil
}
