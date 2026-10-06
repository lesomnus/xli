package xli_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/tab"
)

// When XLI_SHELL_TEST is set the test binary acts as the "app" CLI below, so
// the generated completion scripts can drive it from a real shell.
func TestMain(m *testing.M) {
	if os.Getenv("XLI_SHELL_TEST") == "1" {
		if err := newShellTestCmd().Run(context.Background(), os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func newShellTestCmd() *xli.Command {
	return &xli.Command{
		Name: "app",
		Flags: flg.Flags{
			&flg.Choice{Name: "format", Parser: flg.ChoiceParser{"json", "yaml"}},
			&flg.Switch{Name: "verbose"},
		},
		Commands: xli.Commands{
			&xli.Command{
				Name: "deploy",
				Args: arg.Args{&arg.String{Name: "FILE", Handler: arg.OnTab[string](func(ctx context.Context, t tab.Tab) {
					t.Files("*.go")
				})}},
			},
			&xli.Command{
				Name: "cd",
				Args: arg.Args{&arg.String{Name: "DIR", Handler: arg.OnTab[string](func(ctx context.Context, t tab.Tab) {
					t.Dirs()
				})}},
			},
			&xli.Command{
				Name: "echo",
				Args: arg.Args{&arg.RestStrings{Name: "WORD", Handler: arg.OnTab[[]string](func(ctx context.Context, t tab.Tab) {
					t.ValueD("royale", "a burger")
					t.Value("with")
				})}},
			},
			&xli.Command{
				Name: "say",
				Args: arg.Args{&arg.String{Name: "WORD", Handler: arg.OnTab[string](func(ctx context.Context, t tab.Tab) {
					t.Value("hello world")
					t.Value("it's")
					t.ValueD("linux:amd64", "a platform")
					t.Value("plain?")
				})}},
			},
			xli.NewCmdCompletion(),
		},
	}
}

type shellEnv struct {
	dir    string // working directory with a.go, b.txt, sub/c.go
	path   string // PATH with the "app" wrapper first
	script string // path to the generated completion script
}

func newShellEnv(t *testing.T, shell string) shellEnv {
	t.Helper()
	if _, err := exec.LookPath(shell); err != nil {
		t.Skipf("%s is not installed", shell)
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	dir := filepath.Join(root, "work")
	for _, d := range []string{bin, filepath.Join(dir, "sub")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(bin, "app"):            fmt.Sprintf("#!/bin/sh\nXLI_SHELL_TEST=1 exec %q \"$@\"\n", self),
		filepath.Join(dir, "a.go"):           "",
		filepath.Join(dir, "b.txt"):          "",
		filepath.Join(dir, "sub", "c.go"):    "",
		filepath.Join(root, "completion.sh"): "",
	}
	for p, content := range files {
		if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	b := &bytes.Buffer{}
	c := newShellTestCmd()
	c.Writer = b
	if err := c.Run(context.Background(), []string{"completion", shell}); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "completion.sh")
	if err := os.WriteFile(script, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	return shellEnv{
		dir:    dir,
		path:   bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		script: script,
	}
}

// run executes code in shell with LINE set to the command line being
// completed and returns the sorted, non-empty output lines.
func (e shellEnv) run(t *testing.T, shell string, code string, line string, env ...string) []string {
	t.Helper()

	var cmd *exec.Cmd
	switch shell {
	case "bash":
		cmd = exec.Command("bash", "--norc", "--noprofile", "-c", code)
	case "fish":
		cmd = exec.Command("fish", "--no-config", "-c", code)
	}
	cmd.Dir = e.dir
	cmd.Env = append(os.Environ(), "PATH="+e.path, "HOME="+e.dir, "SCRIPT="+e.script, "LINE="+line)
	cmd.Env = append(cmd.Env, env...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", shell, err, out)
	}

	vs := []string{}
	for _, l := range strings.Split(string(out), "\n") {
		if l != "" {
			vs = append(vs, l)
		}
	}
	slices.Sort(vs)
	return vs
}

func TestCompletionBash(t *testing.T) {
	e := newShellEnv(t, "bash")
	const code = `
source "$SCRIPT"
[[ -v WORDBREAKS ]] && COMP_WORDBREAKS=$WORDBREAKS
[[ -v SHOPT ]] && shopt -s "$SHOPT"
COMP_LINE=$LINE
COMP_POINT=${#LINE}
_app_bash
printf '%s\n' "${COMPREPLY[@]}"
`
	// bash's default COMP_WORDBREAKS, which includes "=".
	const wordbreaks = "WORDBREAKS=\"'><=;|&(:"
	complete := func(line string, env ...string) []string {
		return e.run(t, "bash", code, line, env...)
	}

	t.Run("subcommands", x.F(func(x x.X) {
		x.Equal([]string{"cd", "completion", "deploy", "echo", "say"}, complete("app ", wordbreaks))
	}))
	t.Run("subcommand prefix", x.F(func(x x.X) {
		x.Equal([]string{"deploy"}, complete("app dep", wordbreaks))
	}))
	t.Run("flag names", x.F(func(x x.X) {
		x.Equal([]string{"--format", "--verbose"}, complete("app --", wordbreaks))
	}))
	t.Run("flag values when = breaks words", x.F(func(x x.X) {
		x.Equal([]string{"json", "yaml"}, complete("app --format=", wordbreaks))
		x.Equal([]string{"yaml"}, complete("app --format=y", wordbreaks))
	}))
	t.Run("flag values when = does not break words", x.F(func(x x.X) {
		x.Equal([]string{"--format=json", "--format=yaml"}, complete("app --format=", "WORDBREAKS= "))
	}))
	t.Run("argument values", x.F(func(x x.X) {
		x.Equal([]string{"royale"}, complete("app echo r", wordbreaks))
		x.Equal([]string{"royale", "with"}, complete("app echo royale ", wordbreaks))
	}))
	t.Run("files matching a glob and directories", x.F(func(x x.X) {
		x.Equal([]string{"a.go", "sub"}, complete("app deploy ", wordbreaks))
		x.Equal([]string{"sub/c.go"}, complete("app deploy sub/", wordbreaks))
	}))
	t.Run("directories", x.F(func(x x.X) {
		x.Equal([]string{"sub"}, complete("app cd ", wordbreaks))
	}))
	t.Run("values are quoted and colons kept", x.F(func(x x.X) {
		want := []string{`hello\ world`, `it\'s`, "linux:amd64", `plain\?`}
		x.Equal(want, complete("app say ", wordbreaks))
		x.Equal([]string{"linux:amd64"}, complete("app say li", wordbreaks))
	}))
	t.Run("glob options do not affect candidates", x.F(func(x x.X) {
		want := []string{`hello\ world`, `it\'s`, "linux:amd64", `plain\?`}
		x.Equal(want, complete("app say ", wordbreaks, "SHOPT=nullglob"))
		x.Equal(want, complete("app say ", wordbreaks, "SHOPT=failglob"))
		x.Equal([]string{"a.go", "sub"}, complete("app deploy ", wordbreaks, "SHOPT=failglob"))
	}))
}

func TestCompletionFish(t *testing.T) {
	e := newShellEnv(t, "fish")
	const code = `
source $SCRIPT
complete -C"$LINE"
`
	complete := func(line string) []string {
		return e.run(t, "fish", code, line)
	}

	t.Run("subcommands", x.F(func(x x.X) {
		x.Equal([]string{"cd", "completion\tprint a shell completion script", "deploy", "echo", "say"}, complete("app "))
	}))
	t.Run("subcommand prefix", x.F(func(x x.X) {
		x.Equal([]string{"deploy"}, complete("app dep"))
	}))
	t.Run("flag names", x.F(func(x x.X) {
		x.Equal([]string{"--format", "--verbose"}, complete("app --"))
	}))
	t.Run("flag values", x.F(func(x x.X) {
		x.Equal([]string{"--format=json", "--format=yaml"}, complete("app --format="))
		x.Equal([]string{"--format=yaml"}, complete("app --format=y"))
	}))
	t.Run("argument values with descriptions", x.F(func(x x.X) {
		x.Equal([]string{"royale\ta burger"}, complete("app echo r"))
		x.Equal([]string{"royale\ta burger", "with"}, complete("app echo royale "))
	}))
	t.Run("files matching a glob and directories", x.F(func(x x.X) {
		x.Equal([]string{"a.go", "sub/"}, complete("app deploy "))
		x.Equal([]string{"sub/c.go"}, complete("app deploy sub/"))
	}))
	t.Run("directories", x.F(func(x x.X) {
		x.Equal([]string{"sub/"}, complete("app cd "))
	}))
	t.Run("values with spaces, quotes, and colons", x.F(func(x x.X) {
		x.Equal([]string{"hello world", "it's", "linux:amd64\ta platform", "plain?"}, complete("app say "))
	}))
}
