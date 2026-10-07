package xli

import (
	"context"
)

type ctxKey struct{}

// From is the command ctx carries, or an empty Command if it carries none.
func From(ctx context.Context) *Command {
	v, ok := ctx.Value(ctxKey{}).(*Command)
	if !ok {
		return &Command{}
	}

	return v
}

// Into is ctx carrying cmd, for [From].
func Into(ctx context.Context, cmd *Command) context.Context {
	return context.WithValue(ctx, ctxKey{}, cmd)
}
