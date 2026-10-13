package cache

import (
	"context"
	"encoding/json"
	"fmt"

	pkgcache "github.com/ilaziness/app-tpl/pkg/cache"
)

// Manager is the business cache facade. It exposes typed Get/Set/Delete
// methods per entity, hiding key construction and (de)serialization from
// callers. Business KeySpec definitions and typed methods are organized
// in per-entity files (e.g. user.go).
type Manager struct {
	cache  pkgcache.Cache
	keyGen pkgcache.KeyGenerator
}

// NewManager wraps a pkg/cache.Cache with the default key generator.
func NewManager(c pkgcache.Cache) *Manager {
	return &Manager{
		cache:  c,
		keyGen: pkgcache.NewDefaultKeyGenerator(),
	}
}

// Close releases the underlying cache resources.
func (m *Manager) Close() error {
	return m.cache.Close()
}

// Underlying returns the raw pkg/cache.Cache for adapters (e.g. captcha store).
func (m *Manager) Underlying() pkgcache.Cache {
	return m.cache
}

// --- generic helpers ---

// get reads and decodes a value for the given KeySpec + parts.
func get[T any](m *Manager, ctx context.Context, spec KeySpec, parts ...string) (T, error) {
	var zero T
	key := m.keyGen.Generate(spec.Prefix, parts...)
	val, err := m.cache.Get(ctx, key)
	if err != nil {
		return zero, err
	}
	return decodeAs[T](val)
}

// set writes value for the given KeySpec + parts using the spec's TTL.
func set(m *Manager, ctx context.Context, spec KeySpec, value any, parts ...string) error {
	key := m.keyGen.Generate(spec.Prefix, parts...)
	return m.cache.Set(ctx, key, value, spec.TTL)
}

// del removes the key for the given KeySpec + parts.
func del(m *Manager, ctx context.Context, spec KeySpec, parts ...string) error {
	key := m.keyGen.Generate(spec.Prefix, parts...)
	return m.cache.Delete(ctx, key)
}

// decodeAs converts the cached any into T via a JSON round-trip so Memory
// (raw values) and Redis (JSON-decoded maps) share one decoding path.
func decodeAs[T any](val any) (T, error) {
	var zero T
	data, err := json.Marshal(val)
	if err != nil {
		return zero, fmt.Errorf("cache: failed to marshal cached value: %w", err)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return zero, fmt.Errorf("cache: failed to unmarshal into %T: %w", zero, err)
	}
	return result, nil
}
