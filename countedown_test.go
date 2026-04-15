package xli_test

import (
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
}
