package xli_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/internal/x"
)

func TestCommandRoot(t *testing.T) {
	t.Run("root of standalone command is itself", x.F(func(x x.X) {
		c := &xli.Command{Name: "solo"}
		x.Same(c, c.Root())
	}))
	t.Run("root of nested command is the top ancestor", x.F(func(x x.X) {
		var got *xli.Command
		root := &xli.Command{
			Name: "root",
			Commands: xli.Commands{
				&xli.Command{
					Name: "child",
					Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
						got = cmd.Root()
						return next(ctx)
					}),
				},
			},
		}

		err := root.Run(t.Context(), []string{"child"})
		x.NoError(err)
		x.Same(root, got)
		x.Equal("root", got.Name)
	}))
}
