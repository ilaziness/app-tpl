package cache

import (
	"fmt"
	"time"

	"github.com/ilaziness/app-tpl/internal/config"
	pkgcache "github.com/ilaziness/app-tpl/pkg/cache"
	"github.com/redis/go-redis/v9"
)

// NewManagerFromConfig builds a Manager from the application config. It maps
// the Viper-loaded config structs to pkg/cache's self-contained config types,
// keeping pkg/cache decoupled from internal/config.
//
// redisClient is optional. When the cache driver needs Redis and redisClient
// is non-nil, the shared client is reused (App owns Close). When nil and Redis
// is required, pkg/cache creates its own connection from cfg.Redis.
func NewManagerFromConfig(cfg *config.Config, redisClient *redis.Client) (*Manager, error) {
	factoryCfg := pkgcache.FactoryConfig{
		Enabled: cfg.Cache.Enabled,
		Driver:  cfg.Cache.Driver,
		Memory: pkgcache.MemoryCacheConfig{
			NumCounters: cfg.Cache.Memory.NumCounters,
			MaxCost:     cfg.Cache.Memory.MaxCost,
			BufferItems: cfg.Cache.Memory.BufferItems,
		},
		Redis: pkgcache.RedisCacheConfig{
			KeyPrefix:  cfg.Cache.Redis.KeyPrefix,
			DefaultTTL: time.Duration(cfg.Cache.Redis.DefaultTTL) * time.Second,
		},
		RedisClient: redisClient,
	}
	if redisClient == nil && cfg.Redis.Enabled {
		factoryCfg.RedisOptions = pkgcache.RedisOptions{
			Addr:            fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
			Password:        cfg.Redis.Password,
			DB:              cfg.Redis.DB,
			PoolSize:        cfg.Redis.PoolSize,
			MinIdleConns:    cfg.Redis.MinIdleConns,
			ConnMaxIdleTime: time.Duration(cfg.Redis.IdleTimeout) * time.Second,
		}
	}

	c, err := pkgcache.NewFactory(factoryCfg).Create()
	if err != nil {
		return nil, err
	}
	return NewManager(c), nil
}
