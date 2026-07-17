package flg_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
)

func TestFind(t *testing.T) {
	make_cmd := func(f xli.HandlerFunc, rootDefault *string) *xli.Command {
		return &xli.Command{
			Flags: flg.Flags{
				&flg.String{Name: "val", Default: rootDefault},
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

	t.Run("found in current", x.F(func(x x.X) {
		ok := false
		v := ""
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			v, ok = flg.Find[string, *xli.Command](cmd, "val")
			return nil
		}, nil)

		err := c.Run(t.Context(), []string{"a", "b", "--val=foo"})
		x.NoError(err)
		x.True(ok)
		x.Equal("foo", v)
	}))
	t.Run("found in ancestor", x.F(func(x x.X) {
		ok := false
		v := ""
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			v, ok = flg.Find[string, *xli.Command](cmd, "val")
			return nil
		}, nil)

		err := c.Run(t.Context(), []string{"--val=baz", "a", "b"})
		x.NoError(err)
		x.True(ok)
		x.Equal("baz", v)
	}))
	t.Run("not found", x.F(func(x x.X) {
		ok := true
		v := "unchanged"
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			v, ok = flg.Find[string, *xli.Command](cmd, "val")
			return nil
		}, nil)

		err := c.Run(t.Context(), []string{"a", "b"})
		x.NoError(err)
		x.False(ok)
		x.Empty(v)
	}))
}

func TestMustFind(t *testing.T) {
	make_cmd := func(f xli.HandlerFunc, rootDefault *string) *xli.Command {
		return &xli.Command{
			Flags: flg.Flags{
				&flg.String{Name: "val", Default: rootDefault},
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

	t.Run("found in ancestor", x.F(func(x x.X) {
		v := ""
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			v = flg.MustFind[string, *xli.Command](cmd, "val")
			return nil
		}, nil)

		err := c.Run(t.Context(), []string{"--val=baz", "a", "b"})
		x.NoError(err)
		x.Equal("baz", v)
	}))
	t.Run("falls back to ancestor default", x.F(func(x x.X) {
		d := "def"
		v := ""
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			v = flg.MustFind[string, *xli.Command](cmd, "val")
			return nil
		}, &d)

		err := c.Run(t.Context(), []string{"a", "b"})
		x.NoError(err)
		x.Equal("def", v)
	}))
	t.Run("panics when no value and no default", x.F(func(x x.X) {
		c := make_cmd(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			defer func() { x.NotNil(recover()) }()
			_ = flg.MustFind[string, *xli.Command](cmd, "val")
			return nil
		}, nil)

		err := c.Run(t.Context(), []string{"a", "b"})
		x.NoError(err)
	}))
}

func TestLookupPNilDst(t *testing.T) {
	t.Run("dst is nil", x.F(func(x x.X) {
		ok := true
		c := &xli.Command{
			Flags: flg.Flags{
				&flg.String{Name: "val"},
			},
			Commands: xli.Commands{
				&xli.Command{
					Name: "a",
					Flags: flg.Flags{
						&flg.String{Name: "val"},
					},
					Handler: xli.Handle(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
						ok = flg.LookupP[string, *xli.Command](cmd, "val", nil)
						return nil
					}),
				},
			},
		}

		err := c.Run(t.Context(), []string{"a", "--val=foo"})
		x.NoError(err)
		x.False(ok)
	}))
}
