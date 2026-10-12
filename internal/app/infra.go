package app

import (
	"context"
	"fmt"
	"time"

	"github.com/ilaziness/app-tpl/internal/auth"
	"github.com/ilaziness/app-tpl/internal/cache"
	"github.com/ilaziness/app-tpl/internal/config"
	"github.com/ilaziness/app-tpl/internal/event"
	"github.com/ilaziness/app-tpl/internal/logger"
	"github.com/ilaziness/app-tpl/internal/metrics"
	"github.com/ilaziness/app-tpl/internal/tracing"
	"github.com/ilaziness/app-tpl/internal/utils"
	pkgevent "github.com/ilaziness/app-tpl/pkg/event"
	pkglock "github.com/ilaziness/app-tpl/pkg/lock"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
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

	cacheMgr, err := cache.NewManagerFromConfig(a.cfg)
	if err != nil {
		return err
	}
	a.cacheMgr = cacheMgr
	a.addHook(Hook{
		Name:   "cache",
		OnStop: func(ctx context.Context) error { return a.cacheMgr.Close() },
	})

	// Independent Redis client for locks / scheduler (not the cache client).
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

	lockerFactory := pkglock.NewLockerFactory(pkglock.LockerFactoryOptions{
		RedisClient: a.redisClient,
	})
	a.locker = lockerFactory.Create()

	a.eventBus = pkgevent.NewEventBusWithLogger(a.log)
	a.registerBuiltinEventListeners()
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

	return nil
}

func (a *App) registerBuiltinEventListeners() {
	if _, err := a.eventBus.Subscribe(event.EventAppStopped, func(ctx context.Context, ev *pkgevent.Event) error {
		if payload, ok := ev.Payload.(*event.AppStoppedPayload); ok {
			a.log.Info("Application stopped",
				zap.Duration("uptime", payload.Uptime),
				zap.Time("stop_time", payload.StopTime),
			)
		}
		return nil
	}); err != nil && a.log != nil {
		a.log.Warn("Failed to subscribe to app stopped event", zap.Error(err))
	}
}

// newRedisClient creates a Redis client for locks / scheduler (separate from cache).
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
		return nil, fmt.Errorf("failed to connect to Redis for lock/scheduler: %w", err)
	}

	return client, nil
}
