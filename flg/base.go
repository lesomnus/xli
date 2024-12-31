package flg

import (
	"context"
	"fmt"

	"github.com/lesomnus/xli/mode"
)

type Parser[T any] interface {
	Parse(s string) (T, error)
	ToString(v T) string
	String() string
}

type Base[T any, P Parser[T]] struct {
	Name  string
	Alias rune

	Brief string
	Synop string
	Usage fmt.Stringer

	Value   *T
	Handler Handler[T]

	Parser P

	count int
}

func (f *Base[T, P]) Info() *Info {
	return &Info{
		Name:  f.Name,
		Alias: f.Alias,

		Type:  f.Parser.String(),
		Brief: f.Brief,
		Synop: f.Synop,
		Usage: f.Usage,
	}
}

func (f *Base[T, P]) Default() (string, bool) {
	if f.Value == nil {
		return "", false
	}

	return f.Parser.ToString(*f.Value), true
}

func (f *Base[T, P]) Get() (T, bool) {
	if f.Value == nil {
		var z T
		return z, false
	}
	return *f.Value, true
}

func (f *Base[T, P]) Handle(ctx context.Context, u string) error {
	if m := mode.From(ctx); m == mode.Tab {
		var z T
		f.handle(ctx, z)
		return nil
	}

	v, err := f.Parser.Parse(u)
	if err != nil {
		return err
	}

	f.count++
	if f.Value == nil {
		f.Value = &v
	} else {
		*f.Value = v
	}
	return f.handle(ctx, v)
}

func (f *Base[T, P]) Count() int {
	return f.count
}

func (a *Base[T, P]) handle(ctx context.Context, v T) error {
	if h := a.Handler; h != nil {
		return h.Handle(ctx, v)
	}
	return nil
}
