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
	"github.com/lesomnus/xli/internal/comp"
	"github.com/lesomnus/xli/internal/lex"
	"github.com/lesomnus/xli/mode"
	"github.com/lesomnus/xli/tab"
)

// Command is a node of the command tree: what it is called, what it takes, the
// commands below it, and the handler that runs on the way through it. A tree
// is built once and run once; see the package documentation.
type Command struct {
	// Category groups the command under a heading in its parent's help and
	// completion.
	Category string
	// Name is the word that selects the command on the command line, and
	// Aliases are other words that do.
	Name    string
	Aliases []string
	// Brief is a line saying what the command does, shown beside its name;
	// Synop is the longer description its own help shows.
	Brief string
	Synop string
	// Examples are command lines that use the command, each with a comment
	// above it if it needs one. They end its help and are in the generated
	// documentation. The blank lines around them and the indentation they all
	// share are not part of them, so a raw string can be indented with the
	// code around it.
	Examples string

	// Flags are the command's own flags, and Args its arguments, in order.
	// Neither is given to its parent or its subcommands.
	Flags flg.Flags
	Args  arg.Args
	// Commands are the commands below this one.
	Commands Commands

	// Exclusive lists groups of flag names of which at most one may be given,
	// e.g. {{"json", "yaml"}}. Run returns ErrFlagConflict otherwise.
	Exclusive [][]string

	// Handler is the command's middleware; nil is one that calls next.
	Handler Handler

	// Hidden omits the command from its parent's help and completion; it can
	// still be run by name.
	Hidden bool

	// The command's input and outputs. Run sets the root's to os.Stdin,
	// os.Stdout and os.Stderr where they are nil, and a subcommand's to its
	// parent's where they are nil.
	io.ReadCloser
	io.Writer
	ErrWriter io.Writer

	parent *Command
}

// GetName is Name. GetName, GetFlags and GetArgs are what the frm package
// knows of a command; see [xmd.Command].
func (c *Command) GetName() string {
	return c.Name
}

// GetFlags is Flags.
func (c *Command) GetFlags() flg.Flags {
	return c.Flags
}

// GetArgs is Args.
func (c *Command) GetArgs() arg.Args {
	return c.Args
}

// String is the command's name and aliases, separated by commas.
func (c *Command) String() string {
	vs := make([]string, 1, len(c.Aliases)+1)
	vs[0] = c.Name
	vs = append(vs, c.Aliases...)
	return strings.Join(vs, ",")
}

// HasParent reports whether the command is below another in the run: Run sets
// the parent of each command on the path it takes.
func (c *Command) HasParent() bool {
	return c.parent != nil
}

// Parent is the command above this one in the run, or nil for the root or a
// command not on the path of a run.
func (c *Command) Parent() *Command {
	return c.parent
}

// Tree is the path of the run from the root to this command, the root first.
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

// Root is the command at the top of the run.
func (c *Command) Root() *Command {
	p := c
	for p.parent != nil {
		p = p.parent
	}
	return p
}

// Print writes to the command's Writer as fmt.Fprint does.
func (c *Command) Print(vs ...any) (int, error) {
	return fmt.Fprint(c.Writer, vs...)
}

// Printf writes to the command's Writer as fmt.Fprintf does.
func (c *Command) Printf(format string, vs ...any) (int, error) {
	return fmt.Fprintf(c.Writer, format, vs...)
}

// Println writes to the command's Writer as fmt.Fprintln does.
func (c *Command) Println(vs ...any) (int, error) {
	return fmt.Fprintln(c.Writer, vs...)
}

// Scan reads from the command's ReadCloser as fmt.Fscan does.
func (c *Command) Scan(vs ...any) (int, error) {
	return fmt.Fscan(c.ReadCloser, vs...)
}

// Scanf reads from the command's ReadCloser as fmt.Fscanf does.
func (c *Command) Scanf(format string, vs ...any) (int, error) {
	return fmt.Fscanf(c.ReadCloser, format, vs...)
}

// Scanln reads from the command's ReadCloser as fmt.Fscanln does.
func (c *Command) Scanln(vs ...any) (int, error) {
	return fmt.Fscanln(c.ReadCloser, vs...)
}

