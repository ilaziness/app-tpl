package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockLockProvider struct {
	mu           sync.Mutex
	locked       bool
	acquireErr   error
	renewErr     error
	releaseCount int64
}

func (m *mockLockProvider) Acquire(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.acquireErr != nil {
		return false, m.acquireErr
	}
	if m.locked {
		return false, nil
	}
	m.locked = true
	return true, nil
}

func (m *mockLockProvider) Renew(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.renewErr != nil {
		return m.renewErr
	}
	return nil
}

func (m *mockLockProvider) Release(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.releaseCount++
	m.locked = false
	return nil
}

func (m *mockLockProvider) isLocked() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.locked
}

func (m *mockLockProvider) getReleaseCount() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.releaseCount
}

type mockJob struct {
	initErr   error
	initCount int64
}

func (j *mockJob) Init(ctx context.Context, c Cron) error {
	atomic.AddInt64(&j.initCount, 1)
	return j.initErr
}

func TestScheduler_StartStop_NoopLock(t *testing.T) {
	ctx := context.Background()

	sched := New()
	job := &mockJob{}
	sched.Register(job)

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if atomic.LoadInt64(&job.initCount) != 1 {
		t.Error("expected Job.Init once")
	}
	if !sched.cronStarted.Load() {
		t.Error("expected cronStarted")
	}

	if err := sched.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if sched.cronStarted.Load() {
		t.Error("expected cronStopped after Stop")
	}
}

func TestScheduler_StartWithLock_Acquired(t *testing.T) {
	ctx := context.Background()

	lock := &mockLockProvider{}
	sched := New(WithLock(lock))
	job := &mockJob{}
	sched.Register(job)

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !lock.isLocked() {
		t.Error("expected lock held")
	}
	if atomic.LoadInt64(&job.initCount) != 1 {
		t.Error("expected Job.Init once")
	}

	if err := sched.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if lock.isLocked() {
		t.Error("expected lock released")
	}
	if lock.getReleaseCount() != 1 {
		t.Error("expected Release once")
	}
}

func TestScheduler_StartWithLock_AlreadyHeld(t *testing.T) {
	ctx := context.Background()

	lock := &mockLockProvider{locked: true}
	sched := New(WithLock(lock))
	job := &mockJob{}
	sched.Register(job)

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start should skip when lock held: %v", err)
	}
	if sched.cronStarted.Load() {
		t.Error("expected cron not started")
	}
	if atomic.LoadInt64(&job.initCount) != 0 {
		t.Error("expected Job.Init not called")
	}
	if err := sched.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if lock.getReleaseCount() != 0 {
		t.Error("expected no Release when lock not owned")
	}
}

func TestScheduler_StartWithLock_AcquireError(t *testing.T) {
	ctx := context.Background()

	lock := &mockLockProvider{acquireErr: errors.New("redis down")}
	sched := New(WithLock(lock))
	sched.Register(&mockJob{})

	if err := sched.Start(ctx); err == nil {
		t.Fatal("expected Acquire error")
	}
	if sched.cronStarted.Load() {
		t.Error("expected cron not started")
	}
}

func TestScheduler_StartJobInitFailure_ReleasesLock(t *testing.T) {
	ctx := context.Background()

	lock := &mockLockProvider{}
	sched := New(WithLock(lock))
	sched.Register(&mockJob{initErr: errors.New("init failed")})

	if err := sched.Start(ctx); err == nil {
		t.Fatal("expected Init error")
	}
	if lock.isLocked() {
		t.Error("expected lock released after Init failure")
	}
	if sched.cronStarted.Load() {
		t.Error("expected cron not running")
	}
}

func TestScheduler_StopWithoutStart(t *testing.T) {
	if err := New().Stop(context.Background()); err != nil {
		t.Fatalf("Stop without Start: %v", err)
	}
}

func TestScheduler_StopReleasesLock_AllowsOtherInstance(t *testing.T) {
	ctx := context.Background()
	lock := &mockLockProvider{}

	sched1 := New(WithLock(lock))
	if err := sched1.Start(ctx); err != nil {
		t.Fatalf("sched1 Start: %v", err)
	}

	sched2 := New(WithLock(lock))
	if err := sched2.Start(ctx); err != nil {
		t.Fatalf("sched2 Start: %v", err)
	}
	if sched2.cronStarted.Load() {
		t.Error("sched2 should skip while sched1 holds lock")
	}

	if err := sched1.Stop(ctx); err != nil {
		t.Fatalf("sched1 Stop: %v", err)
	}

	sched3 := New(WithLock(lock))
	if err := sched3.Start(ctx); err != nil {
		t.Fatalf("sched3 Start: %v", err)
	}
	if !sched3.cronStarted.Load() {
		t.Error("expected sched3 to start")
	}
	_ = sched3.Stop(ctx)
}

func TestScheduler_HeartbeatStopsOnRelease(t *testing.T) {
	ctx := context.Background()

	lock := &mockLockProvider{}
	sched := New(WithLock(lock))
	sched.Register(&mockJob{})
	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- sched.Stop(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Stop timed out (heartbeat not stopped)")
	}
	if lock.getReleaseCount() != 1 {
		t.Error("expected Release once")
	}
}

func TestScheduler_HeartbeatStopsCronOnLockLoss(t *testing.T) {
	ctx := context.Background()

	lock := &mockLockProvider{renewErr: ErrLockNotHeld}
	sched := New(WithLock(lock), WithLockRenewInterval(50*time.Millisecond))
	sched.Register(&mockJob{})
	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if !sched.cronStarted.Load() {
				if err := sched.Stop(ctx); err != nil {
					t.Fatalf("Stop after lock loss: %v", err)
				}
				return
			}
		case <-deadline:
			t.Fatal("heartbeat did not stop cron on lock loss")
		}
	}
}

func TestScheduler_StopAfterHeartbeatStoppedCron(t *testing.T) {
	ctx := context.Background()

	lock := &mockLockProvider{renewErr: ErrLockNotHeld}
	sched := New(WithLock(lock), WithLockRenewInterval(50*time.Millisecond))
	sched.Register(&mockJob{})
	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if !sched.cronStarted.Load() {
				done := make(chan error, 1)
				go func() { done <- sched.Stop(ctx) }()
				select {
				case err := <-done:
					if err != nil {
						t.Fatalf("Stop: %v", err)
					}
					return
				case <-time.After(5 * time.Second):
					t.Fatal("Stop timed out")
				}
			}
		case <-deadline:
			t.Fatal("cron was not stopped by heartbeat")
		}
	}
}
