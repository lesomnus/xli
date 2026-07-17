package arg_test

import (
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/internal/x"
)

func TestRestContract(t *testing.T) {
	t.Run("Get returns values and true after parse", x.F(func(x x.X) {
		a := &arg.RestStrings{Name: "X"}
		n, err := a.Parse([]string{"foo", "bar"})
		x.NoError(err)
		x.Equal(2, n)

		vs, ok := a.Get()
		x.True(ok)
		x.Equal([]string{"foo", "bar"}, vs)
	}))
	t.Run("Get returns false with no values", x.F(func(x x.X) {
		a := &arg.RestStrings{Name: "X"}
		vs, ok := a.Get()
		x.False(ok)
		x.Nil(vs)
	}))
	t.Run("IsOptional and IsMany and String", x.F(func(x x.X) {
		a := &arg.RestStrings{Name: "X"}
		x.True(a.IsOptional())
		x.True(a.IsMany())
		x.Equal("[X...]", a.String())
	}))
	t.Run("MustGet returns default when none provided", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{&arg.RestStrings{Name: "X", Default: []string{"d1", "d2"}}},
		}
		err := c.Run(t.Context(), []string{})
		x.NoError(err)

		got := arg.MustGet[[]string](c, "X")
		x.Equal([]string{"d1", "d2"}, got)
	}))
	t.Run("MustGet returns parsed values over default", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{&arg.RestStrings{Name: "X", Default: []string{"d1"}}},
		}
		err := c.Run(t.Context(), []string{"a", "b"})
		x.NoError(err)

		got := arg.MustGet[[]string](c, "X")
		x.Equal([]string{"a", "b"}, got)
	}))
	t.Run("MustGet panics with no default and no values", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{&arg.RestStrings{Name: "X"}},
		}
		err := c.Run(t.Context(), []string{})
		x.NoError(err)

		defer func() { x.NotNil(recover()) }()
		arg.MustGet[[]string](c, "X")
	}))
}

func TestRestParserError(t *testing.T) {
	t.Run("Parse returns error on invalid element", x.F(func(x x.X) {
		p := arg.RestInts{}.Parser
		vs, _, err := p.Parse([]string{"1", "notnum"})
		x.NotNil(err)
		x.Nil(vs)
	}))
	t.Run("String is element type with ellipsis", x.F(func(x x.X) {
		p := arg.RestInts{}.Parser
		x.Equal("int...", p.String())
	}))
}
