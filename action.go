package xli

import (
	"context"
	"errors"

	"github.com/lesomnus/xli/mode"
)

type Action func(ctx context.Context, cmd *Command) (context.Context, error)

var noop Action = func(ctx context.Context, cmd *Command) (context.Context, error) {
	return ctx, nil
}

func Chain(as ...Action) Action {
	return func(ctx context.Context, cmd *Command) (context.Context, error) {
		for _, a := range as {
			ctx_, err := a(ctx, cmd)
			if ctx != nil {
				ctx = ctx_
			}
			if err != nil {
				return ctx, err
			}
		}
		return ctx, nil
	}
}

func Join(as ...Action) Action {
	return func(ctx context.Context, cmd *Command) (context.Context, error) {
		errs := make([]error, len(as))
		for i, a := range as {
			ctx_, err := a(ctx, cmd)
			if ctx_ != nil {
				ctx = ctx_
			}
			errs[i] = err
		}

		return ctx, errors.Join(errs...)
	}
}

func OnF(f func(m mode.Mode) bool, a Action) Action {
	return func(ctx context.Context, cmd *Command) (context.Context, error) {
		m := mode.From(ctx)
		if !f(m) {
			return ctx, nil
		}
		return a(ctx, cmd)
	}
}

func On(m mode.Mode, a Action) Action {
	return OnF(func(m_ mode.Mode) bool { return m_&m == m }, a)
}

func OnExact(m mode.Mode, a Action) Action {
	return OnF(func(m_ mode.Mode) bool { return m_ == m }, a)
}

func OnPass(a Action) Action { return OnExact(mode.Pass|mode.Run, a) }
func OnHelp(a Action) Action { return OnExact(mode.Help, a) }
func OnTap(a Action) Action  { return OnExact(mode.Tap, a) }
func OnRun(a Action) Action  { return OnExact(mode.Run, a) }
