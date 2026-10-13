package cache

import (
	"fmt"

	"github.com/redis/go-redis/v9"
)

// FactoryConfig configures a Factory. It is self-contained and does not
// reference any application config types.
type FactoryConfig struct {
	Enabled bool
	Driver  string // one of "memory" | "redis" | "multi" (ignored when Enabled is false)
	Memory  MemoryCacheConfig
	Redis   RedisCacheConfig
	// RedisOptions is used when RedisClient is nil and Driver is "redis" or "multi".
	RedisOptions RedisOptions
	// RedisClient is an optional shared client. When set, the cache does not own
	// or close it (prefer this in app wiring to avoid dual Redis connections).
	RedisClient *redis.Client
}

// Factory builds a Cache based on FactoryConfig.
type Factory struct {
	cfg FactoryConfig
}

// NewFactory creates a Factory.
func NewFactory(cfg FactoryConfig) *Factory {
	return &Factory{cfg: cfg}
}

func (f *Factory) newRedisLayer() (*RedisCache, error) {
	if f.cfg.RedisClient != nil {
		return NewRedisCacheFromClient(f.cfg.Redis, f.cfg.RedisClient)
	}
	return NewRedisCache(f.cfg.Redis, f.cfg.RedisOptions)
}

// Create returns a Cache instance. When Enabled is false, a NopCache is
// returned (every Get yields ErrCacheMiss, so callers fall back to source).
func (f *Factory) Create() (Cache, error) {
	if !f.cfg.Enabled {
		return NewNopCache(), nil
	}

	switch f.cfg.Driver {
	case "memory":
		return NewMemoryCache(f.cfg.Memory)
	case "redis":
		return f.newRedisLayer()
	case "multi":
		l1, err := NewMemoryCache(f.cfg.Memory)
		if err != nil {
			return nil, err
		}
		l2, err := f.newRedisLayer()
		if err != nil {
			_ = l1.Close()
			return nil, err
		}
		return NewMultiCache(l1, l2), nil
	default:
		return nil, fmt.Errorf("unsupported cache driver: %s", f.cfg.Driver)
	}
}
