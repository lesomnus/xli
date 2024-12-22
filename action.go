package xli

import (
	"context"
	"errors"

	"github.com/lesomnus/xli/mode"
)

type Action func(ctx context.Context, cmd *Command) (context.Context, error)

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

func On(m mode.Mode, a Action) Action {
	return func(ctx context.Context, cmd *Command) (context.Context, error) {
		if mode.From(ctx) != m {
			return ctx, nil
		}
		return a(ctx, cmd)
	}
}

func OnHelp(a Action) Action { return On(mode.Help, a) }
func OnPass(a Action) Action { return On(mode.Pass, a) }
func OnRun(a Action) Action  { return On(mode.Run, a) }
func OnTap(a Action) Action  { return On(mode.Tap, a) }
