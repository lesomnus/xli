package xli

import (
	"context"
	"fmt"
	"iter"
	"slices"

	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/lex"
	"github.com/lesomnus/xli/mode"
)

type frame struct {
	prev *frame
	next *frame

	c_curr *Command
	c_next *Command

	flags  []lex.Flag
	args   []string
	rest   []string // args for next command
	remain []string // remain args after end of command

	is_help bool
}

func (f *frame) Iter() iter.Seq[*frame] {
	return func(yield func(*frame) bool) {
		for ; f != nil; f = f.next {
			if !yield(f) {
				return
			}
		}
	}
}

func (f *frame) Last() *frame {
	for {
		if f.next == nil {
			return f
		}
		f = f.next
	}
}

// Collects flags, args, and subcommands to be executed are without paring.
// Collected information are stored in the `frame` for each command.
// Root frame, `f_root`, holds `c`.
func parseFrameAll(cmd *Command, args_rest []string) (*frame, error) {
	root := &frame{
		c_next: cmd,
		rest:   args_rest,
	}
	for f := root; f.c_next != nil; f = f.next {
		f_next, err := parseFrame(f.c_next, f.rest)
		f_next.prev = f
		f.next = f_next
		if err != nil {
			return root.next, err
		}
		if f_next == nil {
			break
		}
	}

	return root.next, nil
}

// Parses flags, args, and subcommand for given cmd.
// It stops parsing if subcommand is found.
// It does not parse the arguments into Flag or Arg but looks only its placement.
func parseFrame(cmd *Command, args_rest []string) (*frame, error) {
	is_opt := slices.ContainsFunc(cmd.Args, func(a arg.Arg) bool {
		return a.IsOptional()
	})
	if is_opt && len(cmd.Commands) > 0 {
		panic(fmt.Sprintf("%s: command cannot have optional argument if it has subcommands", cmd.String()))
	}

	f := &frame{
		c_curr: cmd,
		flags:  []lex.Flag{},
		args:   []string{},
	}
	for i := 0; i < len(args_rest); i++ {
		t := lex.Lex(args_rest[i])
		switch v := t.(type) {
		case *lex.Err:
			return f, v

		case lex.EndOfCommand:
			f.rest = []string{}
			f.remain = args_rest[i:]
			return f, nil

		case lex.Flag:
			if len(f.args) > 0 {
				return f, fmt.Errorf("flags are must be set at the behind of the arguments: %s", v)
			}
			if n := v.Name(); n == "help" || n == "h" {
				f.is_help = true
				return f, nil
			}

			if w := cmd.Flags.Get(v.Name()); w == nil {
				return f, &FlagError{v, ErrUnknownFlag}
			} else if _, ok := w.(*flg.Switch); ok {
				// Flag is a switch
				if _, ok := v.Arg(); !ok {
					v = v.WithArg("true")
				}
			} else if _, ok := v.Arg(); !ok {
				// Flag is not a switch and requires a value but does not have one.
				i++
				if i == len(args_rest) {
					// There are no more args.
					return f, &FlagError{v, ErrNoFlagValue}
				}

				switch w := lex.Lex(args_rest[i]).(type) {
				case *lex.Err:
					return f, fmt.Errorf("%s: %w", v, w)
				case lex.EndOfCommand:
					return f, &FlagError{v, ErrNoFlagValue}
				case lex.Flag:
					return f, &FlagError{v, ErrNoFlagValue}
				case lex.Arg:
					v = v.WithArg(w)
				default:
					panic("unknown lex item")
				}
			}

			f.flags = append(f.flags, v)

		case lex.Arg:
			if is_opt || len(f.args) < len(cmd.Args) {
				f.args = append(f.args, v.Raw())
				continue
			}
			if len(cmd.Commands) == 0 {
				return f, &ArgError{v, ErrTooManyArgs}
			}

			f.c_next = cmd.Commands.Get(v.Raw())
			if f.c_next == nil {
				return f, &ArgError{v, ErrUnknownCmd}
			}

			// Subcommand is found so stop parsing.
			f.rest = args_rest[i+1:]
			return f, nil

		default:
			panic("unknown lex item")
		}
	}

	if i := len(f.args); i < len(cmd.Args) {
		a := cmd.Args[i]
		if !a.IsOptional() {
			name := fmt.Sprintf("%q", a.Info().Name)
			return f, &ArgError{lex.Arg(name), ErrNeedArgs}
		}
	}

	// All given args are parsed with no subcommand.
	return f, nil
}

// Prepares the command associated with the frame.
// Flag and Arg parser will be executed and runs next frame if exists.
func (f *frame) prepare(ctx context.Context) error {
	c := f.c_curr
	for _, v := range f.flags {
		h := c.Flags.Get(v.Name())
		if h == nil {
			return fmt.Errorf("unknown flag: %s", h)
		}

		a, ok := v.Arg()
		if !ok {
			a = lex.Arg("true")
		}
		if err := h.Handle(ctx, a.Raw()); err != nil {
			return fmt.Errorf("invalid flag: %s: %w", v, err)
		}
	}

	i := 0
	for _, h := range c.Args {
		if i == len(f.args) {
			if h.IsOptional() {
				break
			}
			if _, ok := h.(*arg.Remains); ok {
				break
			}

			panic(fmt.Sprintf("parse failed: argument not given: %q", h.Info().Name))
		}

		// Parser can consume multiple arguments.
		n, err := h.Prase(ctx, f.args[i:])
		if i+n > len(f.args) {
			panic(fmt.Sprintf(`argument parser reported that it parsed more arguments than were given: "%s" parse %v`, h.Info().Name, f.args[i:]))
		}
		if err != nil {
			return fmt.Errorf("invalid argument: %q: %w", f.args[i], err)
		}

		i += n
	}
	if len(c.Args) > 0 {
		if h, ok := c.Args[len(c.Args)-1].(*arg.Remains); ok {
			if !h.IsOptional() && len(f.remain) == 0 {
				panic(fmt.Sprintf("parse failed: argument not given: %q", h.Info().Name))
			}

			_, err := h.Prase(ctx, f.remain)
			if err != nil {
				return fmt.Errorf("invalid argument: %q: %w", f.remain, err)
			}
		}
	}

	return nil
}

// Executes the command associated with the frame.
// Unlike prepare, it executes next frame also.
func (f *frame) execute(ctx context.Context) error {
	c := f.c_curr
	if c.Handler == nil {
		c.Handler = noop
	}

	if next := f.next; next != nil {
		next.c_curr.parent = c
		return c.Handler.Handle(ctx, c, func(ctx context.Context) error {
			c_next := next.c_curr
			if c_next.ReadCloser == nil {
				c_next.ReadCloser = c.ReadCloser
			}
			if c_next.WriteCloser == nil {
				c_next.WriteCloser = c.WriteCloser
			}
			if c_next.ErrWriter == nil {
				c_next.ErrWriter = c.ErrWriter
			}

			return next.execute(ctx)
		})
	}

	m := mode.From(ctx)
	ctx = mode.Into(ctx, m.NoPass())
	return c.Handler.Handle(ctx, c, func(ctx context.Context) error {
		if f.is_help {
			return c.PrintHelp(c)
		}
		return nil
	})
}
