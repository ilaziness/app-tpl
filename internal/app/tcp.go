package app

import (
	tcphandler "github.com/ilaziness/app-tpl/internal/handler/tcp"
	"github.com/ilaziness/app-tpl/internal/server"
)

func (a *App) wireTCP() error {
	if !a.cfg.TCP.Enabled {
		return nil
	}

	echoHandler := tcphandler.NewEchoHandler(a.log)
	a.tcpServer = server.NewTCPServer(a.cfg, a.log, echoHandler)
	return nil
}
