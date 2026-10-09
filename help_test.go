package xli_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/xlitest"
)

func TestPrintHelp(t *testing.T) {
	t.Run("usage separates arguments with spaces", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "cp",
			Args: arg.Args{
				&arg.String{Name: "SRC"},
				&arg.String{Name: "DST"},
			},
		}

		b := &strings.Builder{}
		err := c.PrintHelp(b)
		x.NoError(err)
		x.Contains(b.String(), "cp <SRC> <DST>")
	}))
	t.Run("optional argument is rendered with brackets", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "rm",
			Args: arg.Args{
				&arg.String{Name: "TARGET", Optional: true},
			},
		}

		b := &strings.Builder{}
		err := c.PrintHelp(b)
		x.NoError(err)
		x.Contains(b.String(), "rm [TARGET]")
	}))
	t.Run("flag default and required are shown in options", x.F(func(x x.X) {
		def := "8080"
		c := &xli.Command{
			Name: "srv",
			Flags: flg.Flags{
				&flg.String{Name: "port", Default: &def},
				&flg.String{Name: "token", Required: true},
			},
		}

		b := &strings.Builder{}
		err := c.PrintHelp(b)
		x.NoError(err)
		x.Contains(b.String(), "(default: ")
		x.Contains(b.String(), "(required)")
	}))
	t.Run("argument default is shown", x.F(func(x x.X) {
		def := "out.txt"
		c := &xli.Command{
			Name: "write",
			Args: arg.Args{
				&arg.String{Name: "DST", Optional: true, Brief: "destination", Default: &def},
			},
		}

		b := &strings.Builder{}
		err := c.PrintHelp(b)
		x.NoError(err)
		x.Contains(b.String(), "(default: out.txt)")
	}))
	t.Run("synopsis is rendered as a description", x.F(func(x x.X) {
		c := &xli.Command{
			Name:  "app",
			Synop: "A longer description of the app.",
		}

		b := &strings.Builder{}
		err := c.PrintHelp(b)
		x.NoError(err)
		x.Contains(b.String(), "Description:")
		x.Contains(b.String(), "A longer description of the app.")
	}))
	t.Run("options are placed before arguments in usage", x.F(func(x x.X) {
		c := &xli.Command{
			Name:  "deploy",
			Flags: flg.Flags{&flg.String{Name: "port"}},
			Args:  arg.Args{&arg.String{Name: "TARGET"}},
		}

		b := &strings.Builder{}
		err := c.PrintHelp(b)
		x.NoError(err)
		x.Contains(b.String(), "    deploy [options] <TARGET>\n")
	}))
	t.Run("subcommand usage includes ancestors", x.F(func(x x.X) {
		c := &xli.Command{
			Name:  "app",
			Flags: flg.Flags{&flg.Switch{Name: "verbose"}},
			Commands: xli.Commands{
				&xli.Command{
					Name:  "deploy",
					Flags: flg.Flags{&flg.String{Name: "port"}},
					Args:  arg.Args{&arg.String{Name: "TARGET"}},
				},
			},
		}

		got := xlitest.Run(x.T, c, "deploy", "-h")
		x.NoError(got.Err)
		x.Contains(got.Stdout, "    app deploy [options] <TARGET>\n")
	}))
	t.Run("help flag is listed in options", x.F(func(x x.X) {
		c := &xli.Command{Name: "app"}

		b := &strings.Builder{}
		err := c.PrintHelp(b)
		x.NoError(err)
		x.Contains(b.String(), "Options:")
		x.Contains(b.String(), "-h,--help")
		x.NotContains(b.String(), "[options]")
	}))
	t.Run("flag environment variable is shown", x.F(func(x x.X) {
		c := &xli.Command{
			Name:  "srv",
			Flags: flg.Flags{&flg.String{Name: "port", Brief: "listen port"}},
		}
		f := c.Flags[0].(*flg.String)

		b := &strings.Builder{}
		x.NoError(c.PrintHelp(b))
		x.NotContains(b.String(), "[$")

		c.Flags[0] = envFlag{f, "SRV_PORT"}
		b.Reset()
		x.NoError(c.PrintHelp(b))
		x.Contains(b.String(), "listen port [$SRV_PORT]")
	}))
	t.Run("variadic argument is rendered with ellipsis", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "echo",
			Args: arg.Args{
				&arg.RestStrings{Name: "STRING"},
			},
		}

		b := &strings.Builder{}
		err := c.PrintHelp(b)
		x.NoError(err)
		x.Contains(b.String(), "echo [STRING...]")
	}))
}

