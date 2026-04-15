package lex_test

import (
	"testing"

	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/lex"
)

func TestLex(t *testing.T) {
	t.Run("end of command", x.F(func(x x.X) {
		u := lex.Lex("--")
		x.IsType(lex.EndOfCommand(""), u)
	}))
	t.Run("arg", x.F(func(x x.X) {
		u := lex.Lex("bar")
		x.IsType(lex.Arg(""), u)

		v := u.(lex.Arg)
		x.Equal("bar", v.Raw())
		x.Equal(`"bar"`, v.String())
	}))
	t.Run("single dash is an arg", x.F(func(x x.X) {
		u := lex.Lex("-")
		x.IsType(lex.Arg(""), u)

		v := u.(lex.Arg)
		x.Equal("-", v.Raw())
	}))
	t.Run("flag", x.F(func(x x.X) {
		u := lex.Lex("--foo")
		x.IsType(lex.Flag(""), u)

		v := u.(lex.Flag)
		x.Equal("--foo", v.Raw())
		x.Equal("--foo", v.String())
		x.Equal("foo", v.Name())
		x.False(v.IsShort())

		_, ok := v.Arg()
		x.False(ok)
	}))
	t.Run("flag with value", x.F(func(x x.X) {
		u := lex.Lex("--foo=bar")
		x.IsType(lex.Flag(""), u)

		v := u.(lex.Flag)
		x.Equal("--foo=bar", v.Raw())
		x.Equal(`--foo="bar"`, v.String())
		x.Equal("foo", v.Name())
		x.False(v.IsShort())

		w, ok := v.Arg()
		x.True(ok)
		x.Equal("bar", w.Raw())
		x.Equal(`"bar"`, w.String())
	}))
	t.Run("short flag", x.F(func(x x.X) {
		u := lex.Lex("-foo")
		x.IsType(lex.Flag(""), u)

		v := u.(lex.Flag)
		x.Equal("-foo", v.Raw())
		x.Equal(`-foo`, v.String())
		x.Equal("foo", v.Name())
		x.True(v.IsShort())

		_, ok := v.Arg()
		x.False(ok)
	}))
	t.Run("stacked flags", x.F(func(x x.X) {
		u := lex.Lex("-abc")
		x.IsType(lex.Flag(""), u)

		v := u.(lex.Flag)
		x.True(v.IsStacked())
	}))
	t.Run("stacked flags with value", x.F(func(x x.X) {
		u := lex.Lex("-foo=bar")
		x.IsType(lex.Flag(""), u)

		v := u.(lex.Flag)
		x.Equal("-foo=bar", v.Raw())
		x.Equal(`-foo="bar"`, v.String())
		x.Equal("foo", v.Name())
		x.True(v.IsShort())

		w, ok := v.Arg()
		x.True(ok)
		x.Equal("bar", w.Raw())
		x.Equal(`"bar"`, w.String())
	}))
	t.Run("flag contains space", x.F(func(x x.X) {
		u := lex.Lex("--foo=bar baz")
		x.IsType(lex.Flag(""), u)

		v := u.(lex.Flag)
		x.Equal("--foo=bar baz", v.Raw())
		x.Equal(`--foo="bar baz"`, v.String())
		x.Equal("foo", v.Name())
		x.False(v.IsShort())

		w, ok := v.Arg()
		x.NotNil(ok)
		x.Equal("bar baz", w.Raw())
		x.Equal(`"bar baz"`, w.String())
	}))
	t.Run("too many dashes", x.F(func(x x.X) {
		tcs := []string{
			"---",
			"----",
			"---foo",
			"----foo",
			"---foo=bar",
			"----foo=bar",
		}
		for _, tc := range tcs {
			t.Log("tc:", tc)

			u := lex.Lex(tc)
			x.IsType(&lex.Err{}, u)

			v := u.(*lex.Err)
			x.Equal(tc, v.Raw())
			x.Contains(v.String(), "[!")
		}
	}))
}
