package xli_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/frm"
	"github.com/lesomnus/xli/internal/x"
)

func TestRequireSubcommand(t *testing.T) {
	make_cmd := func(x x.X) (*xli.Command, *[]string) {
		trace := []string{}
		handler := xli.Handle(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			f := frm.From(ctx)
			x.NotNil(f.Cmd())

			name := f.Cmd().GetName()
			trace = append(trace, name)

			return next(ctx)
		})

		c := &xli.Command{
			Name: "foo",
			Handler: xli.Chain(
				handler,
				xli.RequireSubcommand(),
			),
			Commands: xli.Commands{
				&xli.Command{
					Name:    "bar",
					Handler: handler,
				},
			},
		}

		return c, &trace
	}

	t.Run("ok", x.F(func(x x.X) {
		c, trace := make_cmd(x)

		err := c.Run(t.Context(), []string{"bar"})
		x.NoError(err)
		x.Equal(*trace, []string{"foo", "bar"})
	}))
	t.Run("no subcommand", x.F(func(x x.X) {
		c, trace := make_cmd(x)

		err := c.Run(t.Context(), nil)
		x.ErrorContains(err, "required")
		x.Equal(*trace, []string{"foo"})
	}))
}
