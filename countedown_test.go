package xli_test

import (
	"testing"
	"time"

	"github.com/lesomnus/xli"
	"github.com/stretchr/testify/require"
)

func TestCountdown(t *testing.T) {
	t.Run("tick every second", func(t *testing.T) {
		ts := []time.Duration{}
		v := xli.Countdown(t.Context(), 3*time.Second, func() { <-t.Context().Done() }, func(remain time.Duration) bool {
			ts = append(ts, remain)
			return true
		})
		require.False(t, v)
		require.Equal(t, []time.Duration{
			2 * time.Second,
			1 * time.Second,
		}, ts)
	})
}
