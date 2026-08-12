package cache

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, RedisOptions) {
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

	redisOptions := RedisOptions{
		Addr:         fmt.Sprintf("%s:%d", mr.Host(), port),
		PoolSize:     10,
		MinIdleConns: 0,
	}
	return mr, redisOptions
}

func TestRedisCache_SetAndGet(t *testing.T) {
	_, redisOptions := setupTestRedis(t)

	cacheConfig := RedisCacheConfig{
		KeyPrefix:  "test:",
		DefaultTTL: time.Hour,
	}

	c, err := NewRedisCache(cacheConfig, redisOptions)
	if err != nil {
		t.Fatalf("failed to create redis cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	t.Run("Set and Get string value", func(t *testing.T) {
		if err := c.Set(ctx, "test_key", "test_value", 5*time.Minute); err != nil {
			t.Fatalf("failed to set value: %v", err)
		}
		// Redis.Get now unmarshals JSON, so a stored string comes back as a
		// plain string (not a quoted JSON string).
		value, err := c.Get(ctx, "test_key")
		if err != nil {
			t.Fatalf("failed to get value: %v", err)
		}
		if value != "test_value" {
			t.Errorf("expected test_value, got %v", value)
		}
	})

	t.Run("Get non-existent key", func(t *testing.T) {
		_, err := c.Get(ctx, "non_existent")
		if err != ErrCacheMiss {
			t.Errorf("expected ErrCacheMiss, got %v", err)
		}
	})
}

func TestRedisCache_Delete(t *testing.T) {
	_, redisOptions := setupTestRedis(t)

	cacheConfig := RedisCacheConfig{
		KeyPrefix:  "test:",
		DefaultTTL: time.Hour,
	}

	c, err := NewRedisCache(cacheConfig, redisOptions)
	if err != nil {
		t.Fatalf("failed to create redis cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	if err := c.Set(ctx, "delete_key", "delete_value", 5*time.Minute); err != nil {
		t.Fatalf("failed to set value: %v", err)
	}
	if err := c.Delete(ctx, "delete_key"); err != nil {
		t.Fatalf("failed to delete value: %v", err)
	}
	if _, err := c.Get(ctx, "delete_key"); err != ErrCacheMiss {
		t.Errorf("expected ErrCacheMiss after delete, got %v", err)
	}
}

func TestRedisCache_Exists(t *testing.T) {
	_, redisOptions := setupTestRedis(t)

	cacheConfig := RedisCacheConfig{
		KeyPrefix:  "test:",
		DefaultTTL: time.Hour,
	}

	c, err := NewRedisCache(cacheConfig, redisOptions)
	if err != nil {
		t.Fatalf("failed to create redis cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	t.Run("Key exists", func(t *testing.T) {
		if err := c.Set(ctx, "exists_key", "value", 5*time.Minute); err != nil {
			t.Fatalf("failed to set value: %v", err)
		}
		exists, err := c.Exists(ctx, "exists_key")
		if err != nil {
			t.Fatalf("failed to check existence: %v", err)
		}
		if !exists {
			t.Error("expected key to exist")
		}
	})

	t.Run("Key does not exist", func(t *testing.T) {
		exists, err := c.Exists(ctx, "not_exists_key")
		if err != nil {
			t.Fatalf("failed to check existence: %v", err)
		}
		if exists {
			t.Error("expected key to not exist")
		}
	})
}

func TestRedisCache_KeyPrefix(t *testing.T) {
	mr, redisOptions := setupTestRedis(t)

	cacheConfig := RedisCacheConfig{
		KeyPrefix:  "myprefix:",
		DefaultTTL: time.Hour,
	}

	c, err := NewRedisCache(cacheConfig, redisOptions)
	if err != nil {
		t.Fatalf("failed to create redis cache: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	if err := c.Set(ctx, "key1", "value1", 5*time.Minute); err != nil {
		t.Fatalf("failed to set value: %v", err)
	}

	keys := mr.Keys()
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	expectedKey := "myprefix:key1"
	if keys[0] != expectedKey {
		t.Errorf("expected key %s, got %s", expectedKey, keys[0])
	}
}
