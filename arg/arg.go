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

	// Default is the string form of the argument's default value, for help
	// rendering. HasDefault is false when there is no default.
	Default    string
	HasDefault bool

	Handle func(ctx context.Context)
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
