package cache

import (
	"context"
	"testing"
	"time"

	pkgcache "github.com/ilaziness/app-tpl/pkg/cache"
)

func TestManager_UserByID(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	t.Run("miss then set then hit", func(t *testing.T) {
		// miss
		if _, err := m.GetUserByID(ctx, 1); !pkgcache.IsCacheMiss(err) {
			t.Fatalf("expected ErrCacheMiss, got %v", err)
		}

		// set
		user := newTestUser(1)
		if err := m.SetUserByID(ctx, 1, user); err != nil {
			t.Fatalf("SetUserByID failed: %v", err)
		}

		// hit
		got, err := m.GetUserByID(ctx, 1)
		if err != nil {
			t.Fatalf("GetUserByID failed: %v", err)
		}
		if got.ID != user.ID || got.Email != user.Email || got.Name != user.Name {
			t.Errorf("got %+v, want %+v", got, user)
		}
	})

	t.Run("delete", func(t *testing.T) {
		user := newTestUser(2)
		if err := m.SetUserByID(ctx, 2, user); err != nil {
			t.Fatalf("SetUserByID failed: %v", err)
		}
		if err := m.DelUserByID(ctx, 2); err != nil {
			t.Fatalf("DelUserByID failed: %v", err)
		}
		if _, err := m.GetUserByID(ctx, 2); !pkgcache.IsCacheMiss(err) {
			t.Fatalf("expected ErrCacheMiss after delete, got %v", err)
		}
	})
}

func TestManager_UserByEmail(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	user := newTestUser(3)
	user.Email = "bob@example.com"
	if err := m.SetUserByEmail(ctx, "bob@example.com", user); err != nil {
		t.Fatalf("SetUserByEmail failed: %v", err)
	}
	got, err := m.GetUserByEmail(ctx, "bob@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if got.Email != "bob@example.com" || got.ID != 3 {
		t.Errorf("got %+v, want id=3 email=bob@example.com", got)
	}
}

// TestManager_UserRedisPath verifies the JSON round-trip path used when the
// underlying cache is Redis (Get returns map[string]any, not *model.User).
func TestManager_UserRedisPath(t *testing.T) {
	opts := setupTestRedisOptions(t)
	c, err := pkgcache.NewRedisCache(
		pkgcache.RedisCacheConfig{KeyPrefix: "mgr:", DefaultTTL: time.Hour},
		opts,
	)
	if err != nil {
		t.Fatalf("failed to create redis cache: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	m := NewManager(c)
	ctx := context.Background()

	user := newTestUser(10)
	if setErr := m.SetUserByID(ctx, 10, user); setErr != nil {
		t.Fatalf("SetUserByID failed: %v", setErr)
	}
	got, err := m.GetUserByID(ctx, 10)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if got.ID != user.ID || got.Email != user.Email || got.Name != user.Name {
		t.Errorf("got %+v, want %+v", got, user)
	}
}
