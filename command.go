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

	io.ReadCloser
	io.WriteCloser
	ErrWriter io.WriteCloser

	PreAction  Action // Executed before args and flags are parsed.
	Action     Action // Executed after args and flags are parsed.
	PostAction Action // Executed after action, regardless of whether action returns an error.

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

func (c *Command) Run(ctx context.Context, args []string) (context.Context, error) {
	m := mode.From(ctx)
	if m == mode.Unspecified {
		m = mode.Resolve(args)
		ctx = mode.Into(ctx, m|mode.Pass)
	}

	return c.run(ctx, []string{}, args)
}

func (c *Command) run(ctx context.Context, args_prev []string, args_rest []string) (context.Context, error) {
	if c.PreAction == nil {
		c.PreAction = noop
	}
	if c.Action == nil {
		c.Action = noop
	}
	if c.PostAction == nil {
		c.PostAction = noop
	}
	if c.ReadCloser == nil {
		c.ReadCloser = os.Stdin
	}
	if c.WriteCloser == nil {
		c.WriteCloser = os.Stdout
	}
	if c.ErrWriter == nil {
		c.ErrWriter = os.Stderr
	}

	ctx_, err := c.action(ctx, args_prev, args_rest)
	if ctx_ != nil {
		ctx = ctx_
	}

	errs := []error{err, nil}
	ctx_, errs[1] = c.PostAction(ctx, c)
	if ctx_ != nil {
		ctx = ctx_
	}

	m := mode.From(ctx)
	ctx = mode.Into(ctx, m|mode.Pass)
	return ctx, errors.Join(errs...)
}

func (c *Command) action(ctx context.Context, args_prev []string, args_rest []string) (context.Context, error) {
	is_opt := slices.ContainsFunc(c.Args, func(a arg.Arg) bool {
		return a.IsOptional()
	})
	if is_opt && len(c.Commands) > 0 {
		panic(fmt.Sprintf("%s: command cannot have optional argument if it has subcommands", c.String()))
	}

	var (
		flags  = []*lex.Flag{}
		args   = []string{}
		remain = []string{}

		c_next *Command
	)
	// It collects `flags` and `args` without parsing.
	// Sets `c_next` if the subcommand is found.
L:
	for i := 0; i < len(args_rest); i++ {
		t := lex.Lex(args_rest[i])
		switch v := t.(type) {
		case *lex.Err:
			return ctx, v

		case lex.EndOfCommand:
			remain = args_rest[i:]
			args_rest = []string{}
			break L

		case *lex.Flag:
			if len(args) > 0 {
				return ctx, fmt.Errorf("flags are must be set at the behind of the arguments: %s", v)
			}
			if n := v.Name(); n == "help" || n == "h" {
				m := mode.From(ctx).NoPass()
				ctx = mode.Into(ctx, m)
				ctx_, err := c.PreAction(ctx, c)
				if ctx_ != nil {
					ctx = ctx_
				}
				c.PrintHelp(c)
				return ctx, err
			}

			f := c.Flags.Get(v.Name())
			if f == nil {
				return ctx, fmt.Errorf("unknown flag: %s", v)
			}

			if _, ok := f.(*flg.Switch); ok {
				// Flag is a switch
				if v.Arg() == nil {
					v = v.WithArg("true")
				}
			} else if v.Arg() == nil {
				// Flag is not a switch and requires value but does not have one.
				i++
				if i == len(args_rest) {
					// There are no more args.
					return ctx, fmt.Errorf("%s: no value is given", v)
				}

				switch w := lex.Lex(args_rest[i]).(type) {
				case *lex.Err:
					return ctx, fmt.Errorf("%s: %w", v, w)
				case *lex.Flag:
					return ctx, fmt.Errorf("%s: no value is given but was flag: %s", v, w)
				case lex.Arg:
					v = v.WithArg(w)
				default:
					panic("unknown lex item")
				}
			}

			flags = append(flags, v)

		case lex.Arg:
			if is_opt || len(args) < len(c.Args) {
				args = append(args, v.Raw())
				continue
			}
			if len(c.Commands) == 0 {
				return ctx, fmt.Errorf("too many arguments: %s", v)
			}

			c_next = c.Commands.Get(v.Raw())
			if c_next == nil {
				return ctx, fmt.Errorf("unknown subcommand: %s", v)
			}

			args_rest = args_rest[i+1:]
			break L

		default:
			panic("unknown lex item")
		}
	}

	if c_next == nil {
		// There are no more subcommands.
		m := mode.From(ctx).NoPass()
		ctx = mode.Into(ctx, m)
	}

	if a := c.PreAction; a != nil {
		ctx_, err := c.PreAction(ctx, c)
		if ctx_ != nil {
			ctx = ctx_
		}
		if err != nil {
			return ctx, err
		}
	}

	for _, v := range flags {
		h := c.Flags.Get(v.Name())
		if h == nil {
			return ctx, fmt.Errorf("unknown flag: %s", h)
		}

		ctx_, err := h.Handle(ctx, v.Arg().Raw())
		if ctx_ != nil {
			ctx = ctx_
		}
		if err != nil {
			return ctx, fmt.Errorf("invalid flag: %s: %w", v, err)
		}
	}

	i := 0
	for _, h := range c.Args {
		if i == len(args) {
			if h.IsOptional() {
				break
			}
			if _, ok := h.(*arg.Remains); ok {
				break
			}
			return ctx, fmt.Errorf("argument not given: %q", h.Info().Name)
		}

		// Parser can consume multiple arguments.
		ctx_, n, err := h.Prase(ctx, args_prev, args[i:])
		if i+n > len(args) {
			panic(fmt.Sprintf(`argument parser reported that it parsed more arguments than were given: "%s" parse %v`, h.Info().Name, args[i:]))
		}
		args_prev = append(args_prev, args[i:i+n]...)
		if ctx_ != nil {
			ctx = ctx_
		}
		if err != nil {
			return ctx, fmt.Errorf("invalid argument: %q: %w", args[i], err)
		}

		i += n
	}
	if len(c.Args) > 0 {
		if h, ok := c.Args[len(c.Args)-1].(*arg.Remains); ok {
			if !h.IsOptional() && len(remain) == 0 {
				return ctx, fmt.Errorf("argument not given: %q", h.Info().Name)
			}

			ctx_, _, err := h.Prase(ctx, args_prev, remain)
			if ctx_ != nil {
				ctx = ctx_
			}
			if err != nil {
				return ctx, fmt.Errorf("invalid argument: %q: %w", remain, err)
			}
		}
	}

	ctx_, err := c.Action(ctx, c)
	if ctx_ != nil {
		ctx = ctx_
	}
	if err != nil {
		return ctx, err
	}
	if c_next != nil {
		c_next.parent = c
		return c_next.run(ctx, args_prev, args_rest)
	}
	return ctx, nil
}

//go:embed help.go.tpl
var DefaultHelpTemplate string

func (c *Command) PrintHelp(w io.Writer) {
	// TODO: custom template; pass by context?
	tpl := template.New("")
	if _, err := tpl.Parse(DefaultHelpTemplate); err != nil {
		panic(err)
	}
	if err := tpl.Execute(w, c); err != nil {
		panic(err)
	}
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
