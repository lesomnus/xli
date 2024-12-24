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
	append_cmd := func(vs *[]string, v string, err error) xli.Action {
		return func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			(*vs) = append((*vs), v)
			if err := next(ctx); err != nil {
				return err
			}
			return err
		}
	}
	append_flg := func(vs *[]string, v string, err error) flg.Action[string] {
		return func(ctx context.Context, _ string) error {
			(*vs) = append((*vs), v)
			return err
		}
	}
	append_arg := func(vs *[]string, v string, err error) arg.Action[string] {
		return func(ctx context.Context, _ string) error {
			(*vs) = append((*vs), v)
			return err
		}
	}

	t.Run("empty", func(t *testing.T) {
		c := &xli.Command{}

		err := c.Run(context.TODO(), nil)
		require.NoError(t, err)
	})
	t.Run("flags", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			Flags: flg.Flags{
				&flg.String{Name: "foo", Action: append_flg(&vs, "foo", nil)},
				&flg.String{Name: "bar", Action: append_flg(&vs, "bar", nil)},
			},
		}

		err := c.Run(context.TODO(), []string{"--foo=a", "--bar=b"})
		require.NoError(t, err)
		require.Equal(t, []string{"foo", "bar"}, vs)
	})
	t.Run("args", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO", Action: append_arg(&vs, "foo", nil)},
				&arg.String{Name: "BAR", Action: append_arg(&vs, "bar", nil)},
			},
		}

		err := c.Run(context.TODO(), []string{"a", "b"})
		require.NoError(t, err)
		require.Equal(t, []string{"foo", "bar"}, vs)
	})
	t.Run("subcommands", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			Commands: xli.Commands{
				&xli.Command{
					Name: "foo",
					Commands: xli.Commands{
						&xli.Command{
							Name:   "bar",
							Action: append_cmd(&vs, "bar", errors.New("bar-err")),
						},
						&xli.Command{
							Name:   "baz",
							Action: append_cmd(&vs, "baz", errors.New("baz-err")),
						},
					},
				},
			},
		}

		err := c.Run(context.TODO(), []string{"foo", "bar"})
		require.ErrorContains(t, err, "bar-err")
		require.Equal(t, []string{"bar"}, vs)
	})
	t.Run("composite", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			Commands: xli.Commands{
				&xli.Command{
					Name: "foo",
					Flags: flg.Flags{
						&flg.String{Name: "foo", Action: append_flg(&vs, "f-foo", nil)},
						&flg.String{Name: "bar", Action: append_flg(&vs, "f-bar", nil)},
					},
					Args: arg.Args{
						&arg.String{Name: "FOO", Action: append_arg(&vs, "a-foo", nil)},
						&arg.String{Name: "BAR", Action: append_arg(&vs, "a-bar", nil)},
					},
					Commands: xli.Commands{
						&xli.Command{
							Name:   "bar",
							Action: append_cmd(&vs, "bar", errors.New("bar-err")),
						},
					},
					Action: append_cmd(&vs, "foo", nil),
				},
			},
		}

		err := c.Run(context.TODO(), []string{"foo", "--foo=a", "--bar=b", "c", "d", "bar"})
		require.ErrorContains(t, err, "bar-err")
		require.Equal(t, []string{"f-foo", "f-bar", "a-foo", "a-bar", "foo", "bar"}, vs)
	})
	t.Run("help before subcommand", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			Commands: xli.Commands{
				&xli.Command{
					Name: "foo",
					Commands: xli.Commands{
						&xli.Command{
							Name:   "bar",
							Action: append_cmd(&vs, "bar", nil),
						},
					},
					Action: append_cmd(&vs, "foo", nil),
				},
			},
		}

		err := c.Run(context.TODO(), []string{"foo", "--help", "bar"})
		require.NoError(t, err)
		require.Equal(t, []string{"foo"}, vs)
	})
}
