package cache

import (
	"context"
	"strconv"
	"time"

	"github.com/ilaziness/app-tpl/internal/model"
)

// User cache key specifications. Unexported to prevent external mutation;
// callers interact with the cache exclusively through the typed methods below.
var (
	userByID    = KeySpec{Prefix: "user:by_id", TTL: 30 * time.Minute}
	userByEmail = KeySpec{Prefix: "user:by_email", TTL: 30 * time.Minute}
)

// GetUserByID retrieves a cached user by ID. Returns pkgcache.ErrCacheMiss
// when the key is absent. Callers must not cache password hashes in this entry.
func (m *Manager) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	return get[*model.User](m, ctx, userByID, strconv.FormatInt(id, 10))
}

// SetUserByID caches a user by ID. Pass a copy with Password cleared.
func (m *Manager) SetUserByID(ctx context.Context, id int64, user *model.User) error {
	return set(m, ctx, userByID, user, strconv.FormatInt(id, 10))
}

// DelUserByID invalidates the cached user by ID.
func (m *Manager) DelUserByID(ctx context.Context, id int64) error {
	return del(m, ctx, userByID, strconv.FormatInt(id, 10))
}

// GetUserByEmail retrieves a cached public user view by email.
func (m *Manager) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return get[*model.User](m, ctx, userByEmail, email)
}

// SetUserByEmail caches a public user view by email (no password).
func (m *Manager) SetUserByEmail(ctx context.Context, email string, user *model.User) error {
	return set(m, ctx, userByEmail, user, email)
}

// DelUserByEmail invalidates the cached user by email.
func (m *Manager) DelUserByEmail(ctx context.Context, email string) error {
	return del(m, ctx, userByEmail, email)
}
