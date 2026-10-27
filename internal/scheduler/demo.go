package scheduler

import (
	"context"

	pkgscheduler "github.com/ilaziness/app-tpl/pkg/scheduler"
	"go.uber.org/zap"
)

// DemoJob is a no-op sample cron job that demonstrates Job registration.
// It logs once per minute when the scheduler holds the leader lock.
type DemoJob struct {
	log *zap.Logger
}

// NewDemoJob creates a DemoJob.
func NewDemoJob(log *zap.Logger) *DemoJob {
	return &DemoJob{log: log}
}

// Init registers a sample cron entry.
func (j *DemoJob) Init(_ context.Context, c pkgscheduler.Cron) error {
	_, err := c.AddFunc("@every 1m", func() {
		j.log.Debug("scheduler demo job tick")
	})
	if err != nil {
		return err
	}
	j.log.Info("scheduler demo job registered", zap.String("schedule", "@every 1m"))
	return nil
}
