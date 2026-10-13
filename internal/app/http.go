package app

import (
	httphandler "github.com/ilaziness/app-tpl/internal/handler/http"
	"github.com/ilaziness/app-tpl/internal/health"
	"github.com/ilaziness/app-tpl/internal/logger"
	"github.com/ilaziness/app-tpl/internal/repository"
	"github.com/ilaziness/app-tpl/internal/router"
	"github.com/ilaziness/app-tpl/internal/server"
	"github.com/ilaziness/app-tpl/internal/service"
)

func (a *App) wireHTTP() error {
	if !a.cfg.HTTP.Enabled {
		return nil
	}

	userRepo := repository.NewUserRepo(a.db)
	userSvc := service.NewUserService(userRepo, a.cacheMgr)

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
	httpServer, err := server.NewHTTPServer(server.HTTPServerDeps{
		Cfg:          a.cfg,
		Logger:       a.log,
		AccessLogger: accessLogger,
		Handlers:     handlers,
		Metrics:      a.metrics,
		JWT:          a.jwtMgr,
		Redis:        a.redisClient,
	})
	if err != nil {
		return err
	}
	a.httpServer = httpServer
	return nil
}