// newHelpAllCmd is a tree two deep, with a category, a hidden command, and a
// handler on each command that notes it ran.
func newHelpAllCmd(ran *[]string) *xli.Command {
	note := func(name string) xli.Handler {
		return xli.Handle(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			*ran = append(*ran, name)
			return next(ctx)
		})
	}
	return &xli.Command{
		Name:    "app",
		Brief:   "manage things",
		Handler: note("app"),
		Commands: xli.Commands{
			&xli.Command{
				Name:    "deploy",
				Brief:   "deploy things",
				Flags:   flg.Flags{&flg.String{Name: "port", Brief: "listen port"}},
				Args:    arg.Args{&arg.String{Name: "TARGET"}},
				Handler: note("deploy"),
			},
			&xli.Command{
				Category: "Config",
				Name:     "remote",
				Brief:    "manage remotes",
				Handler:  note("remote"),
				Commands: xli.Commands{
					&xli.Command{
						Name:    "add",
						Brief:   "add a remote",
						Args:    arg.Args{&arg.String{Name: "NAME"}},
						Handler: note("add"),
					},
				},
			},
			&xli.Command{Name: "status", Brief: "show status", Handler: note("status")},
			&xli.Command{Name: "debug", Brief: "internal", Hidden: true},
		},
	}
}

func TestHelpAll(t *testing.T) {
	t.Run("is the help of every command below, in the order help lists them", x.F(func(x x.X) {
		got := xlitest.Run(x.T, newHelpAllCmd(&[]string{}), "--help-all")
		x.NoError(got.Err)

		at := -1
		for _, name := range []string{
			"app - manage things",
			"app.deploy - deploy things",
			"app.status - show status",
			"app.remote - manage remotes",
			"app.remote.add - add a remote",
		} {
			i := strings.Index(got.Stdout, "Name:\n    "+name+"\n")
			x.True(i > at, name)
			at = i
		}
		x.Equal(4, strings.Count(got.Stdout, "\n---\n\n"), "a rule before each command below")
		x.Contains(got.Stdout, "    app deploy [options] <TARGET>\n", "usage lines are whole")
		x.Contains(got.Stdout, "    app remote add <NAME>\n")
		x.NotContains(got.Stdout, "debug")
	}))
	t.Run("runs no handler of a command below", x.F(func(x x.X) {
		ran := []string{}
		got := xlitest.Run(x.T, newHelpAllCmd(&ran), "--help-all")
		x.NoError(got.Err)
		x.Equal([]string{"app"}, ran)
	}))
	t.Run("is of the command it is given to", x.F(func(x x.X) {
		ran := []string{}
		got := xlitest.Run(x.T, newHelpAllCmd(&ran), "remote", "--help-all")
		x.NoError(got.Err)
		x.True(strings.HasPrefix(got.Stdout, "Name:\n    app.remote - manage remotes\n"))
		x.Contains(got.Stdout, "Name:\n    app.remote.add - add a remote\n")
		x.NotContains(got.Stdout, "app.deploy")
		x.Equal([]string{"app", "remote"}, ran)
	}))
	t.Run("of a command with none below is its help", x.F(func(x x.X) {
		all := xlitest.Run(x.T, newHelpAllCmd(&[]string{}), "deploy", "--help-all")
		one := xlitest.Run(x.T, newHelpAllCmd(&[]string{}), "deploy", "--help")
		x.NoError(all.Err)
		x.NoError(one.Err)
		x.Equal(one.Stdout, all.Stdout)
	}))
	t.Run("is an option where there are commands below", x.F(func(x x.X) {
		got := xlitest.Run(x.T, newHelpAllCmd(&[]string{}), "--help")
		x.NoError(got.Err)
		x.Contains(got.Stdout, "    -h,--help            show help\n    --help-all           show help of every command below too\n")

		got = xlitest.Run(x.T, newHelpAllCmd(&[]string{}), "deploy", "--help")
		x.NoError(got.Err)
		x.NotContains(got.Stdout, "--help-all")
	}))
}

