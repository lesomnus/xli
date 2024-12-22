package xli

import "context"

type Stringer interface {
	String(ctx context.Context, cmd *Command) string
}

type S string

func (s S) String(ctx context.Context) string {
	return string(s)
}

type D func(ctx context.Context, cmd *Command) string

func (d D) String(ctx context.Context, cmd *Command) string {
	return d(ctx, cmd)
}
