package recovery

import (
	"context"
	"errors"
	"time"
)

// ErrNilDispatcher is returned when a runner is invoked without a Dispatcher.
var ErrNilDispatcher = errors.New("dispatcher is required")

// RunnerConfig configures a recovery runner loop.
type RunnerConfig struct {
	Dispatcher   *Dispatcher
	PollInterval time.Duration
	Now          func() time.Time
	Sleep        func(context.Context, time.Duration) error
}

// Runner executes the recovery dispatcher step loop until a terminal state
// is reached or the context is cancelled.
type Runner struct {
	Dispatcher   *Dispatcher
	PollInterval time.Duration
	Now          func() time.Time
	Sleep        func(context.Context, time.Duration) error
}

// NewRunner creates a new Runner from configuration.
func NewRunner(cfg RunnerConfig) *Runner {
	return &Runner{
		Dispatcher:   cfg.Dispatcher,
		PollInterval: cfg.PollInterval,
		Now:          cfg.Now,
		Sleep:        cfg.Sleep,
	}
}

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	if r.Dispatcher != nil && r.Dispatcher.Now != nil {
		return r.Dispatcher.Now()
	}
	return time.Now()
}

func (r *Runner) sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.Sleep != nil {
		return r.Sleep(ctx, d)
	}
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Run executes the recovery loop until the task reaches a terminal state or ctx is done.
func (r *Runner) Run(ctx context.Context) (Checkpoint, error) {
	if r == nil || r.Dispatcher == nil {
		return Checkpoint{}, ErrNilDispatcher
	}

	var lastCP Checkpoint
	for {
		if err := ctx.Err(); err != nil {
			return lastCP, err
		}

		cp, stepErr := r.Dispatcher.Step(ctx)
		if cp.TaskID != "" || cp.State != "" {
			lastCP = cp
		}

		if cp.Terminal() {
			return cp, nil
		}

		if err := ctx.Err(); err != nil {
			return lastCP, err
		}

		now := r.now()
		waited := false

		if (cp.State == StateWaitingRetry || cp.State == StateDeliveryUncertain || cp.State == StateDelivered) && cp.NextRetryAt != nil {
			delay := cp.NextRetryAt.Sub(now)
			if delay > 0 {
				waited = true
				if err := r.sleep(ctx, delay); err != nil {
					return lastCP, err
				}
			}
		}

		if !waited {
			if stepErr != nil && cp.State == "" {
				return lastCP, stepErr
			}
			if r.PollInterval > 0 && isWaitingOrDelivered(cp.State) {
				if err := r.sleep(ctx, r.PollInterval); err != nil {
					return lastCP, err
				}
			}
		}
	}
}

func isWaitingOrDelivered(state TaskState) bool {
	return state == StateDelivered || state == StateWaitingRetry || state == StateDeliveryUncertain || state == StatePending
}

// Run is a convenience function that executes a recovery loop with the given dispatcher and poll interval.
func Run(ctx context.Context, d *Dispatcher, pollInterval time.Duration) (Checkpoint, error) {
	runner := &Runner{
		Dispatcher:   d,
		PollInterval: pollInterval,
	}
	return runner.Run(ctx)
}
