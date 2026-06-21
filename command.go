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
	"unicode/utf8"

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
	io.Writer
	ErrWriter io.Writer

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
	for p.parent != nil {
		p = p.parent
	}
	return p
}

func (c *Command) Print(vs ...any) (int, error) {
	return fmt.Fprint(c.Writer, vs...)
}

func (c *Command) Printf(format string, vs ...any) (int, error) {
	return fmt.Fprintf(c.Writer, format, vs...)
}

func (c *Command) Println(vs ...any) (int, error) {
	return fmt.Fprintln(c.Writer, vs...)
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

			w := c.Writer
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
		if f.is_help {
			break
		}
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

	// Enforce required flags, but only when actually running the command;
	// --help and completion must work without them.
	if mode.From(ctx).Is(mode.Run) {
		for f := f_root; f != nil; f = f.next {
			for _, fl := range f.c_curr.Flags {
				if info := fl.Info(); info.Required && fl.Count() == 0 {
					return fmt.Errorf("%w: --%s", ErrFlagRequired, info.Name)
				}
			}
		}
	}

	// Set ios if not set.
	if c.ReadCloser == nil {
		c.ReadCloser = os.Stdin
	}
	if c.Writer == nil {
		c.Writer = os.Stdout
	}
	if c.ErrWriter == nil {
		c.ErrWriter = os.Stderr
	}

	// Handlers are invoked sequentially.
	return f_root.execute(ctx)
}

// args must be a normalized one by `NormalizeCompletionArgs`.
func (c *Command) runCompletion(ctx context.Context, args []string) error {
	tab := tab.From(ctx)
	if tab == nil {
		// Completion must never crash the user's shell; without a sink
		// there is nothing to emit.
		return nil
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

	last := ""
	if len(args) > 0 {
		last = args[len(args)-1]
	}

	// A trailing "=" ("--flag=" or "-x=") means a flag value is being
	// completed; this takes precedence over a missing required argument so
	// that a command with required args can still complete its flag values.
	if strings.HasSuffix(last, "=") {
		need_val = true
		need_arg = false
	}
	if !need_val {
		need_arg = need_arg || slices.ContainsFunc(c.Args, func(a arg.Arg) bool {
			return a.IsOptional()
		})
	}

	switch {
	case need_val || need_arg:
		// A flag value or an argument is being completed; handled below.

	case strings.HasPrefix(last, "-"):
		// "--" or "-": suggest flag names.
		for _, u := range c.Flags {
			v := u.Info()
			tab.ValueD(fmt.Sprintf("--%s", v.Name), v.Brief)
		}
		return nil

	default:
		// Start of a (sub)command or a non-flag token: suggest subcommands.
		for _, v := range c.Commands {
			tab.ValueD(v.Name, v.Brief)
		}
		return nil
	}

	if f_root != f_last {
		// Detach last frame.
		f_last.prev.next = nil
		f_last.prev = nil

		ctx = mode.Into(ctx, mode.Tab|mode.Pass)
		for f := range f_root.Iter() {
			if err := f.prepare(ctx); err != nil {
				// Parent frames are only walked to set up context for the
				// command being completed; a parse error there just means
				// there is nothing to complete, so emit nothing.
				return nil
			}
		}
		if err := f_root.execute(ctx); err != nil {
			return nil
		}
	}

	ctx = mode.Into(ctx, mode.Tab)
	if need_val {
		f := lex.Flag(args[len(args)-1])
		var v flg.Flag
		if f.IsShort() {
			r, _ := utf8.DecodeRuneInString(f.Name())
			v = c.Flags.GetByAlias(r)
		} else {
			v = c.Flags.Get(f.Name())
		}
		if v != nil {
			v.Handle(ctx, "")
		}
	} else if need_arg && len(c.Args) > 0 {
		i := len(f_last.args)
		if i >= len(c.Args) {
			i = len(c.Args) - 1
		}
		if h := c.Args[i].Info().Handle; h != nil {
			h(ctx)
		}
	}

	return nil
}

//go:embed help.go.tpl
var DefaultHelpTemplate string

// defaultHelpTemplate is parsed once at startup; the embedded template is a
// compile-time constant, so a parse failure is a programmer error.
var defaultHelpTemplate = template.Must(template.New("help").Parse(DefaultHelpTemplate))

func (c *Command) PrintHelp(w io.Writer) error {
	// TODO(Phase 4): allow a user-supplied template once the injection
	// surface (Command field vs context) is decided.
	return defaultHelpTemplate.Execute(w, c)
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
