package arg

import (
	"context"
	"fmt"

	"github.com/lesomnus/xli/mode"
)

type Parser[T any] interface {
	Parse(rest []string) (T, int, error)
	String() string
}

type Base[T any, P Parser[T]] struct {
	Name string

	Brief string
	Synop string
	Usage fmt.Stringer

	// Default is the value used when the argument is omitted (only meaningful
	// for an optional argument). It is never modified by the framework; a nil
	// Default means there is no default.
	Default *T

	// Value holds the value parsed from the command line; it is nil until the
	// argument is provided. Read it via Get/MustGet rather than directly.
	Value   *T
	Handler Handler[T]

	Optional bool

	Parser P
}

func (a *Base[T, P]) String() string {
	if a.Optional {
		return fmt.Sprintf("[%s]", a.Name)
	} else {
		return fmt.Sprintf("<%s>", a.Name)
	}
}

func (a *Base[T, P]) Info() *Info {
	usage := a.Usage
	if usage == nil {
		usage = a
	}
	info := &Info{
		Name: a.Name,

		Brief: a.Brief,
		Synop: a.Synop,
		Usage: usage,

		Handle: func(ctx context.Context) {
			if a.Handler == nil {
				return
			}
			if mode.From(ctx) == mode.Tab {
				// Completion: invoke the handler with a zero value so an
				// OnTab handler can emit suggestions even before a value
				// has been parsed.
				var z T
				a.Handler.Handle(ctx, z)
				return
			}
			if a.Value != nil {
				a.Handler.Handle(ctx, *a.Value)
			}
		},
	}
	if a.Default != nil {
		info.Default = fmt.Sprintf("%v", *a.Default)
		info.HasDefault = true
	}
	return info
}

// Get returns the value parsed from the command line and whether the argument
// was provided. It does not consider Default; use MustGet for the effective
// value.
func (a *Base[T, P]) Get() (T, bool) {
	if a.Value == nil {
		var z T
		return z, false
	}
	return *a.Value, true
}

// lookupDefault returns the configured default value, if any.
func (a *Base[T, P]) lookupDefault() (T, bool) {
	if a.Default == nil {
		var z T
		return z, false
	}
	return *a.Default, true
}

func (a *Base[T, P]) IsOptional() bool {
	return a.Optional
}

func (a *Base[T, P]) IsMany() bool {
	return false
}

func (a *Base[T, P]) Parse(rest []string) (int, error) {
	v, n, err := a.Parser.Parse(rest)
	if n == 0 || err != nil {
		return n, err
	}

	a.Value = &v
	return n, nil
}
