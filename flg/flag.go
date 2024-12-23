package flg

import (
	"context"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/internal"
)

type Parser[T any] interface {
	Parse(s string) (T, error)
	ToString(v T) string
	String() string
}

type Flag[T any, P Parser[T]] struct {
	Name  string
	Alias rune
	Brief string
	Synop string

	Value  *T
	Action func(ctx context.Context, cmd *xli.Command, v T) (context.Context, error)

	Parser P

	count int
	internal.FlagTag[T]
}

func (f *Flag[T, P]) Info() *xli.FlagInfo {
	return &xli.FlagInfo{
		Name:  f.Name,
		Alias: f.Alias,
		Brief: f.Brief,
		Synop: f.Synop,

		Type: f.Parser.String(),
	}
}

func (f *Flag[T, P]) Handle(ctx context.Context, cmd *xli.Command, v string) (context.Context, error) {
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
		return a(ctx, cmd, w)
	}
	return ctx, nil
}

func (f *Flag[T, P]) Count() int {
	return f.count
}

func (f *Flag[T, P]) Default() (string, bool) {
	if f.Value == nil {
		return "", false
	}

	return f.Parser.ToString(*f.Value), true
}

func (f *Flag[T, P]) Get() (T, bool) {
	if f.Value == nil {
		var z T
		return z, false
	}
	return *f.Value, true
}

func Visit[T any](c *xli.Command, name string, visitor func(v T)) bool {
	f := c.Flags.Get(name)
	if f == nil {
		return false
	}

	g, ok := f.(interface{ Get() (T, bool) })
	if !ok {
		return false
	}

	v, ok := g.Get()
	if !ok {
		return false
	}

	visitor(v)
	return true
}

func VisitP[T any](c *xli.Command, name string, dst *T) bool {
	if dst == nil {
		return false
	}
	return Visit(c, name, func(v T) {
		*dst = v
	})
}
