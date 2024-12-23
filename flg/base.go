package flg

import (
	"context"
	"fmt"
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

	Value  *T
	Action func(ctx context.Context, v T) (context.Context, error)

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

func (f *Base[T, P]) UnderlyingParser() any {
	return f.Parser
}

func (f *Base[T, P]) Handle(ctx context.Context, v string) (context.Context, error) {
	w, err := f.Parser.Parse(v)
	if err != nil {
		return ctx, err
	}

	f.count++
	if f.Value == nil {
		f.Value = &w
	} else {
		*f.Value = w
	}
	if a := f.Action; a != nil {
		return a(ctx, w)
	}
	return ctx, nil
}

func (f *Base[T, P]) Count() int {
	return f.count
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
