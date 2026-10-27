package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const testLockKey = "test:scheduler:lock"

func setupTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return client
}

func TestRedisLockProvider_AcquireAndRelease(t *testing.T) {
	ctx := context.Background()
	p := NewRedisLock(setupTestRedisClient(t), testLockKey)

	ok, err := p.Acquire(ctx)
	if err != nil || !ok {
		t.Fatalf("first Acquire: ok=%v err=%v", ok, err)
	}

	ok2, err := p.Acquire(ctx)
	if err != nil {
		t.Fatalf("second Acquire error: %v", err)
	}
	if ok2 {
		t.Error("expected second Acquire to fail")
	}

	if err := p.Release(ctx); err != nil {
		t.Fatalf("Release: %v", err)
	}

	ok3, err := p.Acquire(ctx)
	if err != nil || !ok3 {
		t.Fatalf("Acquire after Release: ok=%v err=%v", ok3, err)
	}
	_ = p.Release(ctx)
}

func TestRedisLockProvider_MutualExclusion(t *testing.T) {
	ctx := context.Background()
	client := setupTestRedisClient(t)
	p1 := NewRedisLock(client, testLockKey)
	p2 := NewRedisLock(client, testLockKey)

	ok1, err := p1.Acquire(ctx)
	if err != nil || !ok1 {
		t.Fatalf("p1 Acquire: ok=%v err=%v", ok1, err)
	}

	ok2, err := p2.Acquire(ctx)
	if err != nil {
		t.Fatalf("p2 Acquire error: %v", err)
	}
	if ok2 {
		t.Error("expected p2 Acquire to fail")
	}

	if err := p1.Release(ctx); err != nil {
		t.Fatalf("p1 Release: %v", err)
	}
	ok3, err := p2.Acquire(ctx)
	if err != nil || !ok3 {
		t.Fatalf("p2 Acquire after release: ok=%v err=%v", ok3, err)
	}
	_ = p2.Release(ctx)
}

func TestRedisLockProvider_Renew(t *testing.T) {
	ctx := context.Background()
	p := NewRedisLock(setupTestRedisClient(t), testLockKey)
	if _, err := p.Acquire(ctx); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := p.Renew(ctx); err != nil {
		t.Errorf("Renew: %v", err)
	}
	_ = p.Release(ctx)
}

func TestRedisLockProvider_RenewWithoutAcquire(t *testing.T) {
	p := NewRedisLock(setupTestRedisClient(t), testLockKey)
	err := p.Renew(context.Background())
	if !errors.Is(err, ErrLockNotHeld) {
		t.Fatalf("expected ErrLockNotHeld, got %v", err)
	}
}

func TestRedisLockProvider_ReleaseWithoutAcquire(t *testing.T) {
	p := NewRedisLock(setupTestRedisClient(t), testLockKey)
	if err := p.Release(context.Background()); err != nil {
		t.Errorf("Release without Acquire: %v", err)
	}
}

func TestNewRedisLock_PanicsOnEmptyKey(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on empty key")
		}
	}()
	_ = NewRedisLock(setupTestRedisClient(t), "")
}

func TestNewRedisLock_PanicsOnNilClient(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on nil client")
		}
	}()
	_ = NewRedisLock(nil, testLockKey)
}
