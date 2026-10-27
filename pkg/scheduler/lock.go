package scheduler

import (
	"context"
	"errors"
)

// ErrLockNotHeld indicates the leader lock was lost (expired or taken by another instance).
// When Renew returns this error, Scheduler stops cron to avoid split-brain.
var ErrLockNotHeld = errors.New("scheduler: lock not held")

// LockProvider provides single-leader mutual exclusion for the scheduler.
// Multi-instance deployments use a distributed lock so only one process runs cron.
// Single-instance deployments use NoopLockProvider.
type LockProvider interface {
	// Acquire tries to obtain the lock. ok is false when another holder exists (not an error).
	Acquire(ctx context.Context) (ok bool, err error)
	// Renew extends the lock TTL. Returns ErrLockNotHeld if the lock is no longer owned.
	Renew(ctx context.Context) error
	// Release releases the lock. Safe when the lock was never acquired.
	Release(ctx context.Context) error
}

// NoopLockProvider always acquires successfully and does no mutual exclusion.
type NoopLockProvider struct{}

// Acquire always returns true.
func (NoopLockProvider) Acquire(ctx context.Context) (bool, error) {
	return true, nil
}

// Renew is a no-op.
func (NoopLockProvider) Renew(ctx context.Context) error {
	return nil
}

// Release is a no-op.
func (NoopLockProvider) Release(ctx context.Context) error {
	return nil
}
