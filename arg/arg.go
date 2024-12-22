package arg

import (
	"context"

	"github.com/lesomnus/xli"
)

type Parser[T any] interface {
	Prase(ctx context.Context, cmd *xli.Command, prev []string, rest []string) (T, int, error)
}

type Arg[T any, P Parser[T]] struct {
	Name string

	Brief string
	Synop string
	Usage xli.Stringer

	Value  *T
	Action func(ctx context.Context, cmd *xli.Command, v T) (context.Context, error)

	Parser P
}

func (a *Arg[T, P]) Info() *xli.ArgInfo {
	return &xli.ArgInfo{
		Name: a.Name,

		Brief: a.Brief,
		Synop: a.Synop,
		Usage: a.Usage,
	}
}

func (a Arg[T, P]) Get() *T {
	return a.Value
}

func (a *Arg[T, P]) Prase(ctx context.Context, cmd *xli.Command, prev []string, rest []string) (context.Context, int, error) {
	v, n, err := a.Parser.Prase(ctx, cmd, prev, rest)
	if n == 0 || err != nil {
		return ctx, n, err
	}

	if a.Value == nil {
		a.Value = &v
	} else {
		*a.Value = v
	}
	if a.Action != nil {
		ctx, err = a.Action(ctx, cmd, v)
	}
	return ctx, n, err
}
