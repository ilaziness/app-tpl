package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
)

// MemoryCacheConfig configures a MemoryCache (Ristretto).
type MemoryCacheConfig struct {
	NumCounters int64 // expected key count * 10
	MaxCost     int64 // maximum number of cached items (each item has cost=1)
	BufferItems int64 // write buffer size, default 64
}

// MemoryCache is an in-process cache backed by Ristretto.
type MemoryCache struct {
	cache  *ristretto.Cache
	config MemoryCacheConfig
	mu     sync.RWMutex // guards the embedded cache instance
}

// NewMemoryCache creates a new MemoryCache.
func NewMemoryCache(config MemoryCacheConfig) (*MemoryCache, error) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: config.NumCounters,
		MaxCost:     config.MaxCost,
		BufferItems: config.BufferItems,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ristretto cache: %w", err)
	}
	return &MemoryCache{
		cache:  cache,
		config: config,
	}, nil
}

// Get implements Cache.
func (c *MemoryCache) Get(ctx context.Context, key string) (any, error) {
	if key == "" {
		return nil, ErrInvalidKey
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, found := c.cache.Get(key)
	if !found {
		return nil, ErrCacheMiss
	}
	return value, nil
}

// Set implements Cache.
// Note: Ristretto writes asynchronously. Wait() blocks until the write is
// processed, guaranteeing that an immediate Get observes the value at the cost
// of throughput. For high-throughput scenarios, consider removing Wait() and
// accepting eventual consistency.
func (c *MemoryCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if key == "" {
		return ErrInvalidKey
	}

	// cost is fixed at 1 so MaxCost semantically limits the number of cached
	// items. This avoids expensive reflection-based size estimation (the
	// previous fmt.Sprintf("%v") approach was both slow and inaccurate for
	// structs/pointers).
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.cache.SetWithTTL(key, value, 1, ttl)
	c.cache.Wait()
	return nil
}

// Delete implements Cache.
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	if key == "" {
		return ErrInvalidKey
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.cache.Del(key)
	return nil
}

// Exists implements Cache.
func (c *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, ErrInvalidKey
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, found := c.cache.Get(key)
	return found, nil
}

// Clear implements Cache. It rebuilds the underlying Ristretto instance.
// The new cache is created first so that, on creation failure, the previous
// instance remains usable.
func (c *MemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	newCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: c.config.NumCounters,
		MaxCost:     c.config.MaxCost,
		BufferItems: c.config.BufferItems,
	})
	if err != nil {
		return fmt.Errorf("failed to recreate cache: %w", err)
	}

	old := c.cache
	c.cache = newCache
	old.Close()
	return nil
}

// Close implements Cache.
func (c *MemoryCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Close()
	return nil
}
