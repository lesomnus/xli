package arg

import (
	"context"
	"fmt"
)

// Info is what an argument says of itself, for parsing, help and
// documentation.
type Info struct {
	Name string

	Brief string
	Synop string
	Usage fmt.Stringer

	// Default is the string form of the argument's default value, for help
	// rendering. HasDefault is false when there is no default.
	Default    string
	HasDefault bool

	// Handle calls the argument's handler with no value, for completion: a
	// value being completed has not been given yet.
	Handle func(ctx context.Context)
}

// Arg is an argument of a command. The types of this package are Args; a type
// of one's own is too, given these methods.
type Arg interface {
	Info() *Info
	// Parse reads the argument from rest, the words of the command line from
	// its position on, and reports how many it took.
	Parse(rest []string) (int, error)

	// IsOptional reports whether the argument may be left out, and IsMany
	// whether it takes every word that is left: a variadic argument, which
	// the usage line writes as [NAME...].
	IsOptional() bool
	IsMany() bool
}

// Args are the arguments of one command, in order.
type Args []Arg

// Get is the argument called name, or nil.
func (as Args) Get(name string) Arg {
	for _, a := range as {
		if a.Info().Name == name {
			return a
		}
	}

	return nil
}
