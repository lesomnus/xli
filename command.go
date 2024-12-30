package xli

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/template"

	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/mode"
)

type Command struct {
	Category string
	Name     string
	Aliases  []string
	Brief    string
	Synop    string
	Usage    Stringer

	Flags    flg.Flags
	Args     arg.Args
	Commands Commands

	Action Action

	io.ReadCloser
	io.WriteCloser
	ErrWriter io.WriteCloser

	parent *Command
}

func (c *Command) String() string {
	vs := make([]string, 1, len(c.Aliases)+1)
	vs[0] = c.Name
	vs = append(vs, c.Aliases...)
	return strings.Join(vs, ",")
}

func (c *Command) Parent() *Command {
	return c.parent
}

func (c *Command) Tree() []*Command {
	vs := []*Command{}
	p := c
	for p != nil {
		vs = append(vs, p)
		p = p.parent
	}
	slices.Reverse(vs)
	return vs
}

func (c *Command) Root() *Command {
	p := c
	for p != nil {
		p = p.parent
	}
	return p
}

func (c *Command) GetFlags() flg.Flags {
	return c.Flags
}

func (c *Command) GetArgs() arg.Args {
	return c.Args
}

func (c *Command) Print(vs ...any) (int, error) {
	return fmt.Fprint(c.WriteCloser, vs...)
}

func (c *Command) Printf(format string, vs ...any) (int, error) {
	return fmt.Fprintf(c.WriteCloser, format, vs...)
}

func (c *Command) Println(vs ...any) (int, error) {
	return fmt.Fprintln(c.WriteCloser, vs...)
}

func (c *Command) Scan(vs ...any) (int, error) {
	return fmt.Fscan(c.ReadCloser, vs...)
}

func (c *Command) Scanf(format string, vs ...any) (int, error) {
	return fmt.Fscanf(c.ReadCloser, format, vs...)
}

func (c *Command) Scanln(vs ...any) (int, error) {
	return fmt.Fscanln(c.ReadCloser, vs...)
}

// Run parses the `args` and executes the `c.Action`.
// It runs subcommand after all arguments are parsed if found one.
// Any `Flag`s or `Arg`s including the one in the subcommands returns error, it stops running and returns the error.
// It will not executes the subcommand if "--help" or "-h" is found in the execution command.
// Action has responsible to execute subcommand's action.
// This function does not guarantees execution of subcommand's action.
func (c *Command) Run(ctx context.Context, args []string) error {
	f_root, err := parseFrame(c, args)
	if err != nil {
		return err
	}

	// Collects flags, args, and subcommands to be executed are without paring.
	// Collected information are stored in the `frame` for each command.
	// Root frame, `f_root`, holds `c`.
	for f := f_root; f.c_next != nil; f = f.next {
		f_next, err := parseFrame(f.c_next, f.rest)
		if err != nil {
			return err
		}
		if f_next == nil {
			break
		}

		f.next = f_next
	}

	// Parses flags and args according to the collected information.
	// Parsed flags and args should be stored in each Arg and Flag.
	for f := f_root; f != nil; f = f.next {
		if err := f.prepare(ctx); err != nil {
			return err
		}
	}

	// Set a mode if not set.
	if m := mode.From(ctx); m == mode.Unspecified {
		m = mode.Run
		for f := f_root; f != nil; f = f.next {
			if f.is_help {
				m = mode.Help
				break
			}
		}

		ctx = mode.Into(ctx, m|mode.Pass)
	}

	// Set ios if not set.
	if c.ReadCloser == nil {
		c.ReadCloser = os.Stdin
	}
	if c.WriteCloser == nil {
		c.WriteCloser = os.Stdout
	}
	if c.ErrWriter == nil {
		c.ErrWriter = os.Stderr
	}

	// Actions are invoked sequentially.
	return f_root.execute(ctx)
}

//go:embed help.go.tpl
var DefaultHelpTemplate string

func (c *Command) PrintHelp(w io.Writer) error {
	// TODO: custom template; pass by context?
	tpl := template.New("")
	if _, err := tpl.Parse(DefaultHelpTemplate); err != nil {
		panic(err)
	}

	return tpl.Execute(w, c)
}

type Commands []*Command

func (cs Commands) Get(name string) *Command {
	for _, c := range cs {
		if c.Name == name {
			return c
		}
		if slices.Contains(c.Aliases, name) {
			return c
		}
	}

	return nil
}

func (cs Commands) ByCategory() []Commands {
	i := map[string]int{}
	vs := []Commands{}
	for _, c := range cs {
		j, ok := i[c.Category]
		if !ok {
			j = len(vs)
			i[c.Category] = j
			vs = append(vs, Commands{})
		}

		vs[j] = append(vs[j], c)
	}
	return vs
}

func (cs Commands) WithCategory(name string, vs ...*Command) Commands {
	for _, v := range vs {
		v.Category = name
	}
	return append(cs, vs...)
}
