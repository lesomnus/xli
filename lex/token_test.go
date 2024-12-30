package lex_test

import (
	"testing"

	"github.com/lesomnus/xli/lex"
	"github.com/stretchr/testify/require"
)

func TestFlagSpread(t *testing.T) {
	t.Run("stacked flags", func(t *testing.T) {
		u := lex.Lex("-abc")
		require.IsType(t, &lex.Flag{}, u)

		v := u.(*lex.Flag)
		vs := v.Spread()
		require.Len(t, vs, 3)
		require.Equal(t, "-a", vs[0].Raw())
		require.Equal(t, "a", vs[0].Name())
		require.Nil(t, vs[0].Arg())
		require.Equal(t, "b", vs[1].Raw())
		require.Equal(t, "b", vs[1].Name())
		require.Nil(t, vs[1].Arg())
		require.Equal(t, "c", vs[2].Raw())
		require.Equal(t, "c", vs[2].Name())
		require.Nil(t, vs[2].Arg())
	})
	t.Run("stacked flags with value", func(t *testing.T) {
		u := lex.Lex("-abc=foo")
		require.IsType(t, &lex.Flag{}, u)

		v := u.(*lex.Flag)
		vs := v.Spread()
		require.Len(t, vs, 3)
		require.Equal(t, "-a", vs[0].Raw())
		require.Equal(t, "a", vs[0].Name())
		require.Nil(t, vs[0].Arg())
		require.Equal(t, "b", vs[1].Raw())
		require.Equal(t, "b", vs[1].Name())
		require.Nil(t, vs[1].Arg())
		require.Equal(t, "c=foo", vs[2].Raw())
		require.Equal(t, "c", vs[2].Name())
		require.NotNil(t, vs[2].Arg())
		require.Equal(t, "foo", vs[2].Arg().Raw())
	})
	t.Run("long", func(t *testing.T) {
		u := lex.Lex("--foo")
		require.IsType(t, &lex.Flag{}, u)

		v := u.(*lex.Flag)
		vs := v.Spread()
		require.Len(t, vs, 1)
		require.Equal(t, "--foo", vs[0].Raw())
		require.Equal(t, "foo", vs[0].Name())
	})
}
