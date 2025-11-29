package arg

import (
	"context"
	"fmt"
)

type Info struct {
	Name string

	Brief string
	Synop string
	Usage fmt.Stringer

	Handle          func(ctx context.Context)
	TODO_Completion func(ctx context.Context)
}

type Arg interface {
	Info() *Info
	Parse(rest []string) (int, error)

	IsOptional() bool
	IsMany() bool
}

type Args []Arg

func (as Args) Get(name string) Arg {
	for _, a := range as {
		if a.Info().Name == name {
			return a
		}
	}

	return nil
}
