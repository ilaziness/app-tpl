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
	dispatcher := udphandler.NewDispatcher(a.log, timeHandler, statsHandler)

	a.udpServer = server.NewUDPServer(a.cfg, a.log, dispatcher)
	statsHandler.SetSessionCountFunc(a.udpServer.GetSessionCount)
	return nil
}
