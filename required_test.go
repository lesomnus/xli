package xli_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
)

func TestRequiredFlag(t *testing.T) {
	newCmd := func() *xli.Command {
		return &xli.Command{
			Flags: flg.Flags{
				&flg.String{Name: "token", Required: true},
			},
			Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
				return next(ctx)
			}),
		}
	}

	t.Run("missing required flag is an error", x.F(func(x x.X) {
		err := newCmd().Run(t.Context(), nil)
		x.True(errors.Is(err, xli.ErrFlagRequired))
		x.ErrorContains(err, "token")
	}))
	t.Run("provided required flag runs", x.F(func(x x.X) {
		err := newCmd().Run(t.Context(), []string{"--token=abc"})
		x.NoError(err)
	}))
	t.Run("help works without the required flag", x.F(func(x x.X) {
		c := newCmd()
		b := &strings.Builder{}
		c.Writer = b
		err := c.Run(t.Context(), []string{"--help"})
		x.NoError(err)
	}))
	t.Run("required flag on a subcommand is enforced", x.F(func(x x.X) {
		c := &xli.Command{
			Commands: xli.Commands{
				&xli.Command{
					Name: "push",
					Flags: flg.Flags{
						&flg.String{Name: "remote", Required: true},
					},
				},
			},
		}

		err := c.Run(t.Context(), []string{"push"})
		x.True(errors.Is(err, xli.ErrFlagRequired))
		x.ErrorContains(err, "remote")
	}))
}
