package flg_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
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
	t.Run("smoke", x.F(func(x x.X) {
		err := c.Run(t.Context(), []string{"--foo=bar", "--foo", "baz"})
		x.NoError(err)
	}))

	t.Run("given", x.F(func(x x.X) {
		v := ""
		ok := flg.Visit(c, "foo", func(w string) { v = w })
		x.True(ok)
		x.Equal("baz", v)
	}))
	t.Run("aliased", x.F(func(x x.X) {
		err := c.Run(t.Context(), []string{"--foo=bar", "-f", "baz"})
		x.NoError(err)

		v := ""
		ok := flg.Visit(c, "foo", func(w string) { v = w })
		x.True(ok)
		x.Equal("baz", v)
	}))
	t.Run("not exists", x.F(func(x x.X) {
		v := ""
		ok := flg.Visit(c, "qux", func(w string) { v = w })
		x.False(ok)
		x.Empty(v)
	}))
	t.Run("wrong type", x.F(func(x x.X) {
		ok := flg.Visit(c, "foo", func(w int) {})
		x.False(ok)
	}))
	t.Run("not set", x.F(func(x x.X) {
		v := ""
		ok := flg.Visit(c, "qux", func(w string) { v = w })
		x.False(ok)
		x.Empty(v)
	}))
}

func TestVisitP(t *testing.T) {
	c := &xli.Command{
		Flags: flg.Flags{
			&flg.String{Name: "foo"},
		},
	}
	t.Run("smoke", x.F(func(x x.X) {
		err := c.Run(t.Context(), []string{"--foo=bar", "--foo", "baz"})
		x.NoError(err)
	}))

	t.Run("given", x.F(func(x x.X) {
		v := ""
		ok := flg.VisitP(c, "foo", &v)
		x.True(ok)
		x.Equal("baz", v)
	}))
	t.Run("dst is nil", x.F(func(x x.X) {
		ok := flg.VisitP[string](c, "foo", nil)
		x.False(ok)
	}))
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

	t.Run("exist in current", x.F(func(x x.X) {
		ok := false
		v := ""
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			ok = flg.Lookup(cmd, "val", func(w string) { v = w })
			return nil
		})

		err := c.Run(t.Context(), []string{"a", "b", "--val=foo"})
		x.NoError(err)
		x.True(ok)
		x.Equal("foo", v)
	}))
	t.Run("exist in parent", x.F(func(x x.X) {
		ok := false
		v := ""
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			ok = flg.Lookup(cmd, "val", func(w string) { v = w })
			return nil
		})

		err := c.Run(t.Context(), []string{"a", "--val=bar", "b"})
		x.NoError(err)
		x.True(ok)
		x.Equal("bar", v)
	}))
	t.Run("exist in ancestor", x.F(func(x x.X) {
		ok := false
		v := ""
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			ok = flg.Lookup(cmd, "val", func(w string) { v = w })
			return nil
		})

		err := c.Run(t.Context(), []string{"--val=baz", "a", "b"})
		x.NoError(err)
		x.True(ok)
		x.Equal("baz", v)
	}))
	t.Run("nearest one", x.F(func(x x.X) {
		ok := false
		v := ""
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			ok = flg.Lookup(cmd, "val", func(w string) { v = w })
			return nil
		})

		err := c.Run(t.Context(), []string{"--val=baz", "a", "b", "--val=foo"})
		x.NoError(err)
		x.True(ok)
		x.Equal("foo", v)
	}))
	t.Run("not exist", x.F(func(x x.X) {
		ok := false
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			ok = flg.Lookup(cmd, "val", func(w string) {})
			return nil
		})

		err := c.Run(t.Context(), []string{"a", "b"})
		x.NoError(err)
		x.False(ok)
	}))
}
