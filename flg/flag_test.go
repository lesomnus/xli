package flg_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
	"github.com/stretchr/testify/require"
)

func TestVisit(t *testing.T) {
	c := &xli.Command{
		Flags: xli.Flags{
			&flg.String{Name: "foo"},
			&flg.String{Name: "qux"},
		},
	}
	_, err := c.Run(context.TODO(), []string{"--foo=bar", "--foo", "baz"})
	require.NoError(t, err)

	t.Run("given", func(t *testing.T) {
		v := ""
		ok := flg.Visit(c, "foo", func(w string) { v = w })
		require.True(t, ok)
		require.Equal(t, "baz", v)
	})
	t.Run("not exists", func(t *testing.T) {
		v := ""
		ok := flg.Visit(c, "qux", func(w string) { v = w })
		require.False(t, ok)
		require.Empty(t, v)
	})
	t.Run("wrong type", func(t *testing.T) {
		ok := flg.Visit(c, "foo", func(w int) {})
		require.False(t, ok)
	})
	t.Run("not set", func(t *testing.T) {
		v := ""
		ok := flg.Visit(c, "qux", func(w string) { v = w })
		require.False(t, ok)
		require.Empty(t, v)
	})
}
