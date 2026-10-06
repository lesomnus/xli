package cfg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
)

// NewCmdConfig is `config`, which prints the loaded configuration, and
// `config env`, which lists the environment variables it reads. Mount it under
// the root command that Load is on.
func NewCmdConfig[T any](l *Loader[T]) *xli.Command {
	return &xli.Command{
		Name:  "config",
		Brief: "print the configuration, as it was read",
		Synop: "Every value is followed by where it came from. Secrets are redacted; a reference is printed as written.",

		Commands: xli.Commands{newCmdConfigEnv(l)},

		Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
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

// Print writes the configuration as YAML, each value commented with where it
// came from. Secrets are redacted; fields nothing set and that hold their zero
// value are left out.
func (s *Snapshot[T]) Print(w io.Writer) error {
	b := &strings.Builder{}
	root := reflect.ValueOf(s.Config).Elem()

	var prev []string
	for _, f := range s.schema.fields {
		o := s.originOf(f)
		v, ok := f.value(root, false)
		if o.Source == Unset && (!ok || v.IsZero()) {
			continue
		}

		path := strings.Split(f.key, ".")
		common := 0
		for common < len(prev) && common < len(path)-1 && prev[common] == path[common] {
			common++
		}
		for i := common; i < len(path)-1; i++ {
			fmt.Fprintf(b, "%s%s:\n", strings.Repeat("  ", i), path[i])
		}
		prev = path[:len(path)-1]

		val, err := printValue(f, v, ok)
		if err != nil {
			return fmt.Errorf("%s: %w", f.key, err)
		}
		fmt.Fprintf(b, "%s%s: %s", strings.Repeat("  ", len(path)-1), path[len(path)-1], val)
		if c := o.comment(); c != "" {
			fmt.Fprintf(b, "  # %s", c)
		}
		b.WriteByte('\n')
	}

	_, err := io.WriteString(w, b.String())
	return err
}

func printValue(f *field, v reflect.Value, ok bool) (string, error) {
	if !ok || (v.Kind() == reflect.Pointer && v.IsNil()) {
		return "null", nil
	}
	if f.secret {
		if sv, ok := v.Interface().(fmt.Stringer); ok && isSecret(f.typ) {
			return quote(sv.String()), nil
		}
		if v.IsZero() {
			return `""`, nil
		}
		return quote(redacted), nil
	}

	out, err := yaml.MarshalWithOptions(v.Interface(), yaml.Flow(true))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func quote(s string) string {
	out, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Sprintf("%q", s)
	}
	return strings.TrimSpace(string(out))
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
