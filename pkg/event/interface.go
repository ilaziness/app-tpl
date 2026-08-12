package event

import (
	"context"
)

// EventBus 事件总线接口
type EventBus interface {
	// Publish 发布事件
	Publish(ctx context.Context, event *Event) error

	// Subscribe 订阅事件，返回取消订阅函数
	Subscribe(eventName string, handler EventHandler, opts ...SubscriptionOptions) (func(), error)

	// SubscribeWildcard 通配符订阅（支持 "user.*" 匹配 "user.created"），返回取消订阅函数
	SubscribeWildcard(pattern string, handler EventHandler, opts ...SubscriptionOptions) (func(), error)

	// Close 关闭事件总线
	Close() error
}
