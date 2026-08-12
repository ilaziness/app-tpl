// Package cache provides business cache management: KeySpec definitions
// (prefix + TTL) and a Manager exposing typed Get/Set/Delete methods per
// business entity. The generic cache primitives live in
// github.com/ilaziness/app-tpl/pkg/cache.
package cache

import "time"

// KeySpec describes a business cache key: a prefix joined with dynamic parts
// to form the full key, and the TTL applied on Set.
type KeySpec struct {
	Prefix string
	TTL    time.Duration
}
