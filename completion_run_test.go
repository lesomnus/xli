package xli_test

import (
	"context"
	"strings"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/comp"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/tab"
)

// complete drives the shell-completion path end to end the way the generated
// zsh script does: the last three argv entries are the completion tag, the
// word under the cursor (curr), and the buffer left of the cursor (buff).
func complete(t *testing.T, c *xli.Command, curr, buff string, args ...string) string {
	t.Helper()
	b := &strings.Builder{}
	c.Writer = b
	full := append(append([]string{}, args...), comp.Tag("zsh"), curr, buff)
	if err := c.Run(context.Background(), full); err != nil {
		t.Fatalf("completion run failed: %v", err)
	}
	return b.String()
}

func newCompletionTestCmd() *xli.Command {
	return &xli.Command{
		Name: "app",
		Flags: flg.Flags{
			&flg.String{Name: "bar", Alias: 'b', Brief: "bar-brief", Handler: flg.OnTab[string](func(ctx context.Context, t tab.Tab) error {
				t.Value("BVAL")
				return nil
			})},
		},
		Commands: xli.Commands{
			&xli.Command{
				Name:  "echo",
				Brief: "echo-brief",
				Args: arg.Args{
					&arg.RestStrings{Name: "STRING", Handler: arg.OnTab[[]string](func(ctx context.Context, t tab.Tab) {
						t.Value("AVAL")
					})},
				},
			},
			&xli.Command{Name: "ping", Brief: "ping-brief"},
		},
	}
}

