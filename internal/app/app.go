// Package app wires dependencies and manages application lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	stdhttp "net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/example/app-tpl/internal/auth"
	"github.com/example/app-tpl/internal/cache"
	"github.com/example/app-tpl/internal/config"
	"github.com/example/app-tpl/internal/database"
	"github.com/example/app-tpl/internal/event"
	"github.com/example/app-tpl/internal/logger"
	"github.com/example/app-tpl/internal/metrics"
	"github.com/example/app-tpl/internal/server"
	"github.com/example/app-tpl/internal/tracing"
	"go.uber.org/zap"
)

// App holds wired dependencies and manages server lifecycle.
type App struct {
	cfg    *config.Config
	log    *zap.Logger
	logger *logger.Logger

	db       *database.DB
	cache    cache.Cache
	eventBus event.EventBus
	tracer   *tracing.Tracer
	metrics  *metrics.Metrics
	jwtMgr   *auth.JWTManager

	httpServer *server.HTTPServer
	tcpServer  *server.TCPServer
	udpServer  *server.UDPServer

	hooks      []Hook
	hookStates []hookState
	httpFailed atomic.Bool
	appStarted bool
	startTime  time.Time
}

// New creates and wires the application.
func New(cfg *config.Config) (*App, error) {
	a := &App{cfg: cfg}

	if err := a.wireInfra(); err != nil {
		a.shutdownBestEffort()
		return nil, err
	}
	if err := a.wireHTTP(); err != nil {
		a.shutdownBestEffort()
		return nil, err
	}
	if err := a.wireTCP(); err != nil {
		a.shutdownBestEffort()
		return nil, err
	}
	if err := a.wireUDP(); err != nil {
		a.shutdownBestEffort()
		return nil, err
	}

	return a, nil
}

// Run starts enabled servers and blocks until shutdown signal or fatal error.
func (a *App) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startupCtx, startupCancel := context.WithTimeout(context.Background(), a.maxShutdownTimeout())
	defer startupCancel()
	if err := a.startHooks(startupCtx); err != nil {
		a.shutdownBestEffort()
		return fmt.Errorf("startup failed: %w", err)
	}

	if err := a.logStartup(); err != nil {
		a.shutdownBestEffort()
		return err
	}
	a.appStarted = true
	a.startTime = time.Now()

	if err := a.startServers(ctx, stop); err != nil {
		stop()
		a.shutdownBestEffort()
		return err
	}

	<-ctx.Done()

	if err := a.shutdown(); err != nil {
		return err
	}

	if a.httpFailed.Load() {
		return fmt.Errorf("HTTP server exited with error")
	}

	return nil
}

func (a *App) logStartup() error {
	fields := []zap.Field{
		zap.String("name", a.cfg.App.Name),
		zap.String("version", a.cfg.App.Version),
		zap.String("env", a.cfg.App.Env),
	}
	if a.cfg.HTTP.Enabled {
		fields = append(fields, zap.String("http_addr", addrOf(a.cfg.HTTP.Host, a.cfg.HTTP.Port)))
	}
	if a.cfg.TCP.Enabled {
		fields = append(fields, zap.String("tcp_addr", addrOf(a.cfg.TCP.Host, a.cfg.TCP.Port)))
	}
	if a.cfg.UDP.Enabled {
		fields = append(fields, zap.String("udp_addr", addrOf(a.cfg.UDP.Host, a.cfg.UDP.Port)))
	}
	a.log.Info("Application started", fields...)

	return event.PublishAppStarted(a.eventBus, a.cfg.App.Version)
}

