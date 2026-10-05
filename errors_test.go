package xli_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
)

func TestUsageError(t *testing.T) {
	newCmd := func() *xli.Command {
		return &xli.Command{
			Name:    "app",
			Handler: xli.RequireSubcommand(),
			Commands: xli.Commands{
				&xli.Command{
					Name: "deploy",
					Flags: flg.Flags{
						&flg.Int{Name: "port"},
						&flg.String{Name: "token", Required: true},
					},
					Args: arg.Args{&arg.String{Name: "TARGET"}},
				},
			},
		}
	}

	cases := []struct {
		desc     string
		args     []string
		sentinel error
		cmd      string
		msg      string
	}{
		{"unknown subcommand", []string{"nope"}, xli.ErrUnknownCmd, "app", "app: nope: unknown subcommand"},
		{"missing subcommand", []string{}, xli.ErrNeedCmd, "app", "app: subcommand is required"},
		{"unknown flag", []string{"deploy", "--nope=1", "web"}, xli.ErrUnknownFlag, "deploy", "app deploy: --nope: unknown flag"},
		{"missing argument", []string{"deploy", "--token=t"}, xli.ErrNeedArgs, "deploy", `app deploy: "TARGET": required argument not given`},
		{"flag after argument", []string{"deploy", "web", "--token=t"}, xli.ErrFlagAfterArg, "deploy", "app deploy: --token: flag must come before arguments"},
		{"missing required flag", []string{"deploy", "web"}, xli.ErrFlagRequired, "deploy", "app deploy: required flag not set: --token"},
		{"invalid flag value", []string{"deploy", "--port=x", "--token=t", "web"}, nil, "deploy", "app deploy: invalid flag: --port"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, x.F(func(x x.X) {
			err := newCmd().Run(context.Background(), tc.args)

			var ue *xli.UsageError
			x.True(errors.As(err, &ue))
			x.Equal(tc.cmd, ue.Cmd.Name)
			if tc.sentinel != nil {
				x.True(errors.Is(err, tc.sentinel))
			}
			x.ErrorContains(err, tc.msg)
		}))
	}

	t.Run("unknown subcommand suggests a close name", x.F(func(x x.X) {
		err := newCmd().Run(context.Background(), []string{"deplyo"})
		x.ErrorContains(err, `deplyo: unknown subcommand (did you mean "deploy"?)`)
	}))
	t.Run("unknown subcommand suggests an alias", x.F(func(x x.X) {
		c := newCmd()
		c.Commands[0].Aliases = []string{"ship"}
		err := c.Run(context.Background(), []string{"shp"})
		x.ErrorContains(err, `(did you mean "ship"?)`)
	}))
	t.Run("unknown long flag suggests a close name", x.F(func(x x.X) {
		err := newCmd().Run(context.Background(), []string{"deploy", "--prot=1", "web"})
		x.ErrorContains(err, `--prot: unknown flag (did you mean "--port"?)`)
	}))
	t.Run("nothing close means no suggestion", x.F(func(x x.X) {
		err := newCmd().Run(context.Background(), []string{"xyz"})
		x.ErrorContains(err, "xyz: unknown subcommand")
		x.NotContains(err.Error(), "did you mean")
	}))
	t.Run("Path joins the command names", x.F(func(x x.X) {
		err := newCmd().Run(context.Background(), []string{"deploy", "web"})

		var ue *xli.UsageError
		x.True(errors.As(err, &ue))
		x.Equal("app deploy", ue.Path())
	}))
	t.Run("handler errors are not wrapped", x.F(func(x x.X) {
		boom := errors.New("boom")
		c := &xli.Command{
			Name: "app",
			Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
				return boom
			}),
		}

		err := c.Run(context.Background(), nil)
		x.Equal(boom, err)
	}))
}
