package event

import (
	"context"
	"time"

	pkgevent "github.com/ilaziness/app-tpl/pkg/event"
)

// 应用生命周期事件名称常量
const (
	// EventAppStarted 应用启动事件
	EventAppStarted = "app.started"
	// EventAppStopped 应用停止事件
	EventAppStopped = "app.stopped"
)

// AppStartedPayload 应用启动事件数据
type AppStartedPayload struct {
	StartTime time.Time
	Version   string
}

// AppStoppedPayload 应用停止事件数据
type AppStoppedPayload struct {
	StopTime time.Time
	Uptime   time.Duration
}

// PublishAppStarted 发布应用启动事件
func PublishAppStarted(ctx context.Context, bus pkgevent.EventBus, version string) error {
	return bus.Publish(ctx, &pkgevent.Event{
		Name:    EventAppStarted,
		Payload: &AppStartedPayload{StartTime: time.Now(), Version: version},
	})
}

// PublishAppStopped 发布应用停止事件
func PublishAppStopped(ctx context.Context, bus pkgevent.EventBus, uptime time.Duration) error {
	return bus.Publish(ctx, &pkgevent.Event{
		Name:    EventAppStopped,
		Payload: &AppStoppedPayload{StopTime: time.Now(), Uptime: uptime},
	})
}
