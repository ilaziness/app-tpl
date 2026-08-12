// Package cache provides reusable cache primitives (interface, memory/redis/multi/nop
// implementations, factory, errors, key generator). It is self-contained and does
// not depend on the application's internal/config package.
package cache

import (
	"context"
	"time"
)

// Cache defines the cache interface.
type Cache interface {
	// Get retrieves the cached value for the given key.
	// Returns ErrCacheMiss when the key does not exist.
	Get(ctx context.Context, key string) (any, error)

	// Set stores the value with the given TTL. If ttl <= 0, an implementation-
	// specific default TTL is used.
	Set(ctx context.Context, key string, value any, ttl time.Duration) error

	// Delete removes the value for the given key. No error is returned when the
	// key does not exist.
	Delete(ctx context.Context, key string) error

	// Exists reports whether the key exists in the cache.
	Exists(ctx context.Context, key string) (bool, error)

	// Clear removes all keys. Intended for tests/development only; use with
	// caution in production.
	Clear(ctx context.Context) error

	// Close releases resources held by the cache (connections, goroutines, etc.).
	Close() error
}
