package xli

import (
	"context"

	"github.com/lesomnus/xli/mode"
)

type Next func(ctx context.Context) error

type Action func(ctx context.Context, cmd *Command, next Next) error

var noop Action = func(ctx context.Context, cmd *Command, next Next) error {
	return next(ctx)
}

func chain(cmd *Command, as []Action, next Next) Next {
	switch len(as) {
	case 0:
		return next
	case 1:
		return func(ctx context.Context) error {
			return as[0](ctx, cmd, next)
		}
	default:
		return func(ctx context.Context) error {
			return as[0](ctx, cmd, chain(cmd, as[1:], next))
		}
	}
}

func Chain(as ...Action) Action {
	if len(as) == 0 {
		return noop
	}
	return func(ctx context.Context, cmd *Command, next Next) error {
		return chain(cmd, as, next)(ctx)
	}
}

func OnF(f func(m mode.Mode) bool, a Action) Action {
	return func(ctx context.Context, cmd *Command, next Next) error {
		m := mode.From(ctx)
		if !f(m) {
			return next(ctx)
		}
		return a(ctx, cmd, next)
	}
}

func On(m mode.Mode, a Action) Action {
	return OnF(func(m_ mode.Mode) bool { return m_&m == m }, a)
}

func OnExact(m mode.Mode, a Action) Action {
	return OnF(func(m_ mode.Mode) bool { return m_ == m }, a)
}

func OnHelp(a Action) Action     { return OnExact(mode.Help, a) }
func OnTap(a Action) Action      { return OnExact(mode.Tap, a) }
func OnRun(a Action) Action      { return OnExact(mode.Run, a) }
func OnHelpPass(a Action) Action { return OnExact(mode.Help|mode.Pass, a) }
func OnTapPass(a Action) Action  { return OnExact(mode.Tap|mode.Pass, a) }
func OnRunPass(a Action) Action  { return OnExact(mode.Run|mode.Pass, a) }
