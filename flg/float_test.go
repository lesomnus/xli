package flg_test

import (
	"testing"
	"time"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
)

func TestFloatFlag(t *testing.T) {
	t.Run("float64", x.F(func(x x.X) {
		c := &xli.Command{
			Flags: flg.Flags{&flg.Float64{Name: "ratio"}},
		}

		err := c.Run(t.Context(), []string{"--ratio=1.5"})
		x.NoError(err)

		v, ok := flg.Get[float64](c, "ratio")
		x.True(ok)
		x.Equal(1.5, v)
	}))
	t.Run("float32", x.F(func(x x.X) {
		c := &xli.Command{
			Flags: flg.Flags{&flg.Float32{Name: "ratio"}},
		}

		err := c.Run(t.Context(), []string{"--ratio=0.25"})
		x.NoError(err)

		v, ok := flg.Get[float32](c, "ratio")
		x.True(ok)
		x.Equal(float32(0.25), v)
	}))
	t.Run("invalid value is an error", x.F(func(x x.X) {
		c := &xli.Command{
			Flags: flg.Flags{&flg.Float64{Name: "ratio"}},
		}

		err := c.Run(t.Context(), []string{"--ratio=abc"})
		x.ErrorContains(err, "ratio")
	}))
}

func TestDurationFlag(t *testing.T) {
	t.Run("parses a duration", x.F(func(x x.X) {
		c := &xli.Command{
			Flags: flg.Flags{&flg.Duration{Name: "timeout"}},
		}

		err := c.Run(t.Context(), []string{"--timeout=1m30s"})
		x.NoError(err)

		v, ok := flg.Get[time.Duration](c, "timeout")
		x.True(ok)
		x.Equal(90*time.Second, v)
	}))
}