func (a *App) startServers(ctx context.Context, stop context.CancelFunc) error {
	if a.tcpServer != nil && a.tcpServer.Enabled() {
		if err := a.tcpServer.Start(ctx); err != nil {
			return fmt.Errorf("failed to start TCP server: %w", err)
		}
	}

	if a.udpServer != nil && a.udpServer.Enabled() {
		if err := a.udpServer.Start(ctx); err != nil {
			if a.tcpServer != nil && a.tcpServer.Enabled() {
				stopCtx, cancel := context.WithTimeout(context.Background(), a.cfg.TCP.GetShutdownTimeout())
				if stopErr := a.tcpServer.Stop(stopCtx); stopErr != nil && a.log != nil {
					a.log.Error("Failed to stop TCP server after UDP start failure", zap.Error(stopErr))
				}
				cancel()
			}
			return fmt.Errorf("failed to start UDP server: %w", err)
		}
	}

	if a.httpServer != nil && a.httpServer.Enabled() {
		a.log.Info("Starting HTTP server", zap.String("addr", a.httpServer.Addr()))
		go func() {
			if err := a.httpServer.Serve(); err != nil && err != stdhttp.ErrServerClosed {
				a.log.Error("HTTP server exited with error", zap.Error(err))
				a.httpFailed.Store(true)
				stop()
			}
		}()
	}

	return nil
}

func (a *App) shutdownBestEffort() {
	if err := a.shutdown(); err != nil && a.log != nil {
		a.log.Error("Shutdown failed", zap.Error(err))
	}
}

func (a *App) shutdown() error {
	var errs []error

	if a.httpServer != nil && a.httpServer.Enabled() {
		httpCtx, cancel := context.WithTimeout(context.Background(), a.cfg.HTTP.GetShutdownTimeout())
		if a.log != nil {
			a.log.Info("Stopping HTTP server")
		}
		if err := a.httpServer.Shutdown(httpCtx); err != nil {
			if a.log != nil {
				a.log.Error("Failed to shutdown HTTP server", zap.Error(err))
			}
			errs = append(errs, fmt.Errorf("http shutdown: %w", err))
		} else if a.log != nil {
			a.log.Info("HTTP server stopped")
		}
		cancel()
	}

	if a.tcpServer != nil && a.tcpServer.Enabled() {
		tcpCtx, cancel := context.WithTimeout(context.Background(), a.cfg.TCP.GetShutdownTimeout())
		if err := a.tcpServer.Stop(tcpCtx); err != nil {
			if a.log != nil {
				a.log.Error("Failed to stop TCP server", zap.Error(err))
			}
			errs = append(errs, fmt.Errorf("tcp stop: %w", err))
		}
		cancel()
	}

	if a.udpServer != nil && a.udpServer.Enabled() {
		udpCtx, cancel := context.WithTimeout(context.Background(), a.cfg.UDP.GetShutdownTimeout())
		if err := a.udpServer.Stop(udpCtx); err != nil {
			if a.log != nil {
				a.log.Error("Failed to stop UDP server", zap.Error(err))
			}
			errs = append(errs, fmt.Errorf("udp stop: %w", err))
		}
		cancel()
	}

	if a.appStarted && a.eventBus != nil {
		uptime := time.Since(a.startTime)
		if err := event.PublishAppStopped(a.eventBus, uptime); err != nil {
			if a.log != nil {
				a.log.Error("Failed to publish app stopped event", zap.Error(err))
			}
			errs = append(errs, fmt.Errorf("publish app stopped: %w", err))
		}
	}

	infraCtx, cancel := context.WithTimeout(context.Background(), a.maxShutdownTimeout())
	defer cancel()

	if err := a.stopHooks(infraCtx); err != nil {
		errs = append(errs, err)
	}

	if a.logger != nil {
		if a.log != nil {
			a.log.Info("Application stopped")
		}
		_ = a.logger.Sync()
	}

	return errors.Join(errs...)
}

func (a *App) maxShutdownTimeout() time.Duration {
	maxTimeout := 30 * time.Second
	if a.cfg.HTTP.Enabled {
		if d := a.cfg.HTTP.GetShutdownTimeout(); d > maxTimeout {
			maxTimeout = d
		}
	}
	if a.cfg.TCP.Enabled {
		if d := a.cfg.TCP.GetShutdownTimeout(); d > maxTimeout {
			maxTimeout = d
		}
	}
	if a.cfg.UDP.Enabled {
		if d := a.cfg.UDP.GetShutdownTimeout(); d > maxTimeout {
			maxTimeout = d
		}
	}
	return maxTimeout
}

func addrOf(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}

func newDatabase(cfg *config.Config, log *zap.Logger) (*database.DB, error) {
	return database.NewDB(cfg, log)
}
