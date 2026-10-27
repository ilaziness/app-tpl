package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"
)

// defaultRedisLockTTL is the default leader-lock TTL; renew interval must stay below it.
const defaultRedisLockTTL = 30 * time.Second

// RedisLockOption configures NewRedisLock.
type RedisLockOption func(*redisLockOptions)

type redisLockOptions struct {
	ttl time.Duration
}

// WithTTL sets the Redis leader-lock TTL. Values <= 0 are ignored.
func WithTTL(ttl time.Duration) RedisLockOption {
	return func(o *redisLockOptions) {
		if ttl > 0 {
			o.ttl = ttl
		}
	}
}

// redisLockProvider implements LockProvider with bsm/redislock.
type redisLockProvider struct {
	locker *redislock.Client
	key    string
	ttl    time.Duration
	// lock is set after a successful Acquire.
	// Acquire / Renew / Release are serialized by Scheduler (Release waits for heartbeat exit).
	lock *redislock.Lock
}

// NewRedisLock creates a Redis-backed leader LockProvider.
// key must be non-empty (chosen by the application). client must be non-nil.
func NewRedisLock(client redis.UniversalClient, key string, opts ...RedisLockOption) LockProvider {
	if client == nil {
		panic("scheduler: NewRedisLock: redis client is nil")
	}
	if key == "" {
		panic("scheduler: NewRedisLock: lock key is empty")
	}
	o := &redisLockOptions{ttl: defaultRedisLockTTL}
	for _, opt := range opts {
		opt(o)
	}
	return &redisLockProvider{
		locker: redislock.New(client),
		key:    key,
		ttl:    o.ttl,
	}
}

// Acquire tries to obtain the leader lock with NoRetry (fail fast when held elsewhere).
func (p *redisLockProvider) Acquire(ctx context.Context) (bool, error) {
	lock, err := p.locker.Obtain(ctx, p.key, p.ttl, &redislock.Options{
		RetryStrategy: redislock.NoRetry(),
	})
	if errors.Is(err, redislock.ErrNotObtained) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("scheduler: obtain lock: %w", err)
	}
	p.lock = lock
	return true, nil
}

// Renew refreshes the lock TTL. Returns ErrLockNotHeld if the lock is lost.
func (p *redisLockProvider) Renew(ctx context.Context) error {
	if p.lock == nil {
		return ErrLockNotHeld
	}
	if err := p.lock.Refresh(ctx, p.ttl, nil); err != nil {
		if errors.Is(err, redislock.ErrNotObtained) {
			return ErrLockNotHeld
		}
		return fmt.Errorf("scheduler: renew lock: %w", err)
	}
	return nil
}

// Release releases the leader lock. Safe when Acquire was never called.
func (p *redisLockProvider) Release(ctx context.Context) error {
	if p.lock == nil {
		return nil
	}
	if err := p.lock.Release(ctx); err != nil && !errors.Is(err, redislock.ErrLockNotHeld) {
		return fmt.Errorf("scheduler: release lock: %w", err)
	}
	p.lock = nil
	return nil
}
