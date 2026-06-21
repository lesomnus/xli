package arg

import (
	"context"
	"fmt"

	"github.com/lesomnus/xli/mode"
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

func (p *RestParser[T, P]) Parse(rest []string) ([]T, int, error) {
	vs := []T{}
	i := 0
	for i < len(rest) {
		v, n, err := p.Base.Parse(rest[i:])
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

	// Default is the value used when no values are provided. It is never
	// modified by the framework; a nil Default means there is no default.
	Default []T

	// Value holds the values parsed from the command line.
	Value   []T
	Handler Handler[[]T]

	Parser RestParser[T, P]
}

func (a *Rest[T, P]) String() string {
	return fmt.Sprintf("[%s...]", a.Name)
}

func (a *Rest[T, P]) Info() *Info {
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
				// OnTab handler can emit suggestions even before values
				// have been parsed.
				var z []T
				a.Handler.Handle(ctx, z)
				return
			}
			if a.Value != nil {
				a.Handler.Handle(ctx, a.Value)
			}
		},
	}
	if a.Default != nil {
		info.Default = fmt.Sprintf("%v", a.Default)
		info.HasDefault = true
	}
	return info
}

func (a *Rest[T, P]) Get() ([]T, bool) {
	return a.Value, len(a.Value) > 0
}

// lookupDefault returns the configured default values, if any.
func (a *Rest[T, P]) lookupDefault() ([]T, bool) {
	if a.Default == nil {
		return nil, false
	}
	return a.Default, true
}

func (a *Rest[T, P]) IsOptional() bool {
	// Rest implies optional.
	return true
}

func (a *Rest[T, P]) IsMany() bool {
	return true
}

func (a *Rest[T, P]) Parse(rest []string) (int, error) {
	vs, n, err := a.Parser.Parse(rest)
	if n == 0 || err != nil {
		return n, err
	}

	a.Value = vs
	return n, nil
}
