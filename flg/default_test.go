package flg_test

import (
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
)

func TestDefaultContract(t *testing.T) {
	newCmd := func() *xli.Command {
		def := "fallback"
		return &xli.Command{
			Flags: flg.Flags{
				&flg.String{Name: "x", Default: &def},
			},
		}
	}

	t.Run("Get is false when not provided, even with a default", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), nil))

		_, ok := flg.Get[string](c, "x")
		x.False(ok)
	}))
	t.Run("MustGet returns the default when not provided", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), nil))
		x.Equal("fallback", flg.MustGet[string](c, "x"))
	}))
	t.Run("Get and MustGet return the provided value", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), []string{"--x=given"}))

		v, ok := flg.Get[string](c, "x")
		x.True(ok)
		x.Equal("given", v)
		x.Equal("given", flg.MustGet[string](c, "x"))
	}))
	t.Run("VisitP writes only when provided", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), nil))

		dst := "untouched"
		ok := flg.VisitP(c, "x", &dst)
		x.False(ok)
		x.Equal("untouched", dst)
	}))
	t.Run("MustGet panics when neither provided nor default", x.F(func(x x.X) {
		c := &xli.Command{
			Flags: flg.Flags{
				&flg.String{Name: "x"},
			},
		}
		x.NoError(c.Run(t.Context(), nil))

		defer func() {
			x.NotNil(recover())
		}()
		_ = flg.MustGet[string](c, "x")
	}))
}
