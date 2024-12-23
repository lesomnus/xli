package lex_test

import (
	"testing"

	"github.com/lesomnus/xli/lex"
	"github.com/stretchr/testify/require"
)

func TestLex(t *testing.T) {
	t.Run("end of command", func(t *testing.T) {
		u := lex.Lex("--")
		require.IsType(t, lex.EndOfCommand(""), u)
	})
	t.Run("arg", func(t *testing.T) {
		u := lex.Lex("bar")
		require.IsType(t, lex.Arg(""), u)

		v := u.(lex.Arg)
		require.Equal(t, "bar", v.Raw())
		require.Equal(t, `"bar"`, v.String())
	})
	t.Run("flag", func(t *testing.T) {
		u := lex.Lex("--foo")
		require.IsType(t, &lex.Flag{}, u)

		v := u.(*lex.Flag)
		require.Equal(t, "--foo", v.Raw())
		require.Equal(t, "--foo", v.String())
		require.Equal(t, "foo", v.Name())
		require.False(t, v.IsShort())

		w := v.Arg()
		require.Nil(t, w)
	})
	t.Run("flag with value", func(t *testing.T) {
		u := lex.Lex("--foo=bar")
		require.IsType(t, &lex.Flag{}, u)

		v := u.(*lex.Flag)
		require.Equal(t, "--foo=bar", v.Raw())
		require.Equal(t, `--foo="bar"`, v.String())
		require.Equal(t, "foo", v.Name())
		require.False(t, v.IsShort())

		w := v.Arg()
		require.NotNil(t, w)
		require.Equal(t, "bar", w.Raw())
		require.Equal(t, `"bar"`, w.String())
	})
	t.Run("short flag", func(t *testing.T) {
		u := lex.Lex("-foo")
		require.IsType(t, &lex.Flag{}, u)

		v := u.(*lex.Flag)
		require.Equal(t, "-foo", v.Raw())
		require.Equal(t, `-foo`, v.String())
		require.Equal(t, "foo", v.Name())
		require.True(t, v.IsShort())

		w := v.Arg()
		require.Nil(t, w)
	})
	t.Run("short flag with value", func(t *testing.T) {
		u := lex.Lex("-foo=bar")
		require.IsType(t, &lex.Flag{}, u)

		v := u.(*lex.Flag)
		require.Equal(t, "-foo=bar", v.Raw())
		require.Equal(t, `-foo="bar"`, v.String())
		require.Equal(t, "foo", v.Name())
		require.True(t, v.IsShort())

		w := v.Arg()
		require.NotNil(t, w)
		require.Equal(t, "bar", w.Raw())
		require.Equal(t, `"bar"`, w.String())
	})
	t.Run("flag contains space", func(t *testing.T) {
		u := lex.Lex("--foo=bar baz")
		require.IsType(t, &lex.Flag{}, u)

		v := u.(*lex.Flag)
		require.Equal(t, "--foo=bar baz", v.Raw())
		require.Equal(t, `--foo="bar baz"`, v.String())
		require.Equal(t, "foo", v.Name())
		require.False(t, v.IsShort())

		w := v.Arg()
		require.NotNil(t, w)
		require.Equal(t, "bar baz", w.Raw())
		require.Equal(t, `"bar baz"`, w.String())
	})
	t.Run("three dashes", func(t *testing.T) {
		u := lex.Lex("---foo=bar")
		require.IsType(t, &lex.Err{}, u)

		v := u.(*lex.Err)
		require.Equal(t, "---foo=bar", v.Raw())
		require.Contains(t, v.String(), "[!")
	})
	t.Run("spread", func(t *testing.T) {
		u := lex.Lex("-abc")
		require.IsType(t, &lex.Flag{}, u)

		v := u.(*lex.Flag)
		vs := v.Spread()
		require.Len(t, vs, 3)
		require.Equal(t, "-a", vs[0].Raw())
		require.Equal(t, "a", vs[0].Name())
		require.Equal(t, "b", vs[1].Raw())
		require.Equal(t, "b", vs[1].Name())
		require.Equal(t, "c", vs[2].Raw())
		require.Equal(t, "c", vs[2].Name())
	})
	t.Run("spread long", func(t *testing.T) {
		u := lex.Lex("--foo")
		require.IsType(t, &lex.Flag{}, u)

		v := u.(*lex.Flag)
		vs := v.Spread()
		require.Len(t, vs, 1)
		require.Equal(t, "--foo", vs[0].Raw())
		require.Equal(t, "foo", vs[0].Name())
	})
}