func TestCompletionRun(t *testing.T) {
	t.Run("subcommands at root", x.F(func(x x.X) {
		out := complete(t, newCompletionTestCmd(), "", "")
		x.Contains(out, "echo")
		x.Contains(out, "ping")
	}))
	t.Run("flag names", x.F(func(x x.X) {
		out := complete(t, newCompletionTestCmd(), "--", "--", "--")
		x.Contains(out, "--bar")
	}))
	t.Run("long flag value", x.F(func(x x.X) {
		out := complete(t, newCompletionTestCmd(), "--bar=", "--bar=", "--bar=")
		x.Contains(out, "BVAL")
	}))
	t.Run("short flag value", x.F(func(x x.X) {
		out := complete(t, newCompletionTestCmd(), "-b=", "-b=", "-b=")
		x.Contains(out, "BVAL")
	}))
	t.Run("argument value", x.F(func(x x.X) {
		out := complete(t, newCompletionTestCmd(), "", "", "echo")
		x.Contains(out, "AVAL")
	}))
	t.Run("subcommands are grouped by category", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "app",
			Commands: xli.Commands{
				&xli.Command{Name: "echo"},
			}.WithCategory("fruits",
				&xli.Command{Name: "apple"},
			),
		}

		out := complete(t, c, "", "")
		x.Contains(out, "fruits\x1fapple") // grouped under "fruits"
		x.Contains(out, "\x1fecho")        // ungrouped
	}))
	t.Run("flag value is not shadowed by a missing required arg", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "app",
			Flags: flg.Flags{
				&flg.String{Name: "bar", Alias: 'b', Handler: flg.OnTab[string](func(ctx context.Context, t tab.Tab) error {
					t.Value("BVAL")
					return nil
				})},
			},
			Args: arg.Args{
				&arg.String{Name: "REQ"},
			},
		}

		out := complete(t, c, "--bar=", "--bar=", "--bar=")
		x.Contains(out, "BVAL")
	}))
	t.Run("flag names are not shadowed by an optional arg", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "app",
			Flags: flg.Flags{
				&flg.String{Name: "bar", Handler: flg.OnTab[string](func(ctx context.Context, t tab.Tab) error {
					t.Value("BVAL")
					return nil
				})},
			},
			Args: arg.Args{
				&arg.String{Name: "OPT", Optional: true, Handler: arg.OnTab[string](func(ctx context.Context, t tab.Tab) {
					t.Value("AVAL")
				})},
			},
		}

		// Typing "--" must offer flag names, not the optional argument's values.
		out := complete(t, c, "--", "--", "--")
		x.Contains(out, "--bar")
		x.False(strings.Contains(out, "AVAL"))
	}))
	t.Run("filled optional arg is not re-offered at the next position", x.F(func(x x.X) {
		newCmd := func() *xli.Command {
			return &xli.Command{
				Name: "app",
				Args: arg.Args{
					&arg.String{Name: "OPT", Optional: true, Handler: arg.OnTab[string](func(ctx context.Context, t tab.Tab) {
						t.Value("AVAL")
					})},
				},
			}
		}

		// At the empty first position the optional arg's values are offered.
		out := complete(t, newCmd(), "", "")
		x.Contains(out, "AVAL")

		// After the single arg is filled ("app foo "), the cursor sits one
		// position past it; a further tab must not re-offer the same arg.
		out = complete(t, newCmd(), "", "", "foo")
		x.False(strings.Contains(out, "AVAL"))
	}))
	t.Run("filled required arg is not re-offered at the next position", x.F(func(x x.X) {
		newCmd := func() *xli.Command {
			return &xli.Command{
				Name: "app",
				Args: arg.Args{
					&arg.String{Name: "REQ", Handler: arg.OnTab[string](func(ctx context.Context, t tab.Tab) {
						t.Value("AVAL")
					})},
				},
			}
		}

		out := complete(t, newCmd(), "", "")
		x.Contains(out, "AVAL")

		out = complete(t, newCmd(), "", "", "foo")
		x.False(strings.Contains(out, "AVAL"))
	}))
	t.Run("variadic arg keeps offering values past the first", x.F(func(x x.X) {
		// A "rest" argument accepts any number of values, so its hint must
		// still appear once one value has been filled in.
		out := complete(t, newCompletionTestCmd(), "", "", "echo", "foo")
		x.Contains(out, "AVAL")
	}))
	t.Run("position advances to the next arg", x.F(func(x x.X) {
		newCmd := func() *xli.Command {
			return &xli.Command{
				Name: "app",
				Args: arg.Args{
					&arg.String{Name: "FIRST", Handler: arg.OnTab[string](func(ctx context.Context, t tab.Tab) {
						t.Value("FIRST_VAL")
					})},
					&arg.String{Name: "SECOND", Handler: arg.OnTab[string](func(ctx context.Context, t tab.Tab) {
						t.Value("SECOND_VAL")
					})},
				},
			}
		}

		// Nothing typed: the first argument is being completed.
		out := complete(t, newCmd(), "", "")
		x.Contains(out, "FIRST_VAL")
		x.False(strings.Contains(out, "SECOND_VAL"))

		// First argument filled: completion advances to the second argument.
		out = complete(t, newCmd(), "", "", "foo")
		x.Contains(out, "SECOND_VAL")
		x.False(strings.Contains(out, "FIRST_VAL"))

		// Both arguments filled: neither is re-offered.
		out = complete(t, newCmd(), "", "", "foo", "bar")
		x.False(strings.Contains(out, "FIRST_VAL"))
		x.False(strings.Contains(out, "SECOND_VAL"))
	}))
	t.Run("nested subcommands", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "app",
			Commands: xli.Commands{
				&xli.Command{
					Name: "remote",
					Commands: xli.Commands{
						&xli.Command{Name: "add"},
						&xli.Command{Name: "remove"},
					},
				},
			},
		}

		out := complete(t, c, "", "", "remote")
		x.Contains(out, "add")
		x.Contains(out, "remove")
	}))
}

func TestCompletionScript(t *testing.T) {
	t.Run("zsh script is keyed on the root command name", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "app",
			Commands: xli.Commands{
				xli.NewCmdCompletion(),
			},
		}

		b := &strings.Builder{}
		c.Writer = b
		err := c.Run(context.Background(), []string{"completion", "zsh"})
		x.NoError(err)
		x.Contains(b.String(), "#compdef app")
	}))
}
