package xli_test

import (
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/xlitest"
)

func TestChoice(t *testing.T) {
	newCmd := func() *xli.Command {
		return &xli.Command{
			Name: "app",
			Flags: flg.Flags{
				&flg.Choice{Name: "format", Alias: 'f', Parser: flg.ChoiceParser{"json", "yaml"}},
				&flg.Multi[string, flg.ChoiceParser]{Name: "level", Parser: flg.ChoiceParser{"info", "warn"}},
			},
			Args: arg.Args{
				&arg.Choice{Name: "SHELL", Parser: arg.ChoiceParser{"bash", "zsh"}},
				&arg.Rest[string, arg.ChoiceParser]{
					Name:   "COLORS",
					Parser: arg.RestParser[string, arg.ChoiceParser]{Base: arg.ChoiceParser{"red", "blue"}},
				},
			},
		}
	}

	t.Run("accepts listed values", x.F(func(x x.X) {
		c := newCmd()
		got := xlitest.Run(x.T, c, "--format=yaml", "--level", "info", "--level", "warn", "zsh", "red", "blue")
		x.NoError(got.Err)
		x.Equal("yaml", flg.MustGet[string](c, "format"))
		x.Equal([]string{"info", "warn"}, flg.MustGet[[]string](c, "level"))
		x.Equal("zsh", arg.MustGet[string](c, "SHELL"))
		x.Equal([]string{"red", "blue"}, arg.MustGet[[]string](c, "COLORS"))
	}))
	t.Run("rejects other flag values with a suggestion", x.F(func(x x.X) {
		got := xlitest.Run(x.T, newCmd(), "--format=jsno", "zsh")
		x.ErrorContains(got.Err, `invalid value "jsno": must be one of "json", "yaml" (did you mean "json"?)`)
	}))
	t.Run("rejects other argument values", x.F(func(x x.X) {
		got := xlitest.Run(x.T, newCmd(), "sh")
		x.ErrorContains(got.Err, `"sh": must be one of "bash", "zsh"`)
	}))
	t.Run("rejects other rest values", x.F(func(x x.X) {
		got := xlitest.Run(x.T, newCmd(), "zsh", "red", "green")
		x.ErrorContains(got.Err, `must be one of "red", "blue"`)
	}))
	t.Run("help shows the choices as the type", x.F(func(x x.X) {
		got := xlitest.Run(x.T, newCmd(), "-h")
		x.NoError(got.Err)
		x.Contains(got.Stdout, "--format json|yaml")
		x.Contains(got.Stdout, "--level info|warn...")
	}))
	t.Run("flag value completion offers the choices", x.F(func(x x.X) {
		got := xlitest.Complete(x.T, newCmd(), "--format=")
		x.NoError(got.Err)
		x.Equal([]string{"json", "yaml"}, got.Values())

		got = xlitest.Complete(x.T, newCmd(), "-f=")
		x.Equal([]string{"json", "yaml"}, got.Values())

		got = xlitest.Complete(x.T, newCmd(), "--level=")
		x.Equal([]string{"info", "warn"}, got.Values())
	}))
	t.Run("argument completion offers the choices", x.F(func(x x.X) {
		got := xlitest.Complete(x.T, newCmd(), "")
		x.NoError(got.Err)
		x.Equal([]string{"bash", "zsh"}, got.Values())

		got = xlitest.Complete(x.T, newCmd(), "zsh ")
		x.Equal([]string{"red", "blue"}, got.Values())
	}))
}
