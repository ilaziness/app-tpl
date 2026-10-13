package scheduler

import (
	"context"

	"go.uber.org/zap"
)

// DemoJob is a no-op sample cron job that demonstrates Scheduler registration.
// It logs once per minute when the scheduler holds the leader lock.
type DemoJob struct {
	log *zap.Logger
}

// NewDemoJob creates a DemoJob.
func NewDemoJob(log *zap.Logger) *DemoJob {
	if log == nil {
		log = zap.NewNop()
	}
	return &DemoJob{log: log}
}

// Init registers a sample cron entry.
func (j *DemoJob) Init(_ context.Context, sched *Scheduler) error {
	_, err := sched.AddFunc("@every 1m", func() {
		j.log.Debug("scheduler demo job tick")
	})
	if err != nil {
		return err
	}
	j.log.Info("scheduler demo job registered", zap.String("schedule", "@every 1m"))
	return nil
}
