package xli_test

import (
	"strings"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
)

func newDocTestCmd() *xli.Command {
	port := 8080
	return &xli.Command{
		Name:  "app",
		Brief: "manage things",
		Synop: "App does many things.",
		Flags: flg.Flags{&flg.Switch{Name: "verbose", Alias: 'v', Brief: "chatty output"}},
		Commands: xli.Commands{
			&xli.Command{
				Name:  "deploy",
				Brief: "deploy things",
				Flags: flg.Flags{
					&flg.Int{Name: "port", Default: &port, Brief: "listen port"},
					&flg.Choice{Name: "format", Required: true, Parser: flg.ChoiceParser{"json", "yaml"}},
					&flg.Switch{Name: "secret", Hidden: true},
				},
				Args: arg.Args{&arg.String{Name: "TARGET", Brief: "where to deploy"}},
			},
			&xli.Command{
				Name:     "remote",
				Brief:    "manage remotes",
				Commands: xli.Commands{&xli.Command{Name: "add", Brief: "add one"}},
			},
			&xli.Command{Name: "debug", Hidden: true},
		},
	}
}

func TestWriteMarkdown(t *testing.T) {
	b := &strings.Builder{}
	err := xli.WriteMarkdown(b, newDocTestCmd())
	if err != nil {
		t.Fatal(err)
	}
	out := b.String()

	t.Run("sections per command", x.F(func(x x.X) {
		x.Contains(out, "# app\n\nmanage things\n")
		x.Contains(out, "## app deploy\n")
		x.Contains(out, "## app remote\n")
		x.Contains(out, "## app remote add\n")
	}))
	t.Run("usage, description, arguments, options", x.F(func(x x.X) {
		x.Contains(out, "```\napp deploy [options] <TARGET>\n```")
		x.Contains(out, "App does many things.")
		x.Contains(out, "| `<TARGET>` | where to deploy |")
		x.Contains(out, "| `-v, --verbose` |  | chatty output |")
		x.Contains(out, "| `--port` | `int` | listen port (default: 8080) |")
		x.Contains(out, "| `--format` | `json\\|yaml` | (required) |")
	}))
	t.Run("subcommands link to their sections", x.F(func(x x.X) {
		x.Contains(out, "| [`deploy`](#app-deploy) | deploy things |")
		x.Contains(out, "| [`add`](#app-remote-add) | add one |")
	}))
	t.Run("hidden commands and flags are left out", x.F(func(x x.X) {
		x.NotContains(out, "debug")
		x.NotContains(out, "secret")
	}))
}

func TestWriteMan(t *testing.T) {
	b := &strings.Builder{}
	err := xli.WriteMan(b, newDocTestCmd(), 1)
	if err != nil {
		t.Fatal(err)
	}
	out := b.String()

	t.Run("header and top-level sections", x.F(func(x x.X) {
		x.True(strings.HasPrefix(out, ".TH APP 1\n"))
		x.Contains(out, ".SH NAME\napp \\- manage things\n")
		x.Contains(out, ".SH SYNOPSIS\napp [options] [command]\n")
		x.Contains(out, ".SH DESCRIPTION\nApp does many things.\n")
		x.Contains(out, ".SH OPTIONS\n.TP\n\\fB\\-v, \\-\\-verbose\\fR\nchatty output\n")
	}))
	t.Run("subcommands", x.F(func(x x.X) {
		x.Equal(1, strings.Count(out, ".SH COMMANDS\n"))
		x.Contains(out, ".SS app deploy\ndeploy things\n")
		x.Contains(out, ".B app deploy [options] <TARGET>\n")
		x.Contains(out, "\\fB\\-\\-format\\fR \\fIjson|yaml\\fR\n(required)\n")
		x.Contains(out, ".SS app remote add\n")
	}))
	t.Run("hidden commands and flags are left out", x.F(func(x x.X) {
		x.NotContains(out, "debug")
		x.NotContains(out, "secret")
	}))
	t.Run("quotes in a flag type do not break the line", x.F(func(x x.X) {
		c := &xli.Command{
			Name:  "app",
			Flags: flg.Flags{&flg.Choice{Name: "q", Parser: flg.ChoiceParser{`a"b`, "c"}}},
		}
		b := &strings.Builder{}
		x.NoError(xli.WriteMan(b, c, 1))
		x.Contains(b.String(), "\\fB\\-\\-q\\fR \\fIa\"b|c\\fR\n")
	}))
	t.Run("roff control characters are escaped", x.F(func(x x.X) {
		c := &xli.Command{Name: "app", Synop: ".dangerous\n'also\nback\\slash"}
		b := &strings.Builder{}
		x.NoError(xli.WriteMan(b, c, 8))
		x.Contains(b.String(), ".TH APP 8\n")
		x.Contains(b.String(), "\\&.dangerous\n\\&'also\nback\\eslash\n")
	}))
}
