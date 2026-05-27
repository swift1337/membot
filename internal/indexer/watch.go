package indexer

import (
	"context"
	"errors"
	"time"

	"github.com/swift1337/membot/internal/store"
)

type WatchOptions struct {
	Interval time.Duration
	Once     bool
	LockPath string
	AllOptions
}

type WatchRun struct {
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt time.Time  `json:"finished_at"`
	NextRunAt  *time.Time `json:"next_run_at,omitempty"`
	Skipped    bool       `json:"skipped"`
	LockHeld   bool       `json:"lock_held"`
	Result     *AllResult `json:"result,omitempty"`
	Error      string     `json:"error,omitempty"`
}

func Watch(ctx context.Context, st *store.Store, opts WatchOptions, emit func(WatchRun)) error {
	if opts.Interval <= 0 {
		opts.Interval = 120 * time.Second
	}

	runOnce := func() error {
		startedAt := time.Now().UTC()
		run := WatchRun{StartedAt: startedAt}

		lock, err := AcquireLock(opts.LockPath)
		if err != nil {
			if errors.Is(err, ErrLockHeld) {
				run.Skipped = true
				run.LockHeld = true
				run.FinishedAt = time.Now().UTC()
				if !opts.Once {
					next := startedAt.Add(opts.Interval).UTC()
					run.NextRunAt = &next
				}
				emit(run)
				return nil
			}
			run.Error = err.Error()
			run.FinishedAt = time.Now().UTC()
			emit(run)
			return err
		}
		defer func() {
			_ = lock.Release()
		}()

		result, err := IndexAll(ctx, st, opts.AllOptions)
		run.FinishedAt = time.Now().UTC()
		if err != nil {
			run.Error = err.Error()
			emit(run)
			return err
		}
		run.Result = &result
		if !opts.Once {
			next := run.FinishedAt.Add(opts.Interval).UTC()
			run.NextRunAt = &next
		}
		emit(run)
		return nil
	}

	if err := runOnce(); err != nil {
		return err
	}
	if opts.Once {
		return nil
	}

	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := runOnce(); err != nil {
				return err
			}
		}
	}
}
