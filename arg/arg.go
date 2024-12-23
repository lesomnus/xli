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
}

type Arg interface {
	Info() *Info
	Prase(ctx context.Context, pre []string, rest []string) (context.Context, int, error)
	IsOptional() bool
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
