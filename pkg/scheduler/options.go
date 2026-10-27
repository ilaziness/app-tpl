package scheduler

import (
	"time"

	"go.uber.org/zap"
)

// defaultLockRenewInterval is the default renew period; keep it below lock TTL.
const defaultLockRenewInterval = 10 * time.Second

// Option configures Scheduler construction.
type Option func(*options)

type options struct {
	log               *zap.Logger
	lock              LockProvider
	lockRenewInterval time.Duration
}

// WithLogger sets the logger. nil becomes zap.NewNop().
func WithLogger(log *zap.Logger) Option {
	return func(o *options) {
		o.log = log
	}
}

// WithLock sets the leader lock. nil becomes NoopLockProvider.
func WithLock(lock LockProvider) Option {
	return func(o *options) {
		o.lock = lock
	}
}

// WithLockRenewInterval sets how often the scheduler renews the leader lock.
// Values <= 0 are ignored. Renew interval should stay below the lock TTL.
func WithLockRenewInterval(d time.Duration) Option {
	return func(o *options) {
		if d > 0 {
			o.lockRenewInterval = d
		}
	}
}

func applyOptions(opts []Option) *options {
	o := &options{
		log:               zap.NewNop(),
		lock:              NoopLockProvider{},
		lockRenewInterval: defaultLockRenewInterval,
	}
	for _, opt := range opts {
		opt(o)
	}
	if o.log == nil {
		o.log = zap.NewNop()
	}
	if o.lock == nil {
		o.lock = NoopLockProvider{}
	}
	return o
}
