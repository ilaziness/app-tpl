package app

import (
	"context"

	httphandler "github.com/ilaziness/app-tpl/internal/handler/http"
	"github.com/ilaziness/app-tpl/internal/health"
	"github.com/ilaziness/app-tpl/internal/logger"
	"github.com/ilaziness/app-tpl/internal/repository"
	"github.com/ilaziness/app-tpl/internal/router"
	"github.com/ilaziness/app-tpl/internal/scheduler"
	"github.com/ilaziness/app-tpl/internal/server"
	"github.com/ilaziness/app-tpl/internal/service"
)

func (a *App) wireHTTP() error {
	userRepo := repository.NewUserRepo(a.db)
	userSvc := service.NewUserService(userRepo, a.cacheMgr)

	if a.cfg.Scheduler.Enabled {
		sched := scheduler.NewScheduler(a.redisClient)
		if a.cfg.Scheduler.RegisterDemoJob {
			sched.Register(scheduler.NewDemoJob())
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

	if !a.cfg.HTTP.Enabled {
		return nil
	}

	healthHandler := httphandler.NewHealthHandler(a.cfg)
	healthHandler.AddChecker(health.NewDatabaseChecker(a.db))

	handlers, err := router.NewHandlers(healthHandler, httphandler.NewUserHandler(userSvc))
	if err != nil {
		return err
	}
	handlers.InternalServiceKey = a.cfg.HTTP.InternalServiceKey
	// Admin RequireAuth only when JWT is configured; otherwise sample admin APIs stay callable.
	handlers.RequireAdminAuth = a.jwtMgr != nil

	accessLogger := logger.NewStdoutLogger(a.cfg.Log.Level)
	httpServer, err := server.NewHTTPServer(a.cfg, a.log, accessLogger, handlers, a.metrics, a.jwtMgr, a.redisClient)
	if err != nil {
		return err
	}
	a.httpServer = httpServer
	return nil
}
