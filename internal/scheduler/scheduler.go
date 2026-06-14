// Package scheduler runs the platform's periodic enforcement loop: at each tick
// it asks the orchestration layer to process cycle deadlines (record defaults +
// settle). This is the heartbeat that turns "a deadline passed" into
// deterministic on-chain default events and payouts.
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// DeadlineProcessor is the slice of the app service the scheduler needs.
type DeadlineProcessor interface {
	ProcessDeadlines(ctx context.Context, now time.Time) (int, error)
}

// Scheduler periodically invokes deadline processing.
type Scheduler struct {
	proc     DeadlineProcessor
	interval time.Duration
	log      *slog.Logger
	now      func() time.Time
}

// New builds a Scheduler that ticks every interval.
func New(proc DeadlineProcessor, interval time.Duration, log *slog.Logger) *Scheduler {
	return &Scheduler{proc: proc, interval: interval, log: log, now: time.Now}
}

// Run blocks, ticking until ctx is cancelled. Intended for its own goroutine.
func (s *Scheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	s.log.Info("scheduler started", "interval", s.interval.String())
	for {
		select {
		case <-ctx.Done():
			s.log.Info("scheduler stopped")
			return ctx.Err()
		case <-ticker.C:
			n, err := s.proc.ProcessDeadlines(ctx, s.now().UTC())
			if err != nil {
				s.log.Error("process deadlines", "err", err)
				continue
			}
			if n > 0 {
				s.log.Info("scheduler settled cycles", "count", n)
			}
		}
	}
}
