package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// defaultParser is the standard cron parser (5 fields + descriptors).
var defaultParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// DefaultParser returns the package standard cron parser.
func DefaultParser() cron.Parser {
	return defaultParser
}

// Cron is the registration surface Jobs use during Init (and later Reload).
type Cron interface {
	AddFunc(expr string, fn func()) (cron.EntryID, error)
	Remove(id cron.EntryID)
}

// Job is a schedulable unit. Init runs after cron has started; register entries via Cron.
type Job interface {
	Init(ctx context.Context, c Cron) error
}

// Scheduler wraps robfig/cron/v3 and manages Job lifecycle plus optional leader locking.
type Scheduler struct {
	cron *cron.Cron
	jobs []Job
	lock LockProvider
	log  *zap.Logger

	// cronStarted is true while cron is running; heartbeat may clear it on lock loss.
	cronStarted atomic.Bool

	// lockOwned is true when this instance holds the leader lock (Start/Stop/releaseLock only).
	lockOwned bool

	lockRenewInterval time.Duration

	heartbeatCancel context.CancelFunc
	heartbeatDone   chan struct{}
}

// New creates a Scheduler. Default lock is NoopLockProvider (single instance).
func New(opts ...Option) *Scheduler {
	o := applyOptions(opts)
	return &Scheduler{
		cron: cron.New(
			cron.WithParser(defaultParser),
			cron.WithChain(cron.Recover(cron.DefaultLogger)),
		),
		lock:              o.lock,
		log:               o.log,
		lockRenewInterval: o.lockRenewInterval,
	}
}

// Register adds a Job. Call before Start.
func (s *Scheduler) Register(job Job) {
	s.jobs = append(s.jobs, job)
}

// Start acquires the leader lock, starts cron, then runs Job.Init for each job.
//
// If the lock is held by another instance, Start returns nil without starting cron.
// If any Job.Init fails, cron is stopped, the lock is released, and the error is returned.
func (s *Scheduler) Start(ctx context.Context) error {
	ok, err := s.lock.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("scheduler: acquire lock: %w", err)
	}
	if !ok {
		s.log.Info("scheduler: another instance already holds the lock, skipping cron startup")
		return nil
	}
	s.lockOwned = true
	s.startHeartbeat()

	s.cron.Start()
	s.cronStarted.Store(true)
	for _, job := range s.jobs {
		if err := job.Init(ctx, s); err != nil {
			s.stopCron(ctx)
			s.cronStarted.Store(false)
			s.releaseLock(ctx)
			return fmt.Errorf("scheduler: job init: %w", err)
		}
	}
	return nil
}

func (s *Scheduler) startHeartbeat() {
	hbCtx, cancel := context.WithCancel(context.Background())
	s.heartbeatCancel = cancel
	done := make(chan struct{})
	s.heartbeatDone = done
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("scheduler: heartbeat panic recovered",
					zap.Any("panic", r),
					zap.Stack("stack"))
			}
		}()
		ticker := time.NewTicker(s.lockRenewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := s.lock.Renew(ctx)
				cancel()
				if err == nil {
					continue
				}
				if errors.Is(err, ErrLockNotHeld) {
					s.log.Error("scheduler: lock lost, stopping cron to avoid split-brain")
					s.stopCron(context.Background())
					s.cronStarted.Store(false)
					return
				}
				s.log.Warn("scheduler: failed to renew lock", zap.Error(err))
			case <-hbCtx.Done():
				return
			}
		}
	}()
}

func (s *Scheduler) releaseLock(ctx context.Context) {
	if !s.lockOwned {
		return
	}
	if s.heartbeatCancel != nil {
		s.heartbeatCancel()
		<-s.heartbeatDone
		s.heartbeatCancel = nil
		s.heartbeatDone = nil
	}
	if err := s.lock.Release(ctx); err != nil {
		s.log.Warn("scheduler: failed to release lock", zap.Error(err))
	}
	s.lockOwned = false
}

// Stop stops cron (waiting up to 5s for running jobs) and releases the leader lock if owned.
// Idempotent when cron was never started or already stopped by heartbeat.
func (s *Scheduler) Stop(ctx context.Context) error {
	if s.cronStarted.Load() {
		s.stopCron(ctx)
		s.cronStarted.Store(false)
	}
	s.releaseLock(ctx)
	return nil
}

func (s *Scheduler) stopCron(ctx context.Context) {
	stopCtx := s.cron.Stop()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-stopCtx.Done():
	case <-ctx.Done():
	case <-timer.C:
		s.log.Warn("cron scheduler stop timeout after 5s")
	}
}

// AddFunc registers a cron entry. Jobs call this from Init.
func (s *Scheduler) AddFunc(expr string, fn func()) (cron.EntryID, error) {
	return s.cron.AddFunc(expr, fn)
}

// Remove removes a cron entry by ID.
func (s *Scheduler) Remove(id cron.EntryID) {
	s.cron.Remove(id)
}
