package arg

import (
	"context"
	"fmt"
)

type Parser[T any] interface {
	Parse(ctx context.Context, rest []string) (T, int, error)
	String() string
}

type Base[T any, P Parser[T]] struct {
	Name string

	Brief string
	Synop string
	Usage fmt.Stringer

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
	return &Info{
		Name: a.Name,

		Brief: a.Brief,
		Synop: a.Synop,
		Usage: usage,
	}
}

func (a *Base[T, P]) Get() (T, bool) {
	if a.Value == nil {
		var z T
		return z, false
	}
	return *a.Value, true
}

func (a *Base[T, P]) IsOptional() bool {
	return a.Optional
}

func (a *Base[T, P]) IsMany() bool {
	return false
}

func (a *Base[T, P]) Prase(ctx context.Context, rest []string) (int, error) {
	v, n, err := a.Parser.Parse(ctx, rest)
	if n == 0 || err != nil {
		return n, err
	}

	if a.Value == nil {
		a.Value = &v
	} else {
		*a.Value = v
	}
	if a.Handler != nil {
		if err := a.Handler.Handle(ctx, v); err != nil {
			return n, err
		}
	}
	return n, nil
}
