package cache_test

import (
	"context"
	"fmt"
	"time"

	"github.com/ilaziness/app-tpl/pkg/cache"
)

// ExampleMemoryCache demonstrates basic usage of the in-memory cache.
func ExampleMemoryCache() {
	config := cache.MemoryCacheConfig{
		NumCounters: 1000,
		MaxCost:     1 << 20,
		BufferItems: 64,
	}

	c, err := cache.NewMemoryCache(config)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	ctx := context.Background()
	_ = c.Set(ctx, "user:1", "John Doe", 5*time.Minute)

	value, err := c.Get(ctx, "user:1")
	if err != nil {
		fmt.Println("cache miss or error:", err)
		return
	}
	fmt.Println(value)
	// Output: John Doe
}

// ExampleMultiCache demonstrates the L1 (memory) + L2 (Redis) multi-level
// cache strategy. For brevity this example only exercises L1; see the test
// suite for a full L1+L2 example using miniredis.
func ExampleMultiCache() {
	l1Config := cache.MemoryCacheConfig{
		NumCounters: 1000,
		MaxCost:     1 << 20,
		BufferItems: 64,
	}
	l1, err := cache.NewMemoryCache(l1Config)
	if err != nil {
		panic(err)
	}
	defer l1.Close()

	// In production, configure a real Redis for L2:
	//   l2, _ := cache.NewRedisCache(
	//       cache.RedisCacheConfig{KeyPrefix: "app:", DefaultTTL: 10 * time.Minute},
	//       cache.RedisOptions{Addr: "localhost:6379"},
	//   )
	//   multiCache := cache.NewMultiCache(l1, l2)

	ctx := context.Background()
	_ = l1.Set(ctx, "product:1001", "Premium Widget", 10*time.Minute)

	value, err := l1.Get(ctx, "product:1001")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(value)
	// Output: Premium Widget
}

// ExampleCache_keyGenerator shows how to build structured cache keys.
func ExampleCache_keyGenerator() {
	config := cache.MemoryCacheConfig{
		NumCounters: 1000,
		MaxCost:     1 << 20,
		BufferItems: 64,
	}
	c, _ := cache.NewMemoryCache(config)
	defer c.Close()

	keyGen := cache.NewDefaultKeyGenerator()
	ctx := context.Background()

	key := keyGen.Generate("user", "123", "profile")
	_ = c.Set(ctx, key, map[string]string{
		"name":  "Alice",
		"email": "alice@example.com",
	}, 5*time.Minute)

	value, _ := c.Get(ctx, key)
	fmt.Printf("Key: %s\n", key)
	fmt.Printf("Value: %v\n", value)
	// Output:
	// Key: user:123:profile
	// Value: map[email:alice@example.com name:Alice]
}

// ExampleCache_errorHandling demonstrates cache error handling.
func ExampleCache_errorHandling() {
	config := cache.MemoryCacheConfig{
		NumCounters: 1000,
		MaxCost:     1 << 20,
		BufferItems: 64,
	}
	c, _ := cache.NewMemoryCache(config)
	defer c.Close()

	ctx := context.Background()

	_, err := c.Get(ctx, "nonexistent:key")
	if err != nil {
		if cache.IsCacheMiss(err) {
			fmt.Println("Key not found in cache")
		} else {
			fmt.Println("Cache error:", err)
		}
	}

	if err := c.Set(ctx, "", "value", 5*time.Minute); err != nil {
		fmt.Println("Invalid key error:", err)
	}
	// Output:
	// Key not found in cache
	// Invalid key error: cache: invalid key
}
