package app

import (
	httphandler "github.com/example/app-tpl/internal/handler/http"
	"github.com/example/app-tpl/internal/health"
	"github.com/example/app-tpl/internal/repository"
	"github.com/example/app-tpl/internal/router"
	"github.com/example/app-tpl/internal/server"
	"github.com/example/app-tpl/internal/service"
)

func (a *App) wireHTTP() error {
	userRepo := repository.NewUserRepo(a.db)
	userSvc := service.NewUserService(userRepo)

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

	httpServer, err := server.NewHTTPServer(a.cfg, a.log, handlers, a.metrics, a.jwtMgr)
	if err != nil {
		return err
	}
	a.httpServer = httpServer
	return nil
}
