package flag

import (
	"context"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/internal"
)

type Parser[T any] interface {
	Parse(v string) (T, error)
	ToString(v T) string
}

type Flag[T any, P Parser[T]] struct {
	Name    string
	Aliases []rune
	Brief   string
	Synop   string

	Value  *T
	Action func(ctx context.Context, cmd *xli.Command, v T) (context.Context, error)

	Parser P

	internal.FlagTag[T]
}

func (f *Flag[T, P]) Info() *xli.FlagInfo {
	return &xli.FlagInfo{
		Name:    f.Name,
		Aliases: f.Aliases,
		Brief:   f.Brief,
		Synop:   f.Synop,
	}
}

func (f *Flag[T, P]) Handle(ctx context.Context, cmd *xli.Command, v string) (context.Context, error) {
	w, err := f.Parser.Parse(v)
	if err != nil {
		return ctx, err
	}

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

func (f *Flag[T, P]) Default() (string, bool) {
	if f.Value == nil {
		return "", false
	}

	return f.Parser.ToString(*f.Value), true
}
