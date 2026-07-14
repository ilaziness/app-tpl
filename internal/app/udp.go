package app

import (
	udphandler "github.com/ilaziness/app-tpl/internal/handler/udp"
	"github.com/ilaziness/app-tpl/internal/server"
)

func (a *App) wireUDP() error {
	if !a.cfg.UDP.Enabled {
		return nil
	}

	timeHandler := udphandler.NewTimeHandler(a.log)
	statsHandler := udphandler.NewStatsHandler(a.log)

	router := udphandler.NewRouter(a.log, timeHandler, statsHandler)

	a.udpServer = server.NewUDPServer(a.cfg, a.log, router)
	statsHandler.SetSessionCountFunc(a.udpServer.GetSessionCount)
	return nil
}
