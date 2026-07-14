package app

import (
	"context"

	tcphandler "github.com/ilaziness/app-tpl/internal/handler/tcp"
	"github.com/ilaziness/app-tpl/internal/server"
)

func (a *App) wireTCP() error {
	if !a.cfg.TCP.Enabled {
		return nil
	}

	echoHandler := tcphandler.NewEchoHandler(a.log)
	chatHandler := tcphandler.NewChatHandler(a.log)

	router := tcphandler.NewRouter(a.log, echoHandler, chatHandler)
	router.Default(echoHandler)

	a.tcpServer = server.NewTCPServer(a.cfg, a.log, router)

	a.addHook(Hook{
		Name:   "chat_handler",
		OnStop: func(ctx context.Context) error { chatHandler.Shutdown(); return nil },
	})
	return nil
}
