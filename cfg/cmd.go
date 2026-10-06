package cfg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
)

// NewCmdConfig is `config`, which prints the loaded configuration, and
// `config env`, which lists the environment variables it reads. Mount it under
// the command Load is on.
//
// `config` prints a configuration that fails to load as far as it was read,
// and then says what is wrong with it; `config env` does not load it.
func NewCmdConfig[T any](l *Loader[T]) *xli.Command {
	env := newCmdConfigEnv(l)
	c := &xli.Command{
		Name:  "config",
		Brief: "print the configuration, as it was read",
		Synop: "Every value is followed by where it came from. Secrets are redacted; a reference is printed as written.",

		Commands: xli.Commands{env},

		Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			if f := l.failed.Load(); f != nil {
				if f.s != nil {
					if err := f.s.Print(cmd); err != nil {
						return err
					}
				}
				return f.err
			}
			s := l.Current()
			if s == nil {
				return errors.New("cfg: the configuration was not loaded; mount cfg.Load on the root command")
			}
			if err := s.Print(cmd); err != nil {
				return err
			}
			return next(ctx)
		}),
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.skip = append(l.skip, env)
	l.tolerant = append(l.tolerant, c)
	return c
}

func newCmdConfigEnv[T any](l *Loader[T]) *xli.Command {
	return &xli.Command{
		Name:  "env",
		Brief: "print every environment variable the configuration reads",

		Flags: flg.Flags{
			&flg.Switch{Name: "set", Brief: "say which of them are set, without saying what to"},
		},

		Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			set, _ := flg.Find[bool](cmd, "set")
			lookup := lookupIn(l.opts.environ())
			for _, name := range l.EnvNames() {
				if !set {
					cmd.Println(name)
					continue
				}
				if _, ok := lookup(name); ok {
					cmd.Printf("%s\tset\n", name)
				} else {
					cmd.Printf("%s\t-\n", name)
				}
			}
			return next(ctx)
		}),
	}
}

// comment is the origin as a comment in Print.
func (o Origin) comment() string {
	var c string
	switch o.Source {
	case File:
		c = fmt.Sprintf("%s:%d", o.Name, o.Line)
	case Env, Flag:
		c = o.Name
	case Default:
		c = "default"
	default:
		return ""
	}
	if o.Cleared {
		c += " (cleared)"
	}
	if len(o.Refs) > 0 {
		c += " via " + strings.Join(o.Refs, ", ")
	}
	return c
}
