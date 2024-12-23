package arg_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/stretchr/testify/require"
)

func TestVisit(t *testing.T) {
	c := &xli.Command{
		Args: arg.Args{
			&arg.String{Name: "FOO"},
			&arg.Int{Name: "BAR", Optional: true},
		},
	}
	_, err := c.Run(context.TODO(), []string{"foo"})
	require.NoError(t, err)

	t.Run("given", func(t *testing.T) {
		v := ""
		ok := arg.Visit(c, "FOO", func(w string) { v = w })
		require.True(t, ok)
		require.Equal(t, "foo", v)
	})
	t.Run("not exists", func(t *testing.T) {
		v := ""
		ok := arg.Visit(c, "QUX", func(w string) { v = w })
		require.False(t, ok)
		require.Empty(t, v)
	})
	t.Run("wrong type", func(t *testing.T) {
		ok := arg.Visit(c, "FOO", func(w int) {})
		require.False(t, ok)
	})
	t.Run("not set", func(t *testing.T) {
		v := 0
		ok := arg.Visit(c, "BAR", func(w int) { v = w })
		require.False(t, ok)
		require.Empty(t, v)
	})
}

func TestVisitP(t *testing.T) {
	c := &xli.Command{
		Args: arg.Args{
			&arg.String{Name: "FOO"},
		},
	}
	_, err := c.Run(context.TODO(), []string{"foo"})
	require.NoError(t, err)

	t.Run("given", func(t *testing.T) {
		v := ""
		ok := arg.VisitP(c, "FOO", &v)
		require.True(t, ok)
		require.Equal(t, "foo", v)
	})
	t.Run("dst is nil", func(t *testing.T) {
		ok := arg.VisitP[string](c, "FOO", nil)
		require.False(t, ok)
	})
}
