package lex_test

import (
	"testing"

	"github.com/lesomnus/xli/lex"
	"github.com/stretchr/testify/require"
)

func TestFlagWithArg(t *testing.T) {
	t.Run("long with no arg", func(t *testing.T) {
		v := lex.Flag("--foo")
		v = v.WithArg("bar")
		require.Equal(t, "--foo=bar", string(v))
	})
	t.Run("long with arg", func(t *testing.T) {
		v := lex.Flag("--foo=baz")
		v = v.WithArg("bar")
		require.Equal(t, "--foo=bar", string(v))
	})
	t.Run("short with no arg", func(t *testing.T) {
		v := lex.Flag("-foo")
		v = v.WithArg("bar")
		require.Equal(t, "-foo=bar", string(v))
	})
	t.Run("short with arg", func(t *testing.T) {
		v := lex.Flag("-foo=baz")
		v = v.WithArg("bar")
		require.Equal(t, "-foo=bar", string(v))
	})
}

func TestFlagSpread(t *testing.T) {
	t.Run("stacked flags", func(t *testing.T) {
		v := lex.Flag("-abc")
		vs := v.Spread()
		require.Len(t, vs, 3)
		require.Equal(t, "-a", vs[0].Raw())
		require.Equal(t, "a", vs[0].Name())

		_, ok := vs[0].Arg()
		require.False(t, ok)

		require.Equal(t, "b", vs[1].Raw())
		require.Equal(t, "b", vs[1].Name())

		_, ok = vs[1].Arg()
		require.False(t, ok)

		require.Equal(t, "c", vs[2].Raw())
		require.Equal(t, "c", vs[2].Name())

		_, ok = vs[2].Arg()
		require.False(t, ok)
	})
	t.Run("stacked flags with value", func(t *testing.T) {
		v := lex.Flag("-abc=foo")
		vs := v.Spread()
		require.Len(t, vs, 3)
		require.Equal(t, "-a", vs[0].Raw())
		require.Equal(t, "a", vs[0].Name())

		_, ok := vs[0].Arg()
		require.False(t, ok)

		require.Equal(t, "b", vs[1].Raw())
		require.Equal(t, "b", vs[1].Name())

		_, ok = vs[1].Arg()
		require.False(t, ok)

		require.Equal(t, "c=foo", vs[2].Raw())
		require.Equal(t, "c", vs[2].Name())

		w, ok := vs[2].Arg()
		require.True(t, ok)
		require.Equal(t, "foo", w.Raw())
	})
	t.Run("long", func(t *testing.T) {
		v := lex.Flag("--foo")
		vs := v.Spread()
		require.Len(t, vs, 1)
		require.Equal(t, "--foo", vs[0].Raw())
		require.Equal(t, "foo", vs[0].Name())
	})
}
