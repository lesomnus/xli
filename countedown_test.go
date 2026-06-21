package xli_test

import (
	"context"
	"testing"
	"time"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/internal/x"
)

func TestCountdown(t *testing.T) {
	t.Run("tick every second", x.F(func(x x.X) {
		t0 := time.Now()
		ts := []time.Duration{}
		d := 3 * time.Second
		v := xli.Countdown(t.Context(), d, func() { <-t.Context().Done() }, func(remain time.Duration) bool {
			dt := time.Since(t0).Round(time.Second)
			ts = append(ts, remain)
			x.Equal(dt, d-remain)
			return true
		})
		x.False(v)
		x.Equal([]time.Duration{
			3 * time.Second,
			2 * time.Second,
			1 * time.Second,
		}, ts)
	}))
	t.Run("returns true when until finishes", x.F(func(x x.X) {
		v := xli.Countdown(t.Context(), time.Hour, func() {}, func(remain time.Duration) bool {
			return true
		})
		x.True(v)
	}))
	t.Run("returns false when context is cancelled", x.F(func(x x.X) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		block := make(chan struct{})
		v := xli.Countdown(ctx, time.Hour, func() { <-block }, func(remain time.Duration) bool {
			return true
		})
		x.False(v)
		close(block)
	}))
}
