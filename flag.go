package xli

import (
	"context"
	"fmt"
)

type FlagInfo struct {
	Category string
	Name     string
	Alias    rune

	Type string

	Brief string
	Synop string
	Usage Stringer
}

func (i *FlagInfo) String() string {
	if i.Alias == 0 {
		return fmt.Sprintf("   --%s %s", i.Name, i.Type)
	} else {
		return fmt.Sprintf("-%c,--%s %s", i.Alias, i.Name, i.Type)
	}
}

type Flag interface {
	Info() *FlagInfo
	Handle(ctx context.Context, cmd *Command, v string) (context.Context, error)

	Count() int
	Default() (string, bool)
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
		v.Info().Category = name
	}
	return append(fs, vs...)
}
