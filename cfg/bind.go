package cfg

import (
	"context"
	"encoding"
	"fmt"
	"os"
	"reflect"
	"slices"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/frm"
	"github.com/lesomnus/xli/mode"
)

// TypedFlag is a flag whose value is of type T, such as *flg.String for string.
type TypedFlag[T any] interface {
	flg.Flag
	Get() (T, bool)
	GetDefault() (T, bool)
}

// binding ties a flag to a field of a loader's configuration.
type binding struct {
	owner any // the *Loader
	field *field
	name  string
	flag  flg.Flag

	// apply reads the flag's value into v; cleared reports an empty value
	// that cleared the field.
	apply func(v reflect.Value, r *resolver) (cleared bool, err error)
	// applyDefault reads the flag's default into v, if it has one.
	applyDefault func(v reflect.Value, r *resolver) (ok bool, err error)
}

// bound is a flag bound to a field. It is the flag in every respect, and
// reports the field's environment variable in its Info.
type bound[U any] struct {
	flg.Flag
	src TypedFlag[U]
	b   *binding
}

func (w *bound[U]) Unwrap() flg.Flag      { return w.Flag }
func (w *bound[U]) Get() (U, bool)        { return w.src.Get() }
func (w *bound[U]) GetDefault() (U, bool) { return w.src.GetDefault() }
func (w *bound[U]) cfgBinding() *binding  { return w.b }

func (w *bound[U]) Info() *flg.Info {
	info := w.Flag.Info()
	if info.Env == "" {
		info.Env = w.b.field.env
	}
	return info
}

// bindingOf is the binding of f, through wrappers.
func bindingOf(f flg.Flag) *binding {
	for ; f != nil; f = flg.Unwrap(f) {
		if b, ok := f.(interface{ cfgBinding() *binding }); ok {
			return b.cfgBinding()
		}
	}
	return nil
}

// Bind binds the flag f to the field dst points to in the loader's root: when
// the user gives the flag, its value is the field's, over the file and the
// environment. The flag's Default, if any, is the field's default.
//
//	cfg.Bind(l, &c.Ldap.Addr, &flg.String{Name: "listen"})
//
// dst may also point to a struct of fields, such as a TLS block, which the flag
// then sets whole; that is mostly for BindFunc.
//
// A field in a block behind a pointer has an address only once the block is
// made. Make it after New, so that the default stays nil: what the root holds
// when the loader is made is the defaults.
//
// The returned flag is f, reporting the field's environment variable for help.
// It panics if dst is not a field of the root.
func Bind[T any, F TypedFlag[T], C any](l *Loader[C], dst *T, f F) flg.Flag {
	set := func(v reflect.Value, u T) (cleared bool) {
		uv := reflect.ValueOf(&u).Elem()
		if emptyText(uv) {
			v.SetZero()
			return true
		}
		// A copy: the flag's own list must not be the configuration's.
		v.Set(clone(uv))
		return false
	}
	return bind(l, dst, f, &binding{
		apply: func(v reflect.Value, _ *resolver) (bool, error) {
			u, _ := f.Get()
			return set(v, u), nil
		},
		applyDefault: func(v reflect.Value, _ *resolver) (bool, error) {
			u, ok := f.GetDefault()
			if ok {
				set(v, u)
			}
			return ok, nil
		},
	})
}

// BindFunc binds the flag f to the field dst points to, converting the flag's
// value with conv. See Bind.
//
//	cfg.BindFunc(l, &c.Ldap.Tls, &flg.String{Name: "tls"}, parseCertPair)
func BindFunc[T, U any, F TypedFlag[U], C any](l *Loader[C], dst *T, f F, conv func(U) (T, error)) flg.Flag {
	set := func(v reflect.Value, u U) error {
		t, err := conv(u)
		if err != nil {
			return err
		}
		v.Set(clone(reflect.ValueOf(&t).Elem()))
		return nil
	}
	return bind(l, dst, f, &binding{
		apply: func(v reflect.Value, _ *resolver) (bool, error) {
			u, _ := f.Get()
			return false, set(v, u)
		},
		applyDefault: func(v reflect.Value, _ *resolver) (bool, error) {
			u, ok := f.GetDefault()
			if !ok {
				return false, nil
			}
			return true, set(v, u)
		},
	})
}

// BindText binds the string flag f to the field dst points to, which reads
// itself from text, such as a Secret: `--key=${file:/run/key}` works as it
// does in the file. An empty value clears the field. See Bind.
func BindText[T any, PT interface {
	*T
	encoding.TextUnmarshaler
}, F TypedFlag[string], C any](l *Loader[C], dst PT, f F) flg.Flag {
	set := func(v reflect.Value, s string, r *resolver) (bool, error) {
		if s == "" {
			v.SetZero()
			return true, nil
		}
		return false, setText(v, s, r)
	}
	return bind(l, (*T)(dst), f, &binding{
		apply: func(v reflect.Value, r *resolver) (bool, error) {
			s, _ := f.Get()
			return set(v, s, r)
		},
		applyDefault: func(v reflect.Value, r *resolver) (bool, error) {
			s, ok := f.GetDefault()
			if !ok {
				return false, nil
			}
			_, err := set(v, s, r)
			return true, err
		},
	})
}