func TestHelpForAnAgent(t *testing.T) {
	// asPerson clears what says an agent runs the command, and XLI_AGENT, which
	// TestMain sets.
	asPerson := func(t *testing.T) {
		for _, k := range xli.AgentEnvs {
			t.Setenv(k, "")
		}
		t.Setenv("XLI_AGENT", "")
	}
	asAgent := func(t *testing.T) {
		asPerson(t)
		t.Setenv("AI_AGENT", "claude-code_2-1-289_agent")
	}
	helpsAll := func(x x.X, args ...string) bool {
		got := xlitest.Run(x.T, newHelpAllCmd(&[]string{}), args...)
		x.NoError(got.Err)
		return strings.Contains(got.Stdout, "Name:\n    app.remote.add - add a remote\n")
	}

	t.Run("a person gets the help of the command", x.F(func(x x.X) {
		asPerson(x.T)
		x.False(helpsAll(x, "--help"))
		x.False(helpsAll(x, "-h"))
	}))
	for _, k := range xli.AgentEnvs {
		t.Run(k+" says an agent runs it, which gets every command below", x.F(func(x x.X) {
			asPerson(x.T)
			x.T.Setenv(k, "1")
			x.True(helpsAll(x, "--help"))
			x.True(helpsAll(x, "-h"))
		}))
	}
	t.Run("what the agent gets is what --help-all prints", x.F(func(x x.X) {
		asAgent(x.T)
		agent := xlitest.Run(x.T, newHelpAllCmd(&[]string{}), "remote", "--help")
		all := xlitest.Run(x.T, newHelpAllCmd(&[]string{}), "remote", "--help-all")
		x.NoError(agent.Err)
		x.Equal(all.Stdout, agent.Stdout)
	}))
	t.Run("XLI_AGENT=0 says no agent runs it", x.F(func(x x.X) {
		asAgent(x.T)
		x.T.Setenv("XLI_AGENT", "0")
		x.False(helpsAll(x, "--help"))
		x.True(helpsAll(x, "--help-all"), "asked for, it is printed")
	}))
	t.Run("XLI_AGENT=1 says one does", x.F(func(x x.X) {
		asPerson(x.T)
		x.T.Setenv("XLI_AGENT", "1")
		x.True(helpsAll(x, "--help"))
	}))
	t.Run("XLI_AGENT that is not a bool leaves it to the others", x.F(func(x x.X) {
		asPerson(x.T)
		x.T.Setenv("XLI_AGENT", "sometimes")
		x.False(helpsAll(x, "--help"))

		x.T.Setenv("AI_AGENT", "claude-code_2-1-289_agent")
		x.True(helpsAll(x, "--help"))
	}))

	// newBigCmd is newHelpAllCmd with a command below whose help alone is more
	// than an agent's --help prints at once.
	newBigCmd := func() *xli.Command {
		c := newHelpAllCmd(&[]string{})
		c.Commands = append(c.Commands, &xli.Command{
			Name:  "big",
			Brief: "say a lot",
			Synop: strings.Repeat("x", xli.AgentHelpMax),
		})
		return c
	}
	// sizeOf is how long --help-all is for the command newCmd makes, as the
	// help says it.
	sizeOf := func(x x.X, newCmd func() *xli.Command) string {
		all := xlitest.Run(x.T, newCmd(), "--help-all")
		x.NoError(all.Err)
		return fmt.Sprintf("(%d KB)", (len(all.Stdout)+500)/1000)
	}
	// past is what an agent gets beyond the help a person gets.
	past := func(x x.X, newCmd func() *xli.Command, args ...string) string {
		asAgent(x.T)
		got := xlitest.Run(x.T, newCmd(), args...)
		x.NoError(got.Err)

		asPerson(x.T)
		person := xlitest.Run(x.T, newCmd(), args...)
		x.NoError(person.Err)
		x.True(strings.HasPrefix(got.Stdout, person.Stdout), "the help a person gets")
		return strings.TrimPrefix(got.Stdout, person.Stdout)
	}
	t.Run("more than that at once is the command's help and a map of the commands below", x.F(func(x x.X) {
		x.Equal(""+
			"\nThe 5 commands below ({a,b} is a or b):\n"+
			"    app {deploy,status,big}\n"+
			"    app remote add\n"+
			"\nTheir help is more than an AI agent's --help prints at once. --help on one of\n"+
			"them prints its help with the commands below it; --help-all prints all of it\n"+
			sizeOf(x, newBigCmd)+".\n",
			past(x, newBigCmd, "--help"))
	}))
	t.Run("the map puts commands of the same shape on one line", x.F(func(x x.X) {
		verbs := func(names ...string) xli.Commands {
			cs := xli.Commands{}
			for _, name := range names {
				cs = append(cs, &xli.Command{Name: name})
			}
			return cs
		}
		newCmd := func() *xli.Command {
			return &xli.Command{Name: "app", Commands: xli.Commands{
				{Name: "user", Commands: verbs("get", "ls")},
				{Name: "solo"},
				{Name: "team", Commands: verbs("ls", "get")},
				{Name: "role", Commands: verbs("get", "ls", "watch")},
				{Name: "org", Commands: xli.Commands{{Name: "member", Commands: verbs("add", "erase")}}},
				{Name: "big", Synop: strings.Repeat("x", xli.AgentHelpMax)},
				{Name: "secret", Hidden: true, Commands: verbs("get")},
			}}
		}
		x.True(strings.HasPrefix(past(x, newCmd, "--help"), ""+
			"\nThe 16 commands below ({a,b} is a or b):\n"+
			"    app {solo,big}\n"+
			"    app {user,team} {get,ls}\n"+
			"    app role {get,ls,watch}\n"+
			"    app org member {add,erase}\n\n"))
	}))
	t.Run("and the commands below one of them, when they fit", x.F(func(x x.X) {
		asAgent(x.T)
		got := xlitest.Run(x.T, newBigCmd(), "remote", "--help")
		x.NoError(got.Err)
		x.Contains(got.Stdout, "Name:\n    app.remote.add - add a remote\n")
		x.NotContains(got.Stdout, "more than an AI agent's")
	}))
	t.Run("one command below is one", x.F(func(x x.X) {
		newCmd := func() *xli.Command {
			return &xli.Command{Name: "app", Commands: xli.Commands{
				{Name: "big", Synop: strings.Repeat("x", xli.AgentHelpMax)},
			}}
		}
		x.Equal(""+
			"\nThe command below:\n"+
			"    app big\n"+
			"\nIts help is more than an AI agent's --help prints at once. --help on it prints\n"+
			"its help; --help-all prints all of it "+sizeOf(x, newCmd)+".\n",
			past(x, newCmd, "--help"))
	}))
	t.Run("a map that does not fit beside the command's help is a count", x.F(func(x x.X) {
		newCmd := func() *xli.Command {
			return &xli.Command{Name: "app", Synop: strings.Repeat("x", xli.AgentHelpMax), Commands: xli.Commands{
				{Name: "a"},
				{Name: "b"},
			}}
		}
		x.Equal(""+
			"\nThe 2 commands below are not listed: their help, and even the list of them,\n"+
			"is more than an AI agent's --help prints at once. --help on a command above\n"+
			"prints its help with the commands below it; --help-all prints all of it\n"+
			sizeOf(x, newCmd)+".\n",
			past(x, newCmd, "--help"))

		newOne := func() *xli.Command {
			return &xli.Command{Name: "app", Synop: strings.Repeat("x", xli.AgentHelpMax), Commands: xli.Commands{
				{Name: "a"},
			}}
		}
		x.Equal(""+
			"\nThe command below is not listed: its help, and even its name, is more than an\n"+
			"AI agent's --help prints at once. --help on it prints its help; --help-all\n"+
			"prints all of it "+sizeOf(x, newOne)+".\n",
			past(x, newOne, "--help"))
	}))
	t.Run("the help of the command itself is never left out", x.F(func(x x.X) {
		asAgent(x.T)
		c := &xli.Command{Name: "app", Synop: strings.Repeat("x", xli.AgentHelpMax)}
		got := xlitest.Run(x.T, c, "--help")
		x.NoError(got.Err)
		x.Contains(got.Stdout, strings.Repeat("x", xli.AgentHelpMax))
		x.NotContains(got.Stdout, "more than an AI agent's")
	}))
	t.Run("--help-all prints all of it however long", x.F(func(x x.X) {
		asAgent(x.T)
		got := xlitest.Run(x.T, newBigCmd(), "--help-all")
		x.NoError(got.Err)
		x.Contains(got.Stdout, "Name:\n    app.big - say a lot\n")
		x.Contains(got.Stdout, "Name:\n    app.remote.add - add a remote\n")
		x.NotContains(got.Stdout, "more than an AI agent's")
	}))
}

