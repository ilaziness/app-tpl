package cache

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/ilaziness/app-tpl/internal/model"
	pkgcache "github.com/ilaziness/app-tpl/pkg/cache"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	c, err := pkgcache.NewMemoryCache(pkgcache.MemoryCacheConfig{
		NumCounters: 1000,
		MaxCost:     1 << 20,
		BufferItems: 64,
	})
	if err != nil {
		t.Fatalf("failed to create memory cache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return NewManager(c)
}

func newTestUser(id int64) *model.User {
	return &model.User{
		ID:        id,
		Email:     "alice@example.com",
		Name:      "Alice",
		Phone:     "13800000000",
		Status:    1,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// setupTestRedisOptions starts a miniredis instance and returns the
// pkg/cache.RedisOptions pointing at it.
func setupTestRedisOptions(t *testing.T) pkgcache.RedisOptions {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(func() { mr.Close() })
	port := 0
	if p, err := strconv.Atoi(mr.Port()); err == nil {
		port = p
	}
	return pkgcache.RedisOptions{
		Addr:     fmt.Sprintf("%s:%d", mr.Host(), port),
		PoolSize: 10,
	}
}
