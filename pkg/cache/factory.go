package cache

import (
	"fmt"
)

// FactoryConfig configures a Factory. It is self-contained and does not
// reference any application config types.
type FactoryConfig struct {
	Enabled bool
	Driver  string // one of "memory" | "redis" | "multi" (ignored when Enabled is false)
	Memory  MemoryCacheConfig
	Redis   RedisCacheConfig
	// RedisOptions is required when Driver is "redis" or "multi".
	RedisOptions RedisOptions
}

// Factory builds a Cache based on FactoryConfig.
type Factory struct {
	cfg FactoryConfig
}

// NewFactory creates a Factory.
func NewFactory(cfg FactoryConfig) *Factory {
	return &Factory{cfg: cfg}
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
		return NewRedisCache(f.cfg.Redis, f.cfg.RedisOptions)
	case "multi":
		l1, err := NewMemoryCache(f.cfg.Memory)
		if err != nil {
			return nil, err
		}
		l2, err := NewRedisCache(f.cfg.Redis, f.cfg.RedisOptions)
		if err != nil {
			_ = l1.Close()
			return nil, err
		}
		return NewMultiCache(l1, l2), nil
	default:
		return nil, fmt.Errorf("unsupported cache driver: %s", f.cfg.Driver)
	}
}
