package xli

import (
	"context"
	"time"
)

// Countdown calls `until` and invokes `tick` every second until `until` is finished.
// Returns true if `until` is finished; otherwise, returns false.
// Note that `tick` is invoked with `d` at first.
func Countdown(ctx context.Context, d time.Duration, until func(), tick func(remain time.Duration) bool) bool {
	done := make(chan struct{})

	end := time.Now().Add(d)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	go func() {
		defer close(done)
		until()
	}()

	tick(d)
	for {
		select {
		case <-done:
			return true

		case <-ctx.Done():
			return false
		case <-ticker.C:
			dt := time.Until(end).Round(time.Second)
			if dt <= 0 {
				return false
			}
			if !tick(dt) {
				return false
			}
		}
	}

	return false
}
