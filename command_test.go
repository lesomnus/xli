package xli_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/stretchr/testify/require"
)

func TestCommandExecutionOrder(t *testing.T) {
	append_cmd := func(vs *[]string, v string, err error) xli.Handler {
		return xli.Handle(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			(*vs) = append((*vs), v)
			if err := next(ctx); err != nil {
				return err
			}
			return err
		})
	}
	append_flg := func(vs *[]string, v string, err error) flg.Handler[string] {
		return flg.Handle(func(ctx context.Context, _ string) error {
			(*vs) = append((*vs), v)
			return err
		})
	}
	append_arg := func(vs *[]string, v string, err error) arg.Handler[string] {
		return arg.Handle(func(ctx context.Context, _ string) error {
			(*vs) = append((*vs), v)
			return err
		})
	}

	t.Run("empty", func(t *testing.T) {
		c := &xli.Command{}

		err := c.Run(t.Context(), nil)
		require.NoError(t, err)
	})
	t.Run("flags", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			Flags: flg.Flags{
				&flg.String{Name: "foo", Handler: append_flg(&vs, "foo", nil)},
				&flg.String{Name: "bar", Handler: append_flg(&vs, "bar", nil)},
			},
		}

		err := c.Run(t.Context(), []string{"--foo=a", "--bar=b"})
		require.NoError(t, err)
		require.Equal(t, []string{"foo", "bar"}, vs)
	})
	t.Run("args", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO", Handler: append_arg(&vs, "foo", nil)},
				&arg.String{Name: "BAR", Handler: append_arg(&vs, "bar", nil)},
			},
		}

		err := c.Run(t.Context(), []string{"a", "b"})
		require.NoError(t, err)
		require.Equal(t, []string{"foo", "bar"}, vs)
	})
	t.Run("subcommands", func(t *testing.T) {
		vs := []string{}
		c := xli.New(&xli.Command{}, xli.WithSubcommands(func() xli.Commands {
			return xli.Commands{
				xli.New(&xli.Command{
					Name: "foo",
				}, xli.WithSubcommands(func() xli.Commands {
					return xli.Commands{
						&xli.Command{
							Name:    "bar",
							Handler: append_cmd(&vs, "bar", errors.New("bar-err")),
						},
						&xli.Command{
							Name:    "baz",
							Handler: append_cmd(&vs, "baz", errors.New("baz-err")),
						},
					}
				})),
			}
		}))

		err := c.Run(t.Context(), []string{"foo", "bar"})
		require.ErrorContains(t, err, "bar-err")
		require.Equal(t, []string{"bar"}, vs)
	})
	t.Run("composite", func(t *testing.T) {
		vs := []string{}
		c := xli.New(&xli.Command{}, xli.WithSubcommands(func() xli.Commands {
			return xli.Commands{
				xli.New(&xli.Command{
					Name: "foo",
					Flags: flg.Flags{
						&flg.String{Name: "foo", Handler: append_flg(&vs, "f-foo", nil)},
						&flg.String{Name: "bar", Handler: append_flg(&vs, "f-bar", nil)},
					},
					Args: arg.Args{
						&arg.String{Name: "FOO", Handler: append_arg(&vs, "a-foo", nil)},
						&arg.String{Name: "BAR", Handler: append_arg(&vs, "a-bar", nil)},
					},
					Handler: append_cmd(&vs, "foo", nil),
				}, xli.WithSubcommands(func() xli.Commands {
					return xli.Commands{
						&xli.Command{
							Name:    "bar",
							Handler: append_cmd(&vs, "bar", errors.New("bar-err")),
						},
					}
				})),
			}
		}))

		err := c.Run(t.Context(), []string{"foo", "--foo=a", "--bar=b", "c", "d", "bar"})
		require.ErrorContains(t, err, "bar-err")
		require.Equal(t, []string{"f-foo", "f-bar", "a-foo", "a-bar", "foo", "bar"}, vs)
	})
	t.Run("help before subcommand", func(t *testing.T) {
		vs := []string{}
		c := xli.New(&xli.Command{}, xli.WithSubcommands(func() xli.Commands {
			return xli.Commands{
				xli.New(&xli.Command{
					Name:    "foo",
					Handler: append_cmd(&vs, "foo", nil),
				}, xli.WithSubcommands(func() xli.Commands {
					return xli.Commands{
						&xli.Command{
							Name:    "bar",
							Handler: append_cmd(&vs, "bar", nil),
						},
					}
				})),
			}
		}))

		err := c.Run(t.Context(), []string{"foo", "--help", "bar"})
		require.NoError(t, err)
		require.Equal(t, []string{"foo"}, vs)
	})
}
