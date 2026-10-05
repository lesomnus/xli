package xli_test

import (
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/xlitest"
)

func TestHidden(t *testing.T) {
	newCmd := func() *xli.Command {
		return &xli.Command{
			Name: "app",
			Flags: flg.Flags{
				&flg.Switch{Name: "verbose"},
				&flg.Switch{Name: "debug-dump", Hidden: true},
			},
			Commands: xli.Commands{
				&xli.Command{Name: "serve", Brief: "serve things"},
				&xli.Command{Name: "debug", Brief: "internal", Hidden: true},
			},
		}
	}

	t.Run("help omits hidden commands and flags", x.F(func(x x.X) {
		got := xlitest.Run(x.T, newCmd(), "-h")
		x.NoError(got.Err)
		x.Contains(got.Stdout, "serve")
		x.Contains(got.Stdout, "--verbose")
		x.NotContains(got.Stdout, "debug")
	}))
	t.Run("usage omits [command] and [options] when all are hidden", x.F(func(x x.X) {
		c := &xli.Command{
			Name:     "app",
			Flags:    flg.Flags{&flg.Switch{Name: "x", Hidden: true}},
			Commands: xli.Commands{&xli.Command{Name: "y", Hidden: true}},
		}
		got := xlitest.Run(x.T, c, "-h")
		x.NoError(got.Err)
		x.Contains(got.Stdout, "    app\n")
		x.NotContains(got.Stdout, "Commands:")
	}))
	t.Run("hidden commands and flags still run", x.F(func(x x.X) {
		c := newCmd()
		got := xlitest.Run(x.T, c, "--debug-dump", "debug")
		x.NoError(got.Err)

		v, ok := c.Flags.Get("debug-dump").(*flg.Switch).Get()
		x.True(ok)
		x.True(v)
	}))
	t.Run("completion omits hidden commands", x.F(func(x x.X) {
		got := xlitest.Complete(x.T, newCmd(), "")
		x.NoError(got.Err)
		x.True(got.Has("serve"))
		x.False(got.Has("debug"))
	}))
	t.Run("completion omits hidden flags", x.F(func(x x.X) {
		got := xlitest.Complete(x.T, newCmd(), "--")
		x.NoError(got.Err)
		x.True(got.Has("--verbose"))
		x.False(got.Has("--debug-dump"))
	}))
	t.Run("hidden names are not suggested", x.F(func(x x.X) {
		got := xlitest.Run(x.T, newCmd(), "debu")
		x.ErrorContains(got.Err, "unknown subcommand")
		x.NotContains(got.Err.Error(), "did you mean")
	}))
}
