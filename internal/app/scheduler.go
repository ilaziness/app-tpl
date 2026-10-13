package app

import (
	"context"

	"github.com/ilaziness/app-tpl/internal/scheduler"
)

func (a *App) wireScheduler() {
	if !a.cfg.Scheduler.Enabled {
		return
	}

	sched := scheduler.NewScheduler(a.redisClient, a.log)
	if a.cfg.Scheduler.RegisterDemoJob {
		sched.Register(scheduler.NewDemoJob(a.log))
	}
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
