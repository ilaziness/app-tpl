package cache

import (
	"context"
	"testing"
	"time"
)

func setupTestMultiCache(t *testing.T) (*MultiCache, func()) {
	t.Helper()

	l1Config := MemoryCacheConfig{
		NumCounters: 1000,
		MaxCost:     1024 * 1024, // 1M items
		BufferItems: 64,
	}
	l1, err := NewMemoryCache(l1Config)
	if err != nil {
		t.Fatalf("failed to create memory cache: %v", err)
	}

	_, redisOptions := setupTestRedis(t)
	cacheConfig := RedisCacheConfig{
		KeyPrefix:  "multi:",
		DefaultTTL: time.Hour,
	}
	l2, err := NewRedisCache(cacheConfig, redisOptions)
	if err != nil {
		t.Fatalf("failed to create redis cache: %v", err)
	}

	multiCache := NewMultiCache(l1, l2)
	cleanup := func() { _ = multiCache.Close() }
	t.Cleanup(cleanup)

	return multiCache, cleanup
}

func TestMultiCache_GetFromL1(t *testing.T) {
	cache, _ := setupTestMultiCache(t)
	ctx := context.Background()

	if err := cache.l1.Set(ctx, "l1_key", "l1_value", 5*time.Minute); err != nil {
		t.Fatalf("failed to set value in L1: %v", err)
	}
	value, err := cache.Get(ctx, "l1_key")
	if err != nil {
		t.Fatalf("failed to get value: %v", err)
	}
	if value != "l1_value" {
		t.Errorf("expected l1_value, got %v", value)
	}
}

func TestMultiCache_SetBothLevels(t *testing.T) {
	cache, _ := setupTestMultiCache(t)
	ctx := context.Background()

	if err := cache.Set(ctx, "multi_key", "multi_value", 5*time.Minute); err != nil {
		t.Fatalf("failed to set value: %v", err)
	}

	// L1 stores the raw value.
	value, err := cache.l1.Get(ctx, "multi_key")
	if err != nil {
		t.Errorf("expected value in L1, got error: %v", err)
	}
	if value != "multi_value" {
		t.Errorf("expected multi_value in L1, got %v", value)
	}

	// L2 (Redis) now unmarshals JSON, so the value comes back as a plain
	// string rather than a quoted JSON string.
	value, err = cache.l2.Get(ctx, "multi_key")
	if err != nil {
		t.Errorf("expected value in L2, got error: %v", err)
	}
	if value != "multi_value" {
		t.Errorf("expected multi_value in L2, got %v", value)
	}
}

func TestMultiCache_Delete(t *testing.T) {
	cache, _ := setupTestMultiCache(t)
	ctx := context.Background()

	if err := cache.Set(ctx, "delete_key", "delete_value", 5*time.Minute); err != nil {
		t.Fatalf("failed to set value: %v", err)
	}
	if err := cache.Delete(ctx, "delete_key"); err != nil {
		t.Fatalf("failed to delete value: %v", err)
	}
	if _, err := cache.l1.Get(ctx, "delete_key"); err != ErrCacheMiss {
		t.Error("expected key to be deleted from L1")
	}
	if _, err := cache.l2.Get(ctx, "delete_key"); err != ErrCacheMiss {
		t.Error("expected key to be deleted from L2")
	}
}

func TestMultiCache_Exists(t *testing.T) {
	cache, _ := setupTestMultiCache(t)
	ctx := context.Background()

	if err := cache.Set(ctx, "exists_key", "exists_value", 5*time.Minute); err != nil {
		t.Fatalf("failed to set value: %v", err)
	}
	exists, err := cache.Exists(ctx, "exists_key")
	if err != nil {
		t.Fatalf("failed to check existence: %v", err)
	}
	if !exists {
		t.Error("expected key to exist")
	}
}
