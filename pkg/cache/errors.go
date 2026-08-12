package cache

import (
	"errors"
	"fmt"
)

var (
	// ErrCacheMiss is returned when a cache key is not found.
	ErrCacheMiss = errors.New("cache: key not found")
	// ErrCacheUnavailable is returned when the cache service is unavailable.
	ErrCacheUnavailable = errors.New("cache: service unavailable")
	// ErrInvalidKey is returned when an empty or otherwise invalid key is used.
	ErrInvalidKey = errors.New("cache: invalid key")
)

// CacheError is a structured cache error carrying a code and an optional cause.
type CacheError struct {
	Code    string
	Message string
	Err     error
}

// Error implements error.
func (e *CacheError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("cache error [%s]: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("cache error [%s]: %s", e.Code, e.Message)
}

// Unwrap allows errors.Is/As to traverse the wrapped error.
func (e *CacheError) Unwrap() error {
	return e.Err
}

// IsCacheMiss reports whether err is (or wraps) ErrCacheMiss.
func IsCacheMiss(err error) bool {
	return errors.Is(err, ErrCacheMiss)
}

// IsInvalidKey reports whether err is (or wraps) ErrInvalidKey.
func IsInvalidKey(err error) bool {
	return errors.Is(err, ErrInvalidKey)
}

// IsCacheUnavailable reports whether err is (or wraps) ErrCacheUnavailable.
func IsCacheUnavailable(err error) bool {
	return errors.Is(err, ErrCacheUnavailable)
}