// Run runs the command line args -- usually os.Args[1:] -- against the tree
// below c.
//
// It parses the whole line first, into the flags and arguments of every
// command on the path -- calling their handlers -- and returns a [UsageError]
// for what does not fit before any command's handler runs. Then it calls c's
// handler, and a subcommand's handler runs only when its parent's calls next:
// Run does not call it on the handler's behalf. With --help or -h on the line
// it runs in help mode, and the help of the command it was given to is printed
// when that command's handler calls next. --help-all prints the help of every
// command below it as well, and so do --help and -h when an AI agent runs the
// command and that is not more than it takes in at once; see the package
// documentation. A line from a generated completion script runs in completion
// mode.
//
// Run writes into the tree: the values it parses, each command's parent, and
// the IO a subcommand inherits. A tree is run once.
func (c *Command) Run(ctx context.Context, args []string) error {
	if l := len(args); l > 2 {
		tag := args[l-3]
		if sh, ok := strings.CutPrefix(tag, comp.TagPrefix); ok {
			curr := args[l-2] // Word where the cursor is.
			buff := args[l-1] // len(curr) characters on left of the cursor.

			w := c.Writer
			if w == nil {
				w = os.Stdout
			}

			var t tab.Tab
			switch sh {
			case "zsh", "bash", "fish":
				// The line format comp.Writer writes is shell-agnostic; each
				// generated script decodes it with what its shell supports.
				t = comp.NewWriter(w)
			default:
				return errors.New("unknown shell of completion")
			}

			ctx = tab.Into(ctx, t)
			args = normalizeCompletionArgs(args[:l-3], curr, buff)
			return c.runCompletion(ctx, args)
		}
	}

	f_root, err := parseFrameAll(c, args)
	if err != nil {
		return err
	}

	// Set a mode if not set. It is decided before the flags and arguments are
	// parsed: their handlers are called as they are, and see it.
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

	// Parses flags and args according to the collected information.
	// Parsed flags and args should be stored in each Arg and Flag. A
	// command's own see the mode its handler will: with Pass on the way to a
	// subcommand, and without it on the last command.
	for f := range f_root.Iter() {
		if f.is_help {
			break
		}
		fctx := ctx
		if f.next == nil {
			fctx = mode.Into(ctx, mode.From(ctx).NoPass())
		}
		if err := f.prepare(fctx); err != nil {
			return &UsageError{Cmd: f.c_curr, Err: err}
		}
	}

	// Enforce required and exclusive flags, but only when actually running
	// the command; --help and completion must work regardless.
	if mode.From(ctx).Is(mode.Run) {
		for f := f_root; f != nil; f = f.next {
			for _, fl := range f.c_curr.Flags {
				if info := fl.Info(); info.Required && fl.Count() == 0 {
					return &UsageError{
						Cmd: f.c_curr,
						Err: fmt.Errorf("%w: --%s", ErrFlagRequired, info.Name),
					}
				}
			}
			if err := f.c_curr.checkExclusive(); err != nil {
				return err
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

// checkExclusive reports a UsageError if more than one flag of an Exclusive
// group was given.
func (c *Command) checkExclusive() error {
	for _, group := range c.Exclusive {
		given := []string{}
		for _, name := range group {
			fl := c.Flags.Get(name)
			if fl == nil {
				return fmt.Errorf("%s: Exclusive refers to an unknown flag %q", c.Name, name)
			}
			if fl.Count() > 0 {
				given = append(given, "--"+name)
			}
		}
		if len(given) > 1 {
			return &UsageError{
				Cmd: c,
				Err: fmt.Errorf("%w: %s", ErrFlagConflict, strings.Join(given, ", ")),
			}
		}
	}
	return nil
}

// completeCommands emits subcommand candidates, grouped by category.
func completeCommands(t tab.Tab, c *Command) {
	for _, group := range c.Commands.Visible().ByCategory() {
		sink := t
		if cat := group[0].Category; cat != "" {
			sink = t.Group(cat)
		}
		for _, v := range group {
			sink.ValueD(v.Name, v.Brief)
		}
	}
}

// completeFlagNames emits flag-name candidates, grouped by category.
func completeFlagNames(t tab.Tab, c *Command) {
	for _, group := range c.Flags.Visible().ByCategory() {
		sink := t
		if cat := group[0].Info().Category; cat != "" {
			sink = t.Group(cat)
		}
		for _, u := range group {
			v := u.Info()
			sink.ValueD(fmt.Sprintf("--%s", v.Name), v.Brief)
		}
	}
}

// args must be a normalized one by `normalizeCompletionArgs`.
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
	// normalizeCompletionArgs turns a flag name being typed into "--", so any
	// other token starting with "-" is a flag that is already complete and the
	// cursor is on the next word.
	typing_flag := last == "--"

	// Treat an optional argument as "being completed" so its values are offered
	// even with nothing typed — but not when a flag name is being typed, which
	// must still complete flag names rather than the optional arg.
	if !need_val && !typing_flag {
		need_arg = need_arg || slices.ContainsFunc(c.Args, func(a arg.Arg) bool {
			return a.IsOptional()
		})
	}

	switch {
	case need_val || need_arg:
		// A flag value or an argument is being completed; handled below.

	case typing_flag:
		// Suggest flag names, grouped by category.
		completeFlagNames(tab, c)
		return nil

	default:
		// Start of a (sub)command or a non-flag token: suggest subcommands,
		// grouped by category.
		completeCommands(tab, c)
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
			// The cursor sits past the last declared argument. Only a
			// variadic (many) trailing argument keeps accepting values, so
			// its hint may repeat; a fixed argument that is already filled
			// has nothing left to complete and must not be re-offered.
			if last := c.Args[len(c.Args)-1]; last.IsMany() {
				i = len(c.Args) - 1
			} else {
				i = -1
			}
		}
		if i >= 0 {
			if h := c.Args[i].Info().Handle; h != nil {
				h(ctx)
			}
		}
	}

	return nil
}

// helpText is the help template. It is not exported: the help is xli's, and
// there is no hook to replace it (ROADMAP, Phase 3).
//
//go:embed help.go.tpl
var helpText string

// defaultHelpTemplate is parsed once at startup; the embedded template is a
// compile-time constant, so a parse failure is a programmer error.
var defaultHelpTemplate = template.Must(template.New("help").Funcs(helpFuncs).Parse(helpText))

var helpFuncs = template.FuncMap{
	"usage":    usageLine,
	"examples": helpExamples,
}

// usageLine renders the synopsis line of c, e.g. "app deploy [options] <TARGET>".
// Ancestors contribute their name and arguments; options are placed before the
// arguments of c because flags must precede arguments.
func usageLine(c *Command) string {
	parts := []string{}
	tree := c.Tree()
	for _, p := range tree[:len(tree)-1] {
		parts = append(parts, p.Name)
		for _, a := range p.Args {
			parts = append(parts, a.Info().Usage.String())
		}
	}

	parts = append(parts, c.Name)
	if len(c.Flags.Visible()) > 0 {
		parts = append(parts, "[options]")
	}
	for _, a := range c.Args {
		parts = append(parts, a.Info().Usage.String())
	}
	if len(c.Commands.Visible()) > 0 {
		parts = append(parts, "[command]")
	}
	return strings.Join(parts, " ")
}

// helpExamples is Examples as the help prints them, indented as its other
// sections are; empty if there are none.
func helpExamples(s string) string {
	lines := exampleLines(s)
	for i, l := range lines {
		if l != "" {
			lines[i] = "    " + l
		}
	}
	return strings.Join(lines, "\n")
}

// exampleLines is Examples line by line, without the blank lines at either end,
// the indentation the lines share, or the spaces they end with.
func exampleLines(s string) []string {
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil
	}

	indent := lines[0][:len(lines[0])-len(strings.TrimLeft(lines[0], " \t"))]
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := 0
		for n < len(indent) && n < len(l) && indent[n] == l[n] {
			n++
		}
		indent = indent[:n]
	}
	for i, l := range lines {
		lines[i] = strings.TrimRight(strings.TrimPrefix(l, indent), " \t")
	}
	return lines
}

