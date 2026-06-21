package arg_test

import (
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/internal/x"
)

func TestDefaultContract(t *testing.T) {
	newCmd := func() *xli.Command {
		def := "fallback"
		return &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "X", Optional: true, Default: &def},
			},
		}
	}

	t.Run("Get is false when not provided, even with a default", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), nil))

		_, ok := arg.Get[string](c, "X")
		x.False(ok)
	}))
	t.Run("MustGet returns the default when not provided", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), nil))
		x.Equal("fallback", arg.MustGet[string](c, "X"))
	}))
	t.Run("Get and MustGet return the provided value", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), []string{"given"}))

		v, ok := arg.Get[string](c, "X")
		x.True(ok)
		x.Equal("given", v)
		x.Equal("given", arg.MustGet[string](c, "X"))
	}))
	t.Run("MustGet panics when neither provided nor default", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "X", Optional: true},
			},
		}
		x.NoError(c.Run(t.Context(), nil))

		defer func() {
			x.NotNil(recover())
		}()
		_ = arg.MustGet[string](c, "X")
	}))
}
