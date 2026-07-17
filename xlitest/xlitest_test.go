package xlitest_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/tab"
	"github.com/lesomnus/xli/xlitest"
)

func TestRun(t *testing.T) {
	t.Run("captures stdout", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "app",
			Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
				cmd.Print("hello")
				return next(ctx)
			}),
		}

		got := xlitest.Run(t, c)
		x.NoError(got.Err)
		x.Equal("hello", got.Stdout)
		x.Equal("", got.Stderr)
	}))
	t.Run("captures stderr", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "app",
			Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
				fmt.Fprintln(cmd.ErrWriter, "oops")
				return next(ctx)
			}),
		}

		got := xlitest.Run(t, c)
		x.NoError(got.Err)
		x.Equal("", got.Stdout)
		x.Equal("oops\n", got.Stderr)
	}))
	t.Run("reads args into flags and arguments", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "app",
			Flags: flg.Flags{
				&flg.String{Name: "greeting", Alias: 'g'},
			},
			Args: arg.Args{
				&arg.String{Name: "NAME"},
			},
			Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
				g := flg.MustGet[string](cmd, "greeting")
				name := arg.MustGet[string](cmd, "NAME")
				cmd.Printf("%s, %s!", g, name)
				return next(ctx)
			}),
		}

		got := xlitest.Run(t, c, "--greeting=Hi", "World")
		x.NoError(got.Err)
		x.Equal("Hi, World!", got.Stdout)
	}))
	t.Run("returns the run error", x.F(func(x x.X) {
		c := &xli.Command{
			Name:  "app",
			Flags: flg.Flags{&flg.String{Name: "token", Required: true}},
		}

		got := xlitest.Run(t, c)
		x.ErrorContains(got.Err, "required flag")
	}))
	t.Run("feeds stdin", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "app",
			Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
				var name string
				cmd.Scanln(&name)
				cmd.Printf("hi %s", name)
				return next(ctx)
			}),
		}

		got := xlitest.Harness{Cmd: c, Stdin: "Alice\n"}.Run(t)
		x.NoError(got.Err)
		x.Equal("hi Alice", got.Stdout)
	}))
	t.Run("uses the given context", x.F(func(x x.X) {
		type ctxKey struct{}
		ctx := context.WithValue(context.Background(), ctxKey{}, "the-value")

		seen := ""
		c := &xli.Command{
			Name: "app",
			Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
				seen, _ = ctx.Value(ctxKey{}).(string)
				return next(ctx)
			}),
		}

		got := xlitest.Harness{Cmd: c, Ctx: ctx}.Run(t)
		x.NoError(got.Err)
		x.Equal("the-value", seen)
	}))
	t.Run("dispatches to a subcommand", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "app",
			Commands: xli.Commands{
				&xli.Command{
					Name: "greet",
					Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
						cmd.Print("greeted")
						return next(ctx)
					}),
				},
			},
		}

		got := xlitest.Run(t, c, "greet")
		x.NoError(got.Err)
		x.Equal("greeted", got.Stdout)
	}))
}

func newCompletionCmd() *xli.Command {
	return &xli.Command{
		Name: "app",
		Flags: flg.Flags{
			&flg.String{Name: "bar", Alias: 'b', Brief: "bar-brief", Handler: flg.OnTab[string](func(ctx context.Context, t tab.Tab) error {
				t.ValueD("BVAL", "bar-value")
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

func TestComplete(t *testing.T) {
	t.Run("subcommands at root", x.F(func(x x.X) {
		got := xlitest.Complete(t, newCompletionCmd(), "")
		x.NoError(got.Err)
		x.True(got.Has("echo"))
		x.True(got.Has("ping"))
	}))
	t.Run("subcommands filtered by a partial word still offer full set", x.F(func(x x.X) {
		// Filtering by the typed prefix is the shell's job; xli emits the full
		// candidate set and the shell narrows it.
		got := xlitest.Complete(t, newCompletionCmd(), "e")
		x.True(got.Has("echo"))
	}))
	t.Run("flag names", x.F(func(x x.X) {
		got := xlitest.Complete(t, newCompletionCmd(), "--")
		x.True(got.Has("--bar"))
	}))
	t.Run("candidate carries its description", x.F(func(x x.X) {
		got := xlitest.Complete(t, newCompletionCmd(), "--")
		cand, ok := got.Get("--bar")
		x.True(ok)
		x.Equal("bar-brief", cand.Desc)
	}))
	t.Run("long flag value", x.F(func(x x.X) {
		got := xlitest.Complete(t, newCompletionCmd(), "--bar=")
		cand, ok := got.Get("BVAL")
		x.True(ok)
		x.Equal("bar-value", cand.Desc)
	}))
	t.Run("short flag value", x.F(func(x x.X) {
		got := xlitest.Complete(t, newCompletionCmd(), "-b=")
		x.True(got.Has("BVAL"))
	}))
	t.Run("argument value after a subcommand", x.F(func(x x.X) {
		got := xlitest.Complete(t, newCompletionCmd(), "echo ")
		x.True(got.Has("AVAL"))
	}))
	t.Run("no trailing space keeps completing the same word", x.F(func(x x.X) {
		// "echo" without a trailing space is the word being completed, so the
		// subcommand set (not echo's argument values) is offered.
		got := xlitest.Complete(t, newCompletionCmd(), "echo")
		x.True(got.Has("echo"))
		x.False(got.Has("AVAL"))
	}))
	t.Run("a malformed line surfaces as Err", x.F(func(x x.X) {
		// Completion is expected to emit nothing rather than error, so a run
		// that does error (here, an unknown flag) is reported via Err.
		got := xlitest.Complete(t, newCompletionCmd(), "--nope ")
		x.ErrorContains(got.Err, "unknown flag")
		x.Len(got.Candidates, 0)
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

		got := xlitest.Complete(t, c, "remote ")
		x.NoError(got.Err)
		x.Equal([]string{"add", "remove"}, got.Values())
	}))
	t.Run("groups are decoded", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "app",
			Commands: xli.Commands{
				&xli.Command{Name: "echo"},
			}.WithCategory("fruits",
				&xli.Command{Name: "apple"},
			),
		}

		got := xlitest.Complete(t, c, "")
		apple, ok := got.Get("apple")
		x.True(ok)
		x.Equal("fruits", apple.Group)

		echo, ok := got.Get("echo")
		x.True(ok)
		x.Equal("", echo.Group)
	}))
	t.Run("file completion request", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "app",
			Args: arg.Args{
				&arg.String{Name: "PATH", Handler: arg.OnTab[string](func(ctx context.Context, t tab.Tab) {
					t.Files("*.go")
				})},
			},
		}

		got := xlitest.Complete(t, c, "")
		x.True(got.WantFiles)
		x.Equal([]string{"*.go"}, got.FilePatterns)
		x.False(got.WantDirs)
	}))
	t.Run("directory completion request", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "app",
			Args: arg.Args{
				&arg.String{Name: "DIR", Handler: arg.OnTab[string](func(ctx context.Context, t tab.Tab) {
					t.Dirs()
				})},
			},
		}

		got := xlitest.Complete(t, c, "")
		x.True(got.WantDirs)
		x.False(got.WantFiles)
	}))
}
