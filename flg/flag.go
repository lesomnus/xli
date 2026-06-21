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
	Usage    fmt.Stringer
	Required bool

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
		if s, ok := v.(interface{ setCategory(string) }); ok {
			s.setCategory(name)
		}
	}
	return append(fs, vs...)
}
