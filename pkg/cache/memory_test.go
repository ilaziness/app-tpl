package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func newTestMemoryCache() *MemoryCache {
	config := MemoryCacheConfig{
		NumCounters: 1000,
		MaxCost:     1024 * 1024, // 1M items (cost=1 per item)
		BufferItems: 64,
	}
	cache, err := NewMemoryCache(config)
	if err != nil {
		panic(err)
	}
	return cache
}

func TestMemoryCache_SetAndGet(t *testing.T) {
	cache := newTestMemoryCache()
	defer cache.Close()

	ctx := context.Background()

	t.Run("Set and Get string value", func(t *testing.T) {
		if err := cache.Set(ctx, "test_key", "test_value", 5*time.Minute); err != nil {
			t.Fatalf("failed to set value: %v", err)
		}
		value, err := cache.Get(ctx, "test_key")
		if err != nil {
			t.Fatalf("failed to get value: %v", err)
		}
		if value != "test_value" {
			t.Errorf("expected test_value, got %v", value)
		}
	})

	t.Run("Set and Get int value", func(t *testing.T) {
		if err := cache.Set(ctx, "int_key", int64(42), 5*time.Minute); err != nil {
			t.Fatalf("failed to set value: %v", err)
		}
		value, err := cache.Get(ctx, "int_key")
		if err != nil {
			t.Fatalf("failed to get value: %v", err)
		}
		if value != int64(42) {
			t.Errorf("expected 42, got %v", value)
		}
	})

	t.Run("Get non-existent key", func(t *testing.T) {
		_, err := cache.Get(ctx, "non_existent")
		if err != ErrCacheMiss {
			t.Errorf("expected ErrCacheMiss, got %v", err)
		}
	})
}

func TestMemoryCache_Delete(t *testing.T) {
	cache := newTestMemoryCache()
	defer cache.Close()
	ctx := context.Background()

	if err := cache.Set(ctx, "delete_key", "delete_value", 5*time.Minute); err != nil {
		t.Fatalf("failed to set value: %v", err)
	}
	if err := cache.Delete(ctx, "delete_key"); err != nil {
		t.Fatalf("failed to delete value: %v", err)
	}
	if _, err := cache.Get(ctx, "delete_key"); err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss after delete, got %v", err)
	}
}

func TestMemoryCache_Exists(t *testing.T) {
	cache := newTestMemoryCache()
	defer cache.Close()
	ctx := context.Background()

	t.Run("Key exists", func(t *testing.T) {
		if err := cache.Set(ctx, "exists_key", "value", 5*time.Minute); err != nil {
			t.Fatalf("failed to set value: %v", err)
		}
		exists, err := cache.Exists(ctx, "exists_key")
		if err != nil {
			t.Fatalf("failed to check existence: %v", err)
		}
		if !exists {
			t.Error("expected key to exist")
		}
	})

	t.Run("Key does not exist", func(t *testing.T) {
		exists, err := cache.Exists(ctx, "not_exists_key")
		if err != nil {
			t.Fatalf("failed to check existence: %v", err)
		}
		if exists {
			t.Error("expected key to not exist")
		}
	})
}

func TestMemoryCache_TTL(t *testing.T) {
	cache := newTestMemoryCache()
	defer cache.Close()
	ctx := context.Background()

	if err := cache.Set(ctx, "ttl_key", "ttl_value", 100*time.Millisecond); err != nil {
		t.Fatalf("failed to set value: %v", err)
	}
	exists, err := cache.Exists(ctx, "ttl_key")
	if err != nil {
		t.Fatalf("failed to check existence: %v", err)
	}
	if !exists {
		t.Error("expected key to exist immediately after set")
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := cache.Get(ctx, "ttl_key"); err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss after TTL expiration, got %v", err)
	}
}

func TestMemoryCache_Clear(t *testing.T) {
	cache := newTestMemoryCache()
	defer cache.Close()
	ctx := context.Background()

	_ = cache.Set(ctx, "key1", "value1", 5*time.Minute)
	_ = cache.Set(ctx, "key2", "value2", 5*time.Minute)
	_ = cache.Set(ctx, "key3", "value3", 5*time.Minute)

	if err := cache.Clear(ctx); err != nil {
		t.Fatalf("failed to clear cache: %v", err)
	}
	for _, key := range []string{"key1", "key2", "key3"} {
		if _, err := cache.Get(ctx, key); err != ErrCacheMiss {
			t.Errorf("expected %s to be cleared", key)
		}
	}
}

func TestMemoryCache_ConcurrentAccess(t *testing.T) {
	cache := newTestMemoryCache()
	defer cache.Close()
	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("concurrent_key_%d", n)
			_ = cache.Set(ctx, key, n, 5*time.Minute)
		}(i)
	}
	wg.Wait()

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("concurrent_key_%d", n)
			_, _ = cache.Get(ctx, key)
		}(i)
	}
	wg.Wait()
}
