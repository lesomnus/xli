package xli_test

import (
	"errors"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/xlitest"
)

func TestExclusive(t *testing.T) {
	newCmd := func() *xli.Command {
		return &xli.Command{
			Name: "app",
			Flags: flg.Flags{
				&flg.Switch{Name: "json"},
				&flg.Switch{Name: "yaml"},
				&flg.Switch{Name: "toml"},
				&flg.Switch{Name: "verbose"},
			},
			Exclusive: [][]string{{"json", "yaml", "toml"}},
		}
	}

	t.Run("one flag of a group is fine", x.F(func(x x.X) {
		got := xlitest.Run(x.T, newCmd(), "--json", "--verbose")
		x.NoError(got.Err)
	}))
	t.Run("no flag of a group is fine", x.F(func(x x.X) {
		got := xlitest.Run(x.T, newCmd())
		x.NoError(got.Err)
	}))
	t.Run("two flags of a group conflict", x.F(func(x x.X) {
		got := xlitest.Run(x.T, newCmd(), "--json", "--toml")
		x.True(errors.Is(got.Err, xli.ErrFlagConflict))
		x.ErrorContains(got.Err, "app: flags cannot be used together: --json, --toml")

		var ue *xli.UsageError
		x.True(errors.As(got.Err, &ue))
	}))
	t.Run("help is exempt", x.F(func(x x.X) {
		got := xlitest.Run(x.T, newCmd(), "--json", "--yaml", "-h")
		x.NoError(got.Err)
	}))
	t.Run("checked on subcommands too", x.F(func(x x.X) {
		c := &xli.Command{
			Name:     "app",
			Commands: xli.Commands{newCmd()},
		}
		c.Commands[0].Name = "sub"
		got := xlitest.Run(x.T, c, "sub", "--json", "--yaml")
		x.ErrorContains(got.Err, "app sub: flags cannot be used together")
	}))
	t.Run("unknown flag name in a group is reported", x.F(func(x x.X) {
		c := newCmd()
		c.Exclusive = [][]string{{"json", "xml"}}
		got := xlitest.Run(x.T, c)
		x.ErrorContains(got.Err, `unknown flag "xml"`)
	}))
}