func TestExamples(t *testing.T) {
	newCmd := func() *xli.Command {
		return &xli.Command{
			Name:  "deploy",
			Flags: flg.Flags{&flg.String{Name: "port", Brief: "listen port"}},
			Examples: `
				# Deploy the web service on port 9090.
				deploy --port 9090 web

				# A line indented further stays so.
				deploy \
				    --port 9090 web
			`,
		}
	}

	t.Run("end the help, indented as the rest of it", x.F(func(x x.X) {
		b := &strings.Builder{}
		x.NoError(newCmd().PrintHelp(b))
		x.True(strings.HasSuffix(b.String(), " listen port\n\n"+
			"Examples:\n"+
			"    # Deploy the web service on port 9090.\n"+
			"    deploy --port 9090 web\n"+
			"\n"+
			"    # A line indented further stays so.\n"+
			"    deploy \\\n"+
			"        --port 9090 web\n"), b.String())
	}))
	t.Run("are not there when there are none", x.F(func(x x.X) {
		for _, v := range []string{"", "\n\t\t\n\t"} {
			c := newCmd()
			c.Examples = v
			b := &strings.Builder{}
			x.NoError(c.PrintHelp(b))
			x.NotContains(b.String(), "Examples:")
			x.True(strings.HasSuffix(b.String(), " listen port\n"), b.String())
		}
	}))
}

// envFlag reports an environment variable for a flag, as cfg.Bind does.
type envFlag struct {
	flg.Flag
	env string
}

func (f envFlag) Info() *flg.Info {
	info := f.Flag.Info()
	info.Env = f.env
	return info
}
