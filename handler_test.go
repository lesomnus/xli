package xli_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	append_cmd := func(vs *[]string, v string) xli.HandlerFunc {
		return func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			*vs = append(*vs, v)
			return next(ctx)
		}
	}
	new_c := func(vs *[]string) *xli.Command {
		return xli.New(&xli.Command{
			Handler: xli.Chain(
				xli.OnRunPass(append_cmd(vs, "root-pass")),
				xli.OnHelp(append_cmd(vs, "root-help")),
				xli.OnRun(append_cmd(vs, "root-run")),
			),
		}, xli.WithSubcommands(func() xli.Commands {
			return xli.Commands{
				&xli.Command{
					Name: "foo",
					Handler: xli.Chain(
						xli.OnRunPass(append_cmd(vs, "foo-pass")),
						xli.OnHelp(append_cmd(vs, "foo-help")),
						xli.OnRun(append_cmd(vs, "foo-run")),
					),
				},
			}
		}))
	}

	t.Run("run root command", func(t *testing.T) {
		vs := []string{}
		c := new_c(&vs)

		err := c.Run(t.Context(), nil)
		require.NoError(t, err)
		require.Equal(t, []string{"root-run"}, vs)
	})
	t.Run("help root command", func(t *testing.T) {
		vs := []string{}
		c := new_c(&vs)

		err := c.Run(t.Context(), []string{"--help"})
		require.NoError(t, err)
		require.Equal(t, []string{"root-help"}, vs)
	})
	t.Run("run subcommand", func(t *testing.T) {
		vs := []string{}
		c := new_c(&vs)

		err := c.Run(t.Context(), []string{"foo"})
		require.NoError(t, err)
		require.Equal(t, []string{"root-pass", "foo-run"}, vs)
	})
	t.Run("help subcommand", func(t *testing.T) {
		vs := []string{}
		c := new_c(&vs)

		err := c.Run(t.Context(), []string{"foo", "--help"})
		require.NoError(t, err)
		require.Equal(t, []string{"foo-help"}, vs)
	})
}
