package cache

import (
	"context"
	"errors"
	"sync"
	"time"
)

// MultiCache is a two-level cache (L1: memory, L2: Redis). Get checks L1
// first, then L2, and asynchronously warms L1 on an L2 hit.
type MultiCache struct {
	l1 Cache
	l2 Cache
	wg sync.WaitGroup
}

// NewMultiCache creates a MultiCache from the given L1 and L2 caches.
func NewMultiCache(l1, l2 Cache) *MultiCache {
	return &MultiCache{l1: l1, l2: l2}
}

// Get implements Cache.
func (c *MultiCache) Get(ctx context.Context, key string) (any, error) {
	if value, err := c.l1.Get(ctx, key); err == nil {
		return value, nil
	}

	value, err := c.l2.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	// Asynchronously warm L1. A background context is used intentionally so
	// the warm-up is not tied to the request lifecycle. The warm-up TTL is
	// intentionally shorter than the original Set TTL to limit L1 staleness.
	c.wg.Add(1)
	go func() { //nolint:gosec // intentional fire-and-forget cache warm-up
		defer c.wg.Done()
		warmupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.l1.Set(warmupCtx, key, value, 5*time.Minute)
	}()

	return value, nil
}

// Set implements Cache. Both levels are written; errors from either level are
// joined and returned.
func (c *MultiCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return errors.Join(c.l1.Set(ctx, key, value, ttl), c.l2.Set(ctx, key, value, ttl))
}

// Delete implements Cache. Both levels are deleted; errors from either level
// are joined and returned.
func (c *MultiCache) Delete(ctx context.Context, key string) error {
	return errors.Join(c.l1.Delete(ctx, key), c.l2.Delete(ctx, key))
}

// Exists implements Cache.
func (c *MultiCache) Exists(ctx context.Context, key string) (bool, error) {
	exists, err := c.l1.Exists(ctx, key)
	if err == nil && exists {
		return true, nil
	}
	return c.l2.Exists(ctx, key)
}

// Clear implements Cache. Both levels are cleared; errors from either level
// are joined and returned.
func (c *MultiCache) Clear(ctx context.Context) error {
	return errors.Join(c.l1.Clear(ctx), c.l2.Clear(ctx))
}

// Close implements Cache. It waits for in-flight warm-up goroutines, then
// closes both levels. Errors from both levels are joined and returned.
func (c *MultiCache) Close() error {
	c.wg.Wait()
	return errors.Join(c.l1.Close(), c.l2.Close())
}
