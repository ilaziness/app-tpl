package cache

import (
	"errors"
	"testing"
)

func TestCacheErrors(t *testing.T) {
	t.Run("ErrCacheMiss", func(t *testing.T) {
		if !errors.Is(ErrCacheMiss, ErrCacheMiss) {
			t.Error("ErrCacheMiss should be comparable with errors.Is")
		}
	})
	t.Run("ErrCacheUnavailable", func(t *testing.T) {
		if !errors.Is(ErrCacheUnavailable, ErrCacheUnavailable) {
			t.Error("ErrCacheUnavailable should be comparable with errors.Is")
		}
	})
	t.Run("ErrInvalidKey", func(t *testing.T) {
		if !errors.Is(ErrInvalidKey, ErrInvalidKey) {
			t.Error("ErrInvalidKey should be comparable with errors.Is")
		}
	})
}

func TestCacheError(t *testing.T) {
	t.Run("Error without wrapped error", func(t *testing.T) {
		err := &CacheError{
			Code:    "TEST_CODE",
			Message: "test message",
		}
		expected := "cache error [TEST_CODE]: test message"
		if err.Error() != expected {
			t.Errorf("expected %s, got %s", expected, err.Error())
		}
	})
	t.Run("Error with wrapped error", func(t *testing.T) {
		wrappedErr := errors.New("wrapped error")
		err := &CacheError{
			Code:    "TEST_CODE",
			Message: "test message",
			Err:     wrappedErr,
		}
		expected := "cache error [TEST_CODE]: test message: wrapped error"
		if err.Error() != expected {
			t.Errorf("expected %s, got %s", expected, err.Error())
		}
	})
	t.Run("Unwrap", func(t *testing.T) {
		wrappedErr := errors.New("wrapped error")
		err := &CacheError{
			Code:    "TEST_CODE",
			Message: "test message",
			Err:     wrappedErr,
		}
		if unwrapped := err.Unwrap(); unwrapped != wrappedErr {
			t.Errorf("expected %v, got %v", wrappedErr, unwrapped)
		}
	})
}

func TestIsCacheMiss(t *testing.T) {
	if !IsCacheMiss(ErrCacheMiss) {
		t.Error("IsCacheMiss should report true for ErrCacheMiss")
	}
	if IsCacheMiss(ErrInvalidKey) {
		t.Error("IsCacheMiss should report false for ErrInvalidKey")
	}
	// wrapped
	if !IsCacheMiss(&CacheError{Err: ErrCacheMiss}) {
		t.Error("IsCacheMiss should traverse wrapped errors")
	}
}

func TestIsInvalidKey(t *testing.T) {
	if !IsInvalidKey(ErrInvalidKey) {
		t.Error("IsInvalidKey should report true for ErrInvalidKey")
	}
	if IsInvalidKey(ErrCacheMiss) {
		t.Error("IsInvalidKey should report false for ErrCacheMiss")
	}
}

func TestIsCacheUnavailable(t *testing.T) {
	if !IsCacheUnavailable(ErrCacheUnavailable) {
		t.Error("IsCacheUnavailable should report true for ErrCacheUnavailable")
	}
	if IsCacheUnavailable(ErrCacheMiss) {
		t.Error("IsCacheUnavailable should report false for ErrCacheMiss")
	}
	// wrapped
	if !IsCacheUnavailable(&CacheError{Err: ErrCacheUnavailable}) {
		t.Error("IsCacheUnavailable should traverse wrapped errors")
	}
}
