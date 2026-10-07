package flg

import (
	"context"
	"fmt"
)

type Info struct {
	Category string
	Name     string
	Alias    rune

	Type     string
	Brief    string
	Synop    string
	Required bool

	// Hidden reports that the flag is omitted from help and completion; it is
	// still accepted on the command line.
	Hidden bool

	// Env names an environment variable the flag's value can also be given
	// by, for help and generated documentation. The flag machinery does not
	// read it; it is set by whatever does (e.g. the cfg package).
	Env string

	// Default is the string form of the flag's default value, for help
	// rendering. HasDefault is false when the flag has no default.
	Default    string
	HasDefault bool
}

func (i *Info) String() string {
	if i.Alias == 0 {
		return fmt.Sprintf("   --%s %s", i.Name, i.Type)
	} else {
		return fmt.Sprintf("-%c,--%s %s", i.Alias, i.Name, i.Type)
	}
}

type Flag interface {
	Info() *Info
	Handle(ctx context.Context, v string) error

	Count() int

	// NoValue reports whether the flag is a switch that does not consume a
	// value (e.g. a boolean switch). Such flags default to "true" when given
	// without an explicit value.
	NoValue() bool
}

type Flags []Flag

// Visible returns the flags that are not hidden.
func (fs Flags) Visible() Flags {
	vs := Flags{}
	for _, f := range fs {
		if !f.Info().Hidden {
			vs = append(vs, f)
		}
	}
	return vs
}

func (fs Flags) Get(name string) Flag {
	for _, f := range fs {
		if f.Info().Name == name {
			return f
		}
	}

	return nil
}

func (fs Flags) GetByAlias(c rune) Flag {
	for _, f := range fs {
		if f.Info().Alias == c {
			return f
		}
	}

	return nil
}

func (fs Flags) ByCategory() []Flags {
	i := map[string]int{}
	vs := []Flags{}
	for _, f := range fs {
		j, ok := i[f.Info().Category]
		if !ok {
			j = len(vs)
			i[f.Info().Category] = j
			vs = append(vs, Flags{})
		}

		vs[j] = append(vs[j], f)
	}
	return vs
}

func (fs Flags) WithCategory(name string, vs ...Flag) Flags {
	for _, v := range vs {
		for f := v; f != nil; f = Unwrap(f) {
			if s, ok := f.(interface{ setCategory(string) }); ok {
				s.setCategory(name)
				break
			}
		}
	}
	return append(fs, vs...)
}

// Unwrap returns the flag f wraps, or nil if f does not wrap one. A flag that
// decorates another (e.g. to bind it to a configuration field) implements
// `Unwrap() Flag`, which lets the flg package reach the flag underneath.
func Unwrap(f Flag) Flag {
	if u, ok := f.(interface{ Unwrap() Flag }); ok {
		return u.Unwrap()
	}
	return nil
}
