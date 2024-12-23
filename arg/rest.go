package arg

import (
	"context"
	"fmt"
)

type RestStrings = Rest[string, StringParser]

type RestInts = Rest[int, IntParser]
type RestInt32s = Rest[int32, Int32Parser]
type RestInt64s = Rest[int64, Int64Parser]

type RestUints = Rest[uint, UintParser]
type RestUint32s = Rest[uint32, Uint32Parser]
type RestUint64s = Rest[uint64, Uint64Parser]

type RestParser[T any, P Parser[T]] struct {
	Base P
}

func (p *RestParser[T, P]) Prase(ctx context.Context, prev []string, rest []string) ([]T, int, error) {
	vs := []T{}
	i := 0
	for i < len(rest) {
		v, n, err := p.Base.Parse(ctx, prev, rest[i:])
		i += n
		if n == 0 || err != nil {
			return nil, i, err
		}

		vs = append(vs, v)
	}
	return vs, i, nil
}

func (p *RestParser[T, P]) String() string {
	return fmt.Sprintf("%s...", p.Base.String())
}

type Rest[T any, P Parser[T]] struct {
	Name string

	Brief string
	Synop string
	Usage fmt.Stringer

	Value  []T
	Action func(ctx context.Context, v []T) (context.Context, error)

	Parser RestParser[T, P]
}

func (a *Rest[T, P]) Info() *Info {
	return &Info{
		Name: a.Name,

		Brief: a.Brief,
		Synop: a.Synop,
		Usage: a.Usage,
	}
}

func (a *Rest[T, P]) Get() ([]T, bool) {
	return a.Value, len(a.Value) > 0
}

func (a *Rest[T, P]) IsOptional() bool {
	// Rest implies optional.
	return true
}

func (a *Rest[T, P]) Prase(ctx context.Context, prev []string, rest []string) (context.Context, int, error) {
	vs, n, err := a.Parser.Prase(ctx, prev, rest)
	if n == 0 || err != nil {
		return ctx, n, err
	}

	a.Value = vs

	if a.Action != nil {
		ctx_, err := a.Action(ctx, vs)
		if ctx_ != nil {
			ctx = ctx_
		}
		return ctx, n, err
	}
	return ctx, n, nil
}