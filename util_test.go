package xli_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/frm"
	"github.com/stretchr/testify/require"
)

func TestRequireSubcommand(t *testing.T) {
	make_cmd := func() (*xli.Command, *[]string) {
		trace := []string{}
		handler := xli.Handle(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			f := frm.From(ctx)
			require.NotNil(t, f.Cmd())

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

	t.Run("ok", func(t *testing.T) {
		c, trace := make_cmd()

		err := c.Run(context.TODO(), []string{"bar"})
		require.NoError(t, err)
		require.Equal(t, *trace, []string{"foo", "bar"})
	})
	t.Run("no subcommand", func(t *testing.T) {
		c, trace := make_cmd()

		err := c.Run(context.TODO(), nil)
		require.ErrorContains(t, err, "required")
		require.Equal(t, *trace, []string{"foo"})
	})
}