// PrintHelp writes the command's help to w: its usage line, its description,
// its arguments and flags, the commands below it by category, and its
// examples. The help is xli's; there is no template to replace it with
// (ROADMAP, Phase 3).
func (c *Command) PrintHelp(w io.Writer) error {
	return defaultHelpTemplate.Execute(w, c)
}

// printHelpAll writes the help of c and then that of every visible command
// below it, depth first and in the order c's help lists them, each after a
// rule. It is what --help-all prints. Each command is linked to its parent on
// the way, as a run links the ones on its path, so its usage line is whole.
func (c *Command) printHelpAll(w io.Writer) error {
	if err := c.PrintHelp(w); err != nil {
		return err
	}
	for _, group := range c.Commands.Visible().ByCategory() {
		for _, sub := range group {
			sub.parent = c
			if _, err := io.WriteString(w, "\n---\n\n"); err != nil {
				return err
			}
			if err := sub.printHelpAll(w); err != nil {
				return err
			}
		}
	}
	return nil
}

// Commands are the commands below one command.
type Commands []*Command

// Visible returns the commands that are not hidden.
func (cs Commands) Visible() Commands {
	vs := Commands{}
	for _, c := range cs {
		if !c.Hidden {
			vs = append(vs, c)
		}
	}
	return vs
}

// Get is the command called name, by its name or an alias; nil if there is
// none.
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

// ByCategory is cs grouped by Category, each group in the order its first
// command appears.
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

// WithCategory is cs with vs appended, each put in the category name.
func (cs Commands) WithCategory(name string, vs ...*Command) Commands {
	for _, v := range vs {
		v.Category = name
	}
	return append(cs, vs...)
}
