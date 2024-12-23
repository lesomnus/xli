package arg

import (
	"context"
	"fmt"
)

type Parser[T any] interface {
	Parse(ctx context.Context, prev []string, rest []string) (T, int, error)
	String() string
}

type Base[T any, P Parser[T]] struct {
	Name string

	Brief string
	Synop string
	Usage fmt.Stringer

	Value  *T
	Action func(ctx context.Context, v T) (context.Context, error)

	Optional bool

	Parser P
}

func (a *Base[T, P]) Info() *Info {
	return &Info{
		Name: a.Name,

		Brief: a.Brief,
		Synop: a.Synop,
		Usage: a.Usage,
	}
}

func (a Base[T, P]) Get() (T, bool) {
	if a.Value == nil {
		var z T
		return z, false
	}
	return *a.Value, true
}

func (a Base[T, P]) IsOptional() bool {
	return a.Optional
}

func (a Base[T, P]) UnderlyingParser() any {
	return a.Parser
}

func (a *Base[T, P]) Prase(ctx context.Context, prev []string, rest []string) (context.Context, int, error) {
	v, n, err := a.Parser.Parse(ctx, prev, rest)
	if n == 0 || err != nil {
		return ctx, n, err
	}

	if a.Value == nil {
		a.Value = &v
	} else {
		*a.Value = v
	}
	if a.Action != nil {
		ctx, err = a.Action(ctx, v)
	}
	return ctx, n, err
}
