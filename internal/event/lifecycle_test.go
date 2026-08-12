package event

import (
	"context"
	"testing"
	"time"

	pkgevent "github.com/ilaziness/app-tpl/pkg/event"
)

func TestLifecycleEvents(t *testing.T) {
	bus := pkgevent.NewEventBus()
	defer bus.Close()

	appStarted := false
	appStopped := false

	// 订阅应用启动事件
	_, _ = bus.Subscribe(EventAppStarted, func(ctx context.Context, e *pkgevent.Event) error {
		if payload, ok := e.Payload.(*AppStartedPayload); ok {
			appStarted = true
			if payload.Version == "" {
				t.Error("expected version in AppStartedPayload")
			}
		}
		return nil
	})

	// 订阅应用停止事件
	_, _ = bus.Subscribe(EventAppStopped, func(ctx context.Context, e *pkgevent.Event) error {
		if payload, ok := e.Payload.(*AppStoppedPayload); ok {
			appStopped = true
			if payload.Uptime == 0 {
				t.Error("expected uptime in AppStoppedPayload")
			}
		}
		return nil
	})

	// 发布内置事件
	_ = PublishAppStarted(context.Background(), bus, "1.0.0")
	_ = PublishAppStopped(context.Background(), bus, 5*time.Second)

	// 等待执行
	time.Sleep(100 * time.Millisecond)

	if !appStarted {
		t.Error("AppStarted handler was not called")
	}
	if !appStopped {
		t.Error("AppStopped handler was not called")
	}
}
