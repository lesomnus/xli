package lex_test

import (
	"testing"

	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/lex"
)

func TestFlagWithArg(t *testing.T) {
	t.Run("long with no arg", x.F(func(x x.X) {
		v := lex.Flag("--foo")
		v = v.WithArg("bar")
		x.Equal("--foo=bar", string(v))
	}))
	t.Run("long with arg", x.F(func(x x.X) {
		v := lex.Flag("--foo=baz")
		v = v.WithArg("bar")
		x.Equal("--foo=bar", string(v))
	}))
	t.Run("short with no arg", x.F(func(x x.X) {
		v := lex.Flag("-foo")
		v = v.WithArg("bar")
		x.Equal("-foo=bar", string(v))
	}))
	t.Run("short with arg", x.F(func(x x.X) {
		v := lex.Flag("-foo=baz")
		v = v.WithArg("bar")
		x.Equal("-foo=bar", string(v))
	}))
}

func TestFlagSpread(t *testing.T) {
	t.Run("stacked flags", x.F(func(x x.X) {
		v := lex.Flag("-abc")
		vs := v.Spread()
		x.Len(vs, 3)
		x.Equal("-a", vs[0].Raw())
		x.Equal("a", vs[0].Name())

		_, ok := vs[0].Arg()
		x.False(ok)

		x.Equal("b", vs[1].Raw())
		x.Equal("b", vs[1].Name())

		_, ok = vs[1].Arg()
		x.False(ok)

		x.Equal("c", vs[2].Raw())
		x.Equal("c", vs[2].Name())

		_, ok = vs[2].Arg()
		x.False(ok)
	}))
	t.Run("stacked flags with value", x.F(func(x x.X) {
		v := lex.Flag("-abc=foo")
		vs := v.Spread()
		x.Len(vs, 3)
		x.Equal("-a", vs[0].Raw())
		x.Equal("a", vs[0].Name())

		_, ok := vs[0].Arg()
		x.False(ok)

		x.Equal("b", vs[1].Raw())
		x.Equal("b", vs[1].Name())

		_, ok = vs[1].Arg()
		x.False(ok)

		x.Equal("c=foo", vs[2].Raw())
		x.Equal("c", vs[2].Name())

		w, ok := vs[2].Arg()
		x.True(ok)
		x.Equal("foo", w.Raw())
	}))
	t.Run("long", x.F(func(x x.X) {
		v := lex.Flag("--foo")
		vs := v.Spread()
		x.Len(vs, 1)
		x.Equal("--foo", vs[0].Raw())
		x.Equal("foo", vs[0].Name())
	}))
}
