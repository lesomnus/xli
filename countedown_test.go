package xli_test

import (
	"testing"
	"time"

	"github.com/lesomnus/xli"
	"github.com/stretchr/testify/require"
)

func TestCountdown(t *testing.T) {
	t.Run("tick every second", func(t *testing.T) {
		t0 := time.Now()
		ts := []time.Duration{}
		d := 3 * time.Second
		v := xli.Countdown(t.Context(), d, func() { <-t.Context().Done() }, func(remain time.Duration) bool {
			dt := time.Since(t0).Round(time.Second)
			ts = append(ts, remain)
			require.Equal(t, dt, d-remain)
			return true
		})
		require.False(t, v)
		require.Equal(t, []time.Duration{
			3 * time.Second,
			2 * time.Second,
			1 * time.Second,
		}, ts)
	})
}
