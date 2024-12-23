package xli_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/stretchr/testify/require"
)

func TestAction(t *testing.T) {
	appender := func(vs *[]string, v string) xli.Action {
		return func(ctx context.Context, cmd *xli.Command) (context.Context, error) {
			*vs = append(*vs, v)
			return ctx, nil
		}
	}
	new_c := func(vs *[]string) *xli.Command {
		return &xli.Command{
			Commands: xli.Commands{
				&xli.Command{
					Name: "foo",
					PreAction: xli.Chain(
						xli.OnPass(appender(vs, "foo-pre-pass")),
						xli.OnHelp(appender(vs, "foo-pre-help")),
						xli.OnRun(appender(vs, "foo-pre-run")),
					),
					Action: xli.Chain(
						xli.OnPass(appender(vs, "foo-body-pass")),
						xli.OnHelp(appender(vs, "foo-body-help")),
						xli.OnRun(appender(vs, "foo-body-run")),
					),
				},
			},
			PreAction: xli.Chain(
				xli.OnPass(appender(vs, "root-pre-pass")),
				xli.OnHelp(appender(vs, "root-pre-help")),
				xli.OnRun(appender(vs, "root-pre-run")),
			),
			Action: xli.Chain(
				xli.OnPass(appender(vs, "root-body-pass")),
				xli.OnHelp(appender(vs, "root-body-help")),
				xli.OnRun(appender(vs, "root-body-run")),
			),
		}
	}

	t.Run("run root command", func(t *testing.T) {
		vs := []string{}
		c := new_c(&vs)

		_, err := c.Run(context.TODO(), nil)
		require.NoError(t, err)
		require.Equal(t, []string{
			"root-pre-run", "root-body-run",
		}, vs)
	})
	t.Run("help root command", func(t *testing.T) {
		vs := []string{}
		c := new_c(&vs)

		_, err := c.Run(context.TODO(), []string{"--help"})
		require.NoError(t, err)
		require.Equal(t, []string{
			// Help does not hit the body!
			"root-pre-help",
		}, vs)
	})
	t.Run("run subcommand", func(t *testing.T) {
		vs := []string{}
		c := new_c(&vs)

		_, err := c.Run(context.TODO(), []string{"foo"})
		require.NoError(t, err)
		require.Equal(t, []string{
			"root-pre-pass", "root-body-pass",
			"foo-pre-run", "foo-body-run",
		}, vs)
	})
	t.Run("help subcommand", func(t *testing.T) {
		vs := []string{}
		c := new_c(&vs)

		_, err := c.Run(context.TODO(), []string{"foo", "--help"})
		require.NoError(t, err)
		require.Equal(t, []string{
			"foo-pre-help",
		}, vs)
	})
}
