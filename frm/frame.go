package frm

import (
	"context"

	"github.com/lesomnus/xli/xmd"
)

type Frame interface {
	Cmd() xmd.Command
	Prev() Frame
	Next() Frame
}

type ctxKey struct{}

func From(ctx context.Context) Frame {
	v, ok := ctx.Value(ctxKey{}).(Frame)
	if !ok {
		return nil
	}

	return v
}

func Into(ctx context.Context, v Frame) context.Context {
	return context.WithValue(ctx, ctxKey{}, v)
}

func HasSequence(f Frame, names ...string) bool {
	for _, v := range names {
		c := f.Cmd()
		if c == nil || c.GetName() != v {
			return false
		}

		f = f.Next()
	}
	return true
}
