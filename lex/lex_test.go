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
	t.Run("single dash is an arg", func(t *testing.T) {
		u := lex.Lex("-")
		require.IsType(t, lex.Arg(""), u)

		v := u.(lex.Arg)
		require.Equal(t, "-", v.Raw())
	})
	t.Run("flag", func(t *testing.T) {
		u := lex.Lex("--foo")
		require.IsType(t, lex.Flag(""), u)

		v := u.(lex.Flag)
		require.Equal(t, "--foo", v.Raw())
		require.Equal(t, "--foo", v.String())
		require.Equal(t, "foo", v.Name())
		require.False(t, v.IsShort())

		_, ok := v.Arg()
		require.False(t, ok)
	})
	t.Run("flag with value", func(t *testing.T) {
		u := lex.Lex("--foo=bar")
		require.IsType(t, lex.Flag(""), u)

		v := u.(lex.Flag)
		require.Equal(t, "--foo=bar", v.Raw())
		require.Equal(t, `--foo="bar"`, v.String())
		require.Equal(t, "foo", v.Name())
		require.False(t, v.IsShort())

		w, ok := v.Arg()
		require.True(t, ok)
		require.Equal(t, "bar", w.Raw())
		require.Equal(t, `"bar"`, w.String())
	})
	t.Run("short flag", func(t *testing.T) {
		u := lex.Lex("-foo")
		require.IsType(t, lex.Flag(""), u)

		v := u.(lex.Flag)
		require.Equal(t, "-foo", v.Raw())
		require.Equal(t, `-foo`, v.String())
		require.Equal(t, "foo", v.Name())
		require.True(t, v.IsShort())

		_, ok := v.Arg()
		require.False(t, ok)
	})
	t.Run("stacked flags", func(t *testing.T) {
		u := lex.Lex("-abc")
		require.IsType(t, lex.Flag(""), u)

		v := u.(lex.Flag)
		require.True(t, v.IsStacked())
	})
	t.Run("stacked flags with value", func(t *testing.T) {
		u := lex.Lex("-foo=bar")
		require.IsType(t, lex.Flag(""), u)

		v := u.(lex.Flag)
		require.Equal(t, "-foo=bar", v.Raw())
		require.Equal(t, `-foo="bar"`, v.String())
		require.Equal(t, "foo", v.Name())
		require.True(t, v.IsShort())

		w, ok := v.Arg()
		require.True(t, ok)
		require.Equal(t, "bar", w.Raw())
		require.Equal(t, `"bar"`, w.String())
	})
	t.Run("flag contains space", func(t *testing.T) {
		u := lex.Lex("--foo=bar baz")
		require.IsType(t, lex.Flag(""), u)

		v := u.(lex.Flag)
		require.Equal(t, "--foo=bar baz", v.Raw())
		require.Equal(t, `--foo="bar baz"`, v.String())
		require.Equal(t, "foo", v.Name())
		require.False(t, v.IsShort())

		w, ok := v.Arg()
		require.NotNil(t, ok)
		require.Equal(t, "bar baz", w.Raw())
		require.Equal(t, `"bar baz"`, w.String())
	})
	t.Run("too many dashes", func(t *testing.T) {
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
			require.IsType(t, &lex.Err{}, u)

			v := u.(*lex.Err)
			require.Equal(t, tc, v.Raw())
			require.Contains(t, v.String(), "[!")
		}
	})
}
