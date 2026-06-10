package lifecycle

import (
	"context"
	"fmt"
	"time"
)

// StatusGetter returns the current status string for a resource, or an
// error. Implementations should be safe to call repeatedly.
type StatusGetter func(ctx context.Context) (string, error)

// WaitOptions configures WaitForStatus. Target is required. Terminal lists
// statuses that should abort the wait with an error (e.g. ERROR states).
// Interval/MaxInterval/BackoffMultiple default to 2s, 10s, 1.5x respectively
// when zero.
type WaitOptions struct {
	Target          string
	Terminal        []string
	Timeout         time.Duration
	Interval        time.Duration
	MaxInterval     time.Duration
	BackoffMultiple float64
}

const (
	defaultInterval    = 2 * time.Second
	defaultMaxInterval = 10 * time.Second
	defaultBackoff     = 1.5
)

// WaitForStatus polls get until the status matches opts.Target, any of
// opts.Terminal is hit, the context is cancelled, or opts.Timeout elapses.
// Returns the last observed status (helpful for diagnostics) and any error.
func WaitForStatus(ctx context.Context, get StatusGetter, opts WaitOptions) (string, error) {
	interval := opts.Interval
	if interval == 0 {
		interval = defaultInterval
	}
	maxInterval := opts.MaxInterval
	if maxInterval == 0 {
		maxInterval = defaultMaxInterval
	}
	multiple := opts.BackoffMultiple
	if multiple <= 1.0 {
		multiple = defaultBackoff
	}
	deadline := time.Now().Add(opts.Timeout)

	var last string
	for {
		status, err := get(ctx)
		if err != nil {
			return last, err
		}
		last = status

		if status == opts.Target {
			return status, nil
		}
		for _, t := range opts.Terminal {
			if status == t {
				return status, fmt.Errorf("resource reached terminal status %q (wanted %q)", status, opts.Target)
			}
		}
		if !time.Now().Before(deadline) {
			return status, fmt.Errorf("timed out after %s waiting for status %q (last seen %q)", opts.Timeout, opts.Target, status)
		}

		select {
		case <-ctx.Done():
			return status, ctx.Err()
		case <-time.After(interval):
		}

		next := time.Duration(float64(interval) * multiple)
		if next > maxInterval {
			next = maxInterval
		}
		interval = next
	}
}
