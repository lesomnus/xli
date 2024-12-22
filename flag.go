package xli

import (
	"context"
)

type FlagInfo struct {
	Category string
	Name     string
	Aliases  []rune

	Brief string
	Synop string
	Usage Stringer
}

type Flag interface {
	Info() *FlagInfo
	Handle(ctx context.Context, cmd *Command, v string) (context.Context, error)

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
