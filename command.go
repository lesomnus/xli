package xli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/lesomnus/xli/internal"
	"github.com/lesomnus/xli/lex"
	"github.com/lesomnus/xli/mode"
)

type Command struct {
	Name    string
	Aliases []string
	Brief   string
	Synop   string
	Usage   Stringer

	Flags    Flags
	Args     Args
	Commands Commands

	io.Reader
	io.Writer
	ErrWriter io.Writer

	PreAction  Action // Executed before args and flags are parsed.
	Action     Action // Executed after args and flags are parsed.
	PostAction Action // Executed after action, regardless of whether action returns an error.

	parent *Command
}

type Commands []*Command

func (cs Commands) Get(name string) *Command {
	for _, c := range cs {
		if c.Name == name {
			return c
		}
	}

	return nil
}

func (c *Command) Run(ctx context.Context, args []string) (context.Context, error) {
	m := mode.From(ctx)
	if m == mode.Unspecified {
		m = mode.Resolve(args)
		ctx = mode.Into(ctx, m)
	}

	return c.run(ctx, []string{}, args)
}

func (c *Command) run(ctx context.Context, args_prev []string, args_rest []string) (context.Context, error) {
	ctx_, err := c.action(ctx, args_prev, args_rest)
	if ctx_ != nil {
		ctx = ctx_
	}

	errs := []error{err, nil}
	if a := c.PostAction; a != nil {
		ctx_, errs[1] = c.PostAction(ctx, c)
		if ctx_ != nil {
			ctx = ctx_
		}
	}

	return ctx, errors.Join(errs...)
}

func (c *Command) action(ctx context.Context, args_prev []string, args_rest []string) (context.Context, error) {
	if a := c.PreAction; a != nil {
		ctx_, err := c.PreAction(ctx, c)
		if ctx_ != nil {
			ctx = ctx_
		}
		if err != nil {
			return ctx, err
		}
	}

	var (
		flags  = c.Flags
		params = c.Args
		action = c.Action
	)
	for i := 0; i < len(args_rest); i++ {
		switch v := lex.Lex(args_rest[i]).(type) {
		case *lex.Err:
			return ctx, v

		case *lex.Flag:
			j := slices.IndexFunc(flags, func(f Flag) bool {
				return f.Info().Name == v.Name()
			})
			if j < 0 {
				return ctx, fmt.Errorf("unknown flag: %s", v)
			}

			f := flags[j]
			a := v.Arg()
			if _, ok := f.(internal.FlagTagger[bool]); ok {
				// Flag is switch
				if a == nil {
					// mock arg.
					b := lex.Arg("true")
					a = &b
				}
			} else if a == nil {
				// Flag is not switch and requires value but does not have one.
				i++
				if i == len(args_rest) {
					// There is no more args.
					return ctx, fmt.Errorf("%s: no value is given", v)
				} else {
					switch w := lex.Lex(args_rest[i]).(type) {
					case *lex.Err:
						return ctx, fmt.Errorf("%s: %w", v, w)
					case *lex.Flag:
						return ctx, fmt.Errorf("%s: no value is given but was flag: %s", v, w)
					case lex.Arg:
						a = &w
					default:
						panic("unknown lex item")
					}
				}
			}

			ctx_, err := f.Handle(ctx, c, a.Raw())
			if ctx_ != nil {
				ctx = ctx_
			}
			if err != nil {
				return ctx, fmt.Errorf("invalid flag: %s: %w", v, err)
			}

		case lex.Arg:
			// Once `arg` is parsed, there should be no more flags.
			// It prevents additional flags are to be parsed
			flags = nil

			if len(params) > 0 {
				// TODO: some param may want to have multiple arguments
				param := params[0]
				params = params[1:]

				ctx_, n, err := param.Prase(ctx, c, args_prev, args_rest[i:])
				i += n - 1 // Note that `i` is incremented by "for" statement.
				if ctx_ != nil {
					ctx = ctx_
				}
				if err != nil {
					return ctx, fmt.Errorf("invalid argument: %s: %w", v, err)
				}

				// Save current parsed args in the list
				args_prev = append(args_prev, v.Raw())

				continue
			}

			j := slices.IndexFunc(c.Commands, func(c *Command) bool {
				return c.Name == v.Raw()
			})
			if j < 0 {
				return ctx, fmt.Errorf("unknown subcommand: %s", v.Raw())
			}

			if action != nil {
				ctx_, err := action(ctx, c)
				action = nil
				if ctx_ != nil {
					ctx = ctx_
				}
				if err != nil {
					return ctx, err
				}
			}

			// Save current parsed args in the list
			args_prev = append(args_prev, v.Raw())
			return c.Commands[j].run(ctx, args_prev, args_rest[i+1:])

		default:
			panic("unknown lex item")
		}
	}

	if action != nil {
		ctx_, err := action(ctx, c)
		if ctx_ != nil {
			ctx = ctx_
		}
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

func (c *Command) Parent() *Command {
	return c.parent
}

func (c *Command) Root() *Command {
	p := c
	for p != nil {
		p = p.parent
	}
	return p
}