// emptyText reports whether a flag's value is what `--x=` gives: an empty
// string, or a list of one.
func emptyText(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Slice:
		return v.Len() == 1 && v.Index(0).Kind() == reflect.String && v.Index(0).Len() == 0
	}
	return false
}

func bind[T any, U any, F TypedFlag[U], C any](l *Loader[C], dst *T, f F, b *binding) flg.Flag {
	fd, ok := l.schema.findAny(reflect.ValueOf(l.root).Elem(), dst)
	if !ok {
		panic(fmt.Sprintf("cfg: --%s is bound to a pointer that is not a field of the configuration", f.Info().Name))
	}
	b.owner = l
	b.field = fd
	b.name = f.Info().Name
	w := &bound[U]{Flag: f, src: f, b: b}
	b.flag = w
	return w
}

// ConfigName is the name of the flag the Load handler reads the file's path
// from.
const ConfigName = "config"

// ConfigFlag is `--config`, for the root command Load is mounted on.
func ConfigFlag() *flg.String {
	return &flg.String{
		Name:  ConfigName,
		Brief: "the configuration file to read",
	}
}

// Load is a handler that loads the configuration before the command it is
// mounted on, and the commands under it, run. On the root command it loads for
// every command but those in except and the commands under them, such as
// xli.NewCmdCompletion, which need no configuration and should not fail for a
// bad one:
//
//	comp := xli.NewCmdCompletion()
//	root := &xli.Command{
//		Flags:    flg.Flags{cfg.ConfigFlag()},
//		Commands: xli.Commands{serve, comp, cfg.NewCmdConfig(l)},
//		Handler:  xli.Chain(cfg.Load(l, comp), xli.RequireSubcommand()),
//	}
//
// It may as well be mounted on each command that needs the configuration
// instead. It runs once for a command run, whichever of the commands on the
// way it is mounted on.
//
// It reads the file named by --config (none for an empty one), or else the
// first of the default paths that exists; the process environment; and the
// flags bound to l on every command of the path being run, above and below
// it: flags are parsed before any handler runs. Environment variables under
// the prefix that nothing reads, and secret files not there yet, are reported
// to the command's ErrWriter.
//
// A configuration that fails to load fails the command, but for `config` of
// NewCmdConfig, which prints what it read and what is wrong with it; `config
// env` does not load it at all.
//
// It runs when the command runs, also on the way to a subcommand
// (mode.Run|mode.Pass), and not for help or completion.
func Load[T any](l *Loader[T], except ...*xli.Command) xli.Handler {
	return xli.On(mode.Run, func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
		if ctx.Value(loadedKey{l}) != nil {
			// Loaded on the way here.
			return next(ctx)
		}
		ctx = context.WithValue(ctx, loadedKey{l}, true)

		// The commands of the path being run: the ones above this one, it,
		// and the ones below.
		path := cmd.Tree()
		for f := frm.From(ctx); f != nil; f = f.Next() {
			if c, ok := f.Cmd().(*xli.Command); ok && c != cmd {
				path = append(path, c)
			}
		}
		for _, c := range path {
			if slices.Contains(except, c) || l.skips(c) {
				return next(ctx)
			}
		}

		fs := []flg.Flag{}
		for _, c := range path {
			fs = append(fs, c.GetFlags()...)
		}
		bound, err := l.bindings(fs)
		if err != nil {
			return err
		}
		in := &inputs{environ: l.opts.environ(), bound: bound}
		if p, ok := flg.Find[string](cmd, ConfigName); ok {
			in.path, in.noFile = p, p == ""
		}

		s, err := l.load(in)
		if err != nil {
			if !l.tolerates(path[len(path)-1]) {
				return err
			}
			l.failed.Store(&failure[T]{s, err})
			return next(ctx)
		}
		l.failed.Store(nil)

		w := cmd.ErrWriter
		if w == nil {
			w = os.Stderr
		}
		for _, name := range s.Unknown {
			fmt.Fprintf(w, "%s: %s is set and nothing reads it%s\n", l.name, name, hint(name, l.EnvNames()))
		}
		for _, err := range s.Warnings {
			fmt.Fprintf(w, "%s: %s\n", l.name, err)
		}
		return next(ctx)
	})
}

// loadedKey marks a context the Load handler of a loader loaded on.
type loadedKey struct{ l any }

func (l *Loader[T]) skips(c *xli.Command) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Contains(l.skip, c)
}

func (l *Loader[T]) tolerates(c *xli.Command) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Contains(l.tolerant, c)
}
