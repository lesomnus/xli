package xli

import (
	"context"
)

type ctxKey struct{}

func From(ctx context.Context) *Command {
	v, ok := ctx.Value(ctxKey{}).(*Command)
	if !ok {
		return &Command{}
	}

	return v
}

func Into(ctx context.Context, cmd *Command) context.Context {
	return context.WithValue(ctx, ctxKey{}, cmd)
}
