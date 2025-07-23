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
		Flags: flg.Flags{
			&flg.String{
				Name:  "foo",
				Alias: 'f',
			},
			&flg.String{Name: "qux"},
		},
	}
	err := c.Run(t.Context(), []string{"--foo=bar", "--foo", "baz"})
	require.NoError(t, err)

	t.Run("given", func(t *testing.T) {
		v := ""
		ok := flg.Visit(c, "foo", func(w string) { v = w })
		require.True(t, ok)
		require.Equal(t, "baz", v)
	})
	t.Run("aliased", func(t *testing.T) {
		err := c.Run(t.Context(), []string{"--foo=bar", "-f", "baz"})
		require.NoError(t, err)

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

func TestVisitP(t *testing.T) {
	c := &xli.Command{
		Flags: flg.Flags{
			&flg.String{Name: "foo"},
		},
	}
	err := c.Run(t.Context(), []string{"--foo=bar", "--foo", "baz"})
	require.NoError(t, err)

	t.Run("given", func(t *testing.T) {
		v := ""
		ok := flg.VisitP(c, "foo", &v)
		require.True(t, ok)
		require.Equal(t, "baz", v)
	})
	t.Run("dst is nil", func(t *testing.T) {
		ok := flg.VisitP[string](c, "foo", nil)
		require.False(t, ok)
	})
}

func TestLookupP(t *testing.T) {
	make_cmd := func(f xli.HandlerFunc) *xli.Command {
		return &xli.Command{
			Flags: flg.Flags{
				&flg.String{Name: "val"},
			},
			Commands: xli.Commands{
				&xli.Command{
					Name: "a",
					Flags: flg.Flags{
						&flg.String{Name: "val"},
					},
					Commands: xli.Commands{
						&xli.Command{
							Name: "b",
							Flags: flg.Flags{
								&flg.String{Name: "val"},
							},
							Handler: xli.Handle(f),
						},
					},
				},
			},
		}
	}

	t.Run("exist in current", func(t *testing.T) {
		ok := false
		v := ""
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			ok = flg.Lookup(cmd, "val", func(w string) { v = w })
			return nil
		})

		err := c.Run(t.Context(), []string{"a", "b", "--val=foo"})
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "foo", v)
	})
	t.Run("exist in parent", func(t *testing.T) {
		ok := false
		v := ""
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			ok = flg.Lookup(cmd, "val", func(w string) { v = w })
			return nil
		})

		err := c.Run(t.Context(), []string{"a", "--val=bar", "b"})
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "bar", v)
	})
	t.Run("exist in ancestor", func(t *testing.T) {
		ok := false
		v := ""
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			ok = flg.Lookup(cmd, "val", func(w string) { v = w })
			return nil
		})

		err := c.Run(t.Context(), []string{"--val=baz", "a", "b"})
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "baz", v)
	})
	t.Run("nearest one", func(t *testing.T) {
		ok := false
		v := ""
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			ok = flg.Lookup(cmd, "val", func(w string) { v = w })
			return nil
		})

		err := c.Run(t.Context(), []string{"--val=baz", "a", "b", "--val=foo"})
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, "foo", v)
	})
	t.Run("not exist", func(t *testing.T) {
		ok := false
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			ok = flg.Lookup(cmd, "val", func(w string) {})
			return nil
		})

		err := c.Run(t.Context(), []string{"a", "b"})
		require.NoError(t, err)
		require.False(t, ok)
	})
}
