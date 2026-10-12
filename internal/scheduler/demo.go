package scheduler

import (
	"context"

	"github.com/ilaziness/app-tpl/internal/logger"
	"go.uber.org/zap"
)

// DemoJob is a no-op sample cron job that demonstrates Scheduler registration.
// It logs once per minute when the scheduler holds the leader lock.
type DemoJob struct{}

// NewDemoJob creates a DemoJob.
func NewDemoJob() *DemoJob {
	return &DemoJob{}
}

// Init registers a sample cron entry.
func (j *DemoJob) Init(_ context.Context, sched *Scheduler) error {
	_, err := sched.AddFunc("@every 1m", func() {
		logger.Log.Debug("scheduler demo job tick")
	})
	if err != nil {
		return err
	}
	logger.Log.Info("scheduler demo job registered", zap.String("schedule", "@every 1m"))
	return nil
}
