package tab

import "context"

type Tab interface {
	Value(v string)
	ValueD(v string, desc string)
}

type ctxKey struct{}

func From(ctx context.Context) Tab {
	v, ok := ctx.Value(ctxKey{}).(Tab)
	if !ok {
		return nil
	}

	return v
}

func Into(ctx context.Context, v Tab) context.Context {
	return context.WithValue(ctx, ctxKey{}, v)
}
