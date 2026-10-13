package app

import (
	"context"
	"fmt"
	"time"

	"github.com/ilaziness/app-tpl/internal/auth"
	"github.com/ilaziness/app-tpl/internal/cache"
	"github.com/ilaziness/app-tpl/internal/config"
	"github.com/ilaziness/app-tpl/internal/constant"
	"github.com/ilaziness/app-tpl/internal/logger"
	"github.com/ilaziness/app-tpl/internal/metrics"
	"github.com/ilaziness/app-tpl/internal/response"
	"github.com/ilaziness/app-tpl/internal/tracing"
	"github.com/ilaziness/app-tpl/internal/utils"
	pkgevent "github.com/ilaziness/app-tpl/pkg/event"
	"github.com/redis/go-redis/v9"
)

func (a *App) wireInfra() error {
	err := logger.Init(logger.Config{
		Level:      a.cfg.Log.Level,
		Format:     a.cfg.Log.Format,
		Output:     a.cfg.Log.Output,
		Filename:   a.cfg.Log.Filename,
		MaxSize:    a.cfg.Log.MaxSize,
		MaxBackups: a.cfg.Log.MaxBackups,
		MaxAge:     a.cfg.Log.MaxAge,
		Compress:   a.cfg.Log.Compress,
	})
	if err != nil {
		return err
	}
	logInst := logger.Log
	a.logger = logInst
	a.log = logInst.Logger

	// Ensure utils.Go panic recovery can log.
	utils.SetLogger(a.log)

	// Response cause details only in development (avoids reading config.Cfg globally).
	response.Configure(a.cfg.App.Env == constant.EnvDev)

	db, err := newDatabase(a.cfg, a.log)
	if err != nil {
		return err
	}
	a.db = db
	a.addHook(Hook{
		Name:   "database",
		OnStop: func(ctx context.Context) error { return a.db.Close() },
	})

	a.jwtMgr = auth.NewJWTManagerFromConfig(a.cfg)

	// Shared Redis client for cache / rate-limit / scheduler / business locks when enabled.
	if a.cfg.Redis.Enabled {
		redisClient, redisErr := newRedisClient(a.cfg)
		if redisErr != nil {
			return redisErr
		}
		a.redisClient = redisClient
		a.addHook(Hook{
			Name:   "redis_client",
			OnStop: func(ctx context.Context) error { return a.redisClient.Close() },
		})
	}

	cacheMgr, err := cache.NewManagerFromConfig(a.cfg, a.redisClient)
	if err != nil {
		return err
	}
	a.cacheMgr = cacheMgr
	a.addHook(Hook{
		Name:   "cache",
		OnStop: func(ctx context.Context) error { return a.cacheMgr.Close() },
	})

	// Business distributed locks (pkg/lock): create via pkglock.NewLockerFactory when a
	// service needs them, e.g. lockerFactory.Create() then inject into that service.
	// Do not eagerly wire an unused process-wide locker here.

	a.eventBus = pkgevent.NewEventBusWithLogger(a.log)
	a.addHook(Hook{
		Name:   "event_bus",
		OnStop: func(ctx context.Context) error { return a.eventBus.Close() },
	})

	tracer, err := tracing.NewTracer(a.cfg)
	if err != nil {
		return err
	}
	a.tracer = tracer
	a.addHook(Hook{
		Name:   "tracer",
		OnStop: func(ctx context.Context) error { return a.tracer.Shutdown(ctx) },
	})

	a.metrics = metrics.NewMetrics(a.cfg)

	a.wireScheduler()

	return nil
}

// newRedisClient creates the shared Redis client used by cache, rate-limit, and scheduler.
func newRedisClient(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:            fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password:        cfg.Redis.Password,
		DB:              cfg.Redis.DB,
		PoolSize:        cfg.Redis.PoolSize,
		MinIdleConns:    cfg.Redis.MinIdleConns,
		ConnMaxIdleTime: time.Duration(cfg.Redis.IdleTimeout) * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return client, nil
}
