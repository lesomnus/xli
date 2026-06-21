package arg_test

import (
	"testing"
	"time"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/internal/x"
)

func TestFloatArg(t *testing.T) {
	t.Run("float64", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{&arg.Float64{Name: "RATIO"}},
		}

		err := c.Run(t.Context(), []string{"1.5"})
		x.NoError(err)

		v, ok := arg.Get[float64](c, "RATIO")
		x.True(ok)
		x.Equal(1.5, v)
	}))
	t.Run("invalid value is an error", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{&arg.Float64{Name: "RATIO"}},
		}

		err := c.Run(t.Context(), []string{"abc"})
		x.ErrorContains(err, "invalid argument")
	}))
}

func TestDurationArg(t *testing.T) {
	t.Run("parses a duration", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{&arg.Duration{Name: "TIMEOUT"}},
		}

		err := c.Run(t.Context(), []string{"1m30s"})
		x.NoError(err)

		v, ok := arg.Get[time.Duration](c, "TIMEOUT")
		x.True(ok)
		x.Equal(90*time.Second, v)
	}))
}
