package mode_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/mode"
)

func TestMode(t *testing.T) {
	t.Run("Is matches all set bits", x.F(func(x x.X) {
		m := mode.Run | mode.Pass
		x.True(m.Is(mode.Run))
		x.True(m.Is(mode.Pass))
		x.True(m.Is(mode.Run | mode.Pass))
		x.False(m.Is(mode.Help))
	}))
	t.Run("NoPass clears the Pass bit", x.F(func(x x.X) {
		m := mode.Run | mode.Pass
		x.Equal(mode.Run, m.NoPass())
		x.False(m.NoPass().Is(mode.Pass))
	}))
	t.Run("From returns Unspecified when absent", x.F(func(x x.X) {
		x.Equal(mode.Unspecified, mode.From(context.Background()))
	}))
	t.Run("Into and From round-trip", x.F(func(x x.X) {
		ctx := mode.Into(context.Background(), mode.Help)
		x.Equal(mode.Help, mode.From(ctx))
	}))
}
