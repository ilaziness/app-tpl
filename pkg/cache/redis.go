package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCacheConfig configures a RedisCache (key prefix and default TTL).
type RedisCacheConfig struct {
	KeyPrefix  string        // key prefix applied to all keys
	DefaultTTL time.Duration // default TTL used when Set is called with ttl <= 0
}

// RedisOptions describes the Redis connection parameters. It mirrors the
// subset of go-redis Options used by this package and intentionally avoids
// any framework-specific tags so the package stays self-contained.
type RedisOptions struct {
	Addr            string // host:port
	Password        string
	DB              int
	PoolSize        int
	MinIdleConns    int
	ConnMaxIdleTime time.Duration
}

// RedisCache is a Cache backed by Redis.
type RedisCache struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

// NewRedisCache creates a RedisCache. It pings Redis once to verify
// connectivity and returns an error if the server is unreachable.
func NewRedisCache(cacheConfig RedisCacheConfig, opts RedisOptions) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:            opts.Addr,
		Password:        opts.Password,
		DB:              opts.DB,
		PoolSize:        opts.PoolSize,
		MinIdleConns:    opts.MinIdleConns,
		ConnMaxIdleTime: opts.ConnMaxIdleTime,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	prefix := "app:"
	if cacheConfig.KeyPrefix != "" {
		prefix = cacheConfig.KeyPrefix
	}

	defaultTTL := 1 * time.Hour
	if cacheConfig.DefaultTTL > 0 {
		defaultTTL = cacheConfig.DefaultTTL
	}

	return &RedisCache{
		client: client,
		prefix: prefix,
		ttl:    defaultTTL,
	}, nil
}

// buildKey applies the configured prefix to the key.
func (c *RedisCache) buildKey(key string) string {
	return c.prefix + key
}

// Get implements Cache. The stored JSON is unmarshaled into an any so that
// the return value has the same semantics as MemoryCache.Get (an any, not a
// raw JSON string).
func (c *RedisCache) Get(ctx context.Context, key string) (any, error) {
	if key == "" {
		return nil, ErrInvalidKey
	}

	data, err := c.client.Get(ctx, c.buildKey(key)).Bytes()
	if err == redis.Nil {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, &CacheError{
			Code:    "REDIS_GET_ERROR",
			Message: "failed to get value from Redis",
			Err:     err,
		}
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, &CacheError{
			Code:    "REDIS_UNMARSHAL_ERROR",
			Message: "failed to unmarshal cached value",
			Err:     err,
		}
	}
	return value, nil
}

// Set implements Cache. The value is JSON-marshaled before storage.
func (c *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if key == "" {
		return ErrInvalidKey
	}
	if ttl <= 0 {
		ttl = c.ttl
	}

	data, err := json.Marshal(value)
	if err != nil {
		return &CacheError{
			Code:    "REDIS_MARSHAL_ERROR",
			Message: "failed to marshal value",
			Err:     err,
		}
	}

	if err := c.client.Set(ctx, c.buildKey(key), data, ttl).Err(); err != nil {
		return &CacheError{
			Code:    "REDIS_SET_ERROR",
			Message: "failed to set value to Redis",
			Err:     err,
		}
	}
	return nil
}

// Delete implements Cache.
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	if key == "" {
		return ErrInvalidKey
	}
	if err := c.client.Del(ctx, c.buildKey(key)).Err(); err != nil {
		return &CacheError{
			Code:    "REDIS_DELETE_ERROR",
			Message: "failed to delete value from Redis",
			Err:     err,
		}
	}
	return nil
}

// Exists implements Cache.
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, ErrInvalidKey
	}
	count, err := c.client.Exists(ctx, c.buildKey(key)).Result()
	if err != nil {
		return false, &CacheError{
			Code:    "REDIS_EXISTS_ERROR",
			Message: "failed to check existence in Redis",
			Err:     err,
		}
	}
	return count > 0, nil
}

// Clear implements Cache. It scans and deletes all keys with the configured
// prefix. Intended for tests/development only.
func (c *RedisCache) Clear(ctx context.Context) error {
	pattern := c.prefix + "*"
	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return &CacheError{
				Code:    "REDIS_CLEAR_ERROR",
				Message: "failed to scan keys in Redis",
				Err:     err,
			}
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return &CacheError{
					Code:    "REDIS_CLEAR_ERROR",
					Message: "failed to delete keys in Redis",
					Err:     err,
				}
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

// Close implements Cache.
func (c *RedisCache) Close() error {
	return c.client.Close()
}
