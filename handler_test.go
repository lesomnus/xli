package xli_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
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

func TestOnTab(t *testing.T) {
	t.Run("OnTab is not invoked in run mode", x.F(func(x x.X) {
		fired := false
		c := &xli.Command{
			Handler: xli.OnTab(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
				fired = true
				return next(ctx)
			}),
		}

		err := c.Run(t.Context(), nil)
		x.NoError(err)
		x.False(fired)
	}))
}

// TestFlagAndArgHandlersSeeTheMode is a flag's or an argument's handler called
// in the mode its command's handler is: run on the way to a subcommand
// (mode.Run|mode.Pass), or as the command run (mode.Run). They are called
// while the line is parsed, which happened before the mode was decided, and so
// a handler gated on a mode -- flg.OnRun -- was never called by Run at all.
func TestFlagAndArgHandlersSeeTheMode(t *testing.T) {
	x := x.New(t)

	seen := []string{}
	rec := func(name string) func(ctx context.Context, v string) error {
		return func(ctx context.Context, v string) error {
			seen = append(seen, name)
			return nil
		}
	}
	c := &xli.Command{
		Name: "app",
		Flags: flg.Flags{
			&flg.String{Name: "run", Handler: flg.OnRun(rec("app --run"))},
			&flg.String{Name: "pass", Handler: flg.OnRunPass(rec("app --pass"))},
		},
		Commands: xli.Commands{{
			Name: "sub",
			Flags: flg.Flags{
				&flg.String{Name: "run", Handler: flg.OnRun(rec("sub --run"))},
				&flg.String{Name: "pass", Handler: flg.OnRunPass(rec("sub --pass"))},
			},
			Args: arg.Args{&arg.String{Name: "A", Handler: arg.OnRun(rec("sub A"))}},
		}},
	}

	err := c.Run(t.Context(), []string{"--run", "1", "--pass", "2", "sub", "--run", "3", "--pass", "4", "a"})
	x.NoError(err)
	x.Equal([]string{"app --pass", "sub --run", "sub A"}, seen)
}
