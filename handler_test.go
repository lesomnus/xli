package xli_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/internal/x"
)

func TestHandler(t *testing.T) {
	append_cmd := func(vs *[]string, v string) xli.HandlerFunc {
		return func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			*vs = append(*vs, v)
			return next(ctx)
		}
	}
	new_c := func(vs *[]string) *xli.Command {
		return &xli.Command{
			Commands: xli.Commands{
				&xli.Command{
					Name: "foo",
					Handler: xli.Chain(
						xli.OnRunPass(append_cmd(vs, "foo-pass")),
						xli.OnHelp(append_cmd(vs, "foo-help")),
						xli.OnRun(append_cmd(vs, "foo-run")),
					),
				},
			},
			Handler: xli.Chain(
				xli.OnRunPass(append_cmd(vs, "root-pass")),
				xli.OnHelp(append_cmd(vs, "root-help")),
				xli.OnRun(append_cmd(vs, "root-run")),
			),
		}
	}

	t.Run("run root command", x.F(func(x x.X) {
		vs := []string{}
		c := new_c(&vs)

		err := c.Run(t.Context(), nil)
		x.NoError(err)
		x.Equal([]string{"root-run"}, vs)
	}))
	t.Run("help root command", x.F(func(x x.X) {
		vs := []string{}
		c := new_c(&vs)

		err := c.Run(t.Context(), []string{"--help"})
		x.NoError(err)
		x.Equal([]string{"root-help"}, vs)
	}))
	t.Run("run subcommand", x.F(func(x x.X) {
		vs := []string{}
		c := new_c(&vs)

		err := c.Run(t.Context(), []string{"foo"})
		x.NoError(err)
		x.Equal([]string{"root-pass", "foo-run"}, vs)
	}))
	t.Run("help subcommand", x.F(func(x x.X) {
		vs := []string{}
		c := new_c(&vs)

		err := c.Run(t.Context(), []string{"foo", "--help"})
		x.NoError(err)
		x.Equal([]string{"foo-help"}, vs)
	}))
}
