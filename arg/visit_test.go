package arg_test

import (
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/internal/x"
)

func TestVisit(t *testing.T) {
	c := &xli.Command{
		Args: arg.Args{
			&arg.String{Name: "FOO"},
			&arg.Int{Name: "BAR", Optional: true},
		},
	}
	t.Run("smoke", x.F(func(x x.X) {
		err := c.Run(t.Context(), []string{"foo"})
		x.NoError(err)
	}))

	t.Run("given", x.F(func(x x.X) {
		v := ""
		ok := arg.Visit(c, "FOO", func(w string) { v = w })
		x.True(ok)
		x.Equal("foo", v)
	}))
	t.Run("not exists", x.F(func(x x.X) {
		v := ""
		ok := arg.Visit(c, "QUX", func(w string) { v = w })
		x.False(ok)
		x.Empty(v)
	}))
	t.Run("wrong type", x.F(func(x x.X) {
		ok := arg.Visit(c, "FOO", func(w int) {})
		x.False(ok)
	}))
	t.Run("not set", x.F(func(x x.X) {
		v := 0
		ok := arg.Visit(c, "BAR", func(w int) { v = w })
		x.False(ok)
		x.Empty(v)
	}))
}

func TestVisitP(t *testing.T) {
	c := &xli.Command{
		Args: arg.Args{
			&arg.String{Name: "FOO"},
		},
	}
	t.Run("smoke", x.F(func(x x.X) {
		err := c.Run(t.Context(), []string{"foo"})
		x.NoError(err)
	}))

	t.Run("given", x.F(func(x x.X) {
		v := ""
		ok := arg.VisitP(c, "FOO", &v)
		x.True(ok)
		x.Equal("foo", v)
	}))
	t.Run("dst is nil", x.F(func(x x.X) {
		ok := arg.VisitP[string](c, "FOO", nil)
		x.False(ok)
	}))
}
