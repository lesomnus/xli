package xli

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/template"

	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/lex"
	"github.com/lesomnus/xli/mode"
	"github.com/lesomnus/xli/tab"
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

	Handler Handler

	io.ReadCloser
	io.WriteCloser
	ErrWriter io.WriteCloser

	parent *Command
}

func (c *Command) GetName() string {
	return c.Name
}

func (c *Command) GetFlags() flg.Flags {
	return c.Flags
}

func (c *Command) GetArgs() arg.Args {
	return c.Args
}

func (c *Command) String() string {
	vs := make([]string, 1, len(c.Aliases)+1)
	vs[0] = c.Name
	vs = append(vs, c.Aliases...)
	return strings.Join(vs, ",")
}

func (c *Command) HasParent() bool {
	return c.parent != nil
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

// Run parses the `args` and executes the `c.Handler`.
// It runs subcommand after all arguments are parsed if found one.
// Any `Flag`s or `Arg`s including the one in the subcommands returns error, it stops running and returns the error.
// It will not executes the subcommand if "--help" or "-h" is found in the execution command.
// Handler has responsible to execute subcommand's handler.
// This function does not guarantees execution of subcommand's handler.
func (c *Command) Run(ctx context.Context, args []string) error {
	if l := len(args); l > 2 {
		tag := args[l-3]
		if sh, ok := strings.CutPrefix(tag, completion_tag_prefix); ok {
			curr := args[l-2] // Word where the cursor is.
			buff := args[l-1] // len(curr) characters on left of the cursor.

			w := c.WriteCloser
			if w == nil {
				w = os.Stdout
			}

			var t tab.Tab
			switch sh {
			case "zsh":
				t = tab.NewZshTab(w)
			default:
				return errors.New("unknown shell of completion")
			}

			ctx = tab.Into(ctx, t)
			args = NormalizeCompletionArgs(args[:l-3], curr, buff)
			return c.runCompletion(ctx, args)
		}
	}

	f_root, err := parseFrameAll(c, args)
	if err != nil {
		return err
	}

	// Parses flags and args according to the collected information.
	// Parsed flags and args should be stored in each Arg and Flag.
	for f := range f_root.Iter() {
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

	// Handlers are invoked sequentially.
	return f_root.execute(ctx)
}

// args must be normalized one by `NormalizeCompletionArgs`.
func (c *Command) runCompletion(ctx context.Context, args []string) error {
	tab := tab.From(ctx)
	if tab == nil {
		panic("tab must exist")
	}

	f_root, parse_err := parseFrameAll(c, args)
	f_last := f_root.Last()

	need_val := false
	need_arg := false
	if parse_err != nil {
		need_val = errors.Is(parse_err, ErrNoFlagValue)
		need_arg = errors.Is(parse_err, ErrNeedArgs)
		if !(need_val || need_arg) {
			return parse_err
		}
	}

	c = f_last.c_curr
	need_arg = need_arg || slices.ContainsFunc(c.Args, func(a arg.Arg) bool {
		return a.IsOptional()
	})

	switch {
	case need_val || need_arg:
		break

	case len(args) == 0:
		fallthrough
	case !strings.HasPrefix(args[len(args)-1], "--"):
		for _, v := range c.Commands {
			tab.ValueD(v.Name, v.Brief)
		}
		return nil

	case len(c.Flags) == 0:
		return nil

	case !strings.HasSuffix(args[len(args)-1], "="):
		// last == "--"
		for _, u := range c.Flags {
			v := u.Info()
			tab.ValueD(fmt.Sprintf("--%s", v.Name), v.Brief)
		}
		return nil

	default:
		// Some args are given.
		// Required args are given if the command needs some.
		// The command has flags and value of one of the flags needed to be completed.
		need_val = true
	}

	if f_root != f_last {
		// Detach last frame.
		f_last.prev.next = nil
		f_last.prev = nil

		ctx = mode.Into(ctx, mode.Tab|mode.Pass)
		for f := range f_root.Iter() {
			if err := f.prepare(ctx); err != nil {
				// TODO: should I ignore some parse error?
				return nil
			}
		}
		if err := f_root.execute(ctx); err != nil {
			return nil
		}
	}

	ctx = mode.Into(ctx, mode.Tab)
	if need_val {
		a := args[len(args)-1]
		n := lex.Flag(a).Name()
		if v := c.Flags.Get(n); v != nil {
			v.Handle(ctx, "")
		}
	} else if need_arg {
		i := len(f_last.args)
		if l := len(c.Args); i >= l {
			i = l - 1
		}
		v := c.Args[i]
		v.Prase(ctx, nil)
	} else {
		panic("some completion not considered")
	}

	return nil
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
