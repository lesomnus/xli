package arg

import (
	"context"
	"fmt"

	"github.com/lesomnus/xli/mode"
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
	if m := mode.From(ctx); m == mode.Tab {
		var z T
		a.handle(ctx, z)
		return 0, nil
	}

	v, n, err := a.Parser.Parse(ctx, rest)
	if n == 0 || err != nil {
		return n, err
	}

	if a.Value == nil {
		a.Value = &v
	} else {
		*a.Value = v
	}
	return n, a.handle(ctx, v)
}

func (a *Base[T, P]) handle(ctx context.Context, v T) error {
	if h := a.Handler; h != nil {
		return h.Handle(ctx, v)
	}
	return nil
}
