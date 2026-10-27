package app

import (
	"context"

	"github.com/ilaziness/app-tpl/internal/scheduler"
	pkgscheduler "github.com/ilaziness/app-tpl/pkg/scheduler"
)

const schedulerLockKey = "app-tpl:scheduler:lock"

func (a *App) wireScheduler() {
	if !a.cfg.Scheduler.Enabled {
		return
	}

	opts := []pkgscheduler.Option{pkgscheduler.WithLogger(a.log)}
	if a.redisClient != nil {
		opts = append(opts, pkgscheduler.WithLock(
			pkgscheduler.NewRedisLock(a.redisClient, schedulerLockKey),
		))
	}
	sched := pkgscheduler.New(opts...)
	sched.Register(scheduler.NewDemoJob(a.log))

	a.addHook(Hook{
		Name: "scheduler",
		OnStart: func(ctx context.Context) error {
			return sched.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return sched.Stop(ctx)
		},
	})
}
