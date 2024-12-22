package xli

import (
	"context"
)

type ArgInfo struct {
	Name string

	Brief string
	Synop string
	Usage Stringer
}

type Arg interface {
	Info() *ArgInfo
	Prase(ctx context.Context, cmd *Command, pre []string, rest []string) (context.Context, int, error)
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
