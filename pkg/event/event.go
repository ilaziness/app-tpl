package event

import (
	"context"
	"time"
)

// Event 事件基础结构
type Event struct {
	Name      string    // 事件名称
	Payload   any       // 事件数据
	Timestamp time.Time // 事件时间戳（零值时由总线在发布时填充）
}

// EventHandler 事件处理器
type EventHandler func(ctx context.Context, event *Event) error

// HandlerPriority 处理器优先级。
//
// 零值（PriorityNormal）即默认优先级，SubscriptionOptions 无需显式设置。
type HandlerPriority int

const (
	// PriorityLow 低优先级
	PriorityLow HandlerPriority = iota
	// PriorityNormal 普通优先级（默认）
	PriorityNormal
	// PriorityHigh 高优先级
	PriorityHigh
)

// SubscriptionOptions 订阅选项
type SubscriptionOptions struct {
	Async    bool            // 是否异步处理
	Priority HandlerPriority // 优先级（零值为普通优先级）
}
