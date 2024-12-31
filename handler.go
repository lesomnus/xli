package xli

import (
	"context"

	"github.com/lesomnus/xli/mode"
)

type Next func(ctx context.Context) error

type HandlerFunc func(ctx context.Context, cmd *Command, next Next) error

type Handler interface {
	Handle(ctx context.Context, cmd *Command, next Next) error
}

func Handle(f HandlerFunc) Handler {
	return handler(f)
}

type handler HandlerFunc

func (h handler) Handle(ctx context.Context, cmd *Command, next Next) error {
	return h(ctx, cmd, next)
}

var noop Handler = handler(func(ctx context.Context, cmd *Command, next Next) error {
	return next(ctx)
})

func chain(cmd *Command, hs []Handler, next Next) Next {
	switch len(hs) {
	case 0:
		return next
	case 1:
		return func(ctx context.Context) error {
			return hs[0].Handle(ctx, cmd, next)
		}
	default:
		return func(ctx context.Context) error {
			return hs[0].Handle(ctx, cmd, chain(cmd, hs[1:], next))
		}
	}
}

func Chain(hs ...Handler) Handler {
	if len(hs) == 0 {
		return noop
	}
	return handler(func(ctx context.Context, cmd *Command, next Next) error {
		return chain(cmd, hs, next)(ctx)
	})
}

func OnF(f func(m mode.Mode) bool, a HandlerFunc) Handler {
	return handler(func(ctx context.Context, cmd *Command, next Next) error {
		m := mode.From(ctx)
		if !f(m) {
			return next(ctx)
		}
		return a(ctx, cmd, next)
	})
}

func On(m mode.Mode, f HandlerFunc) Handler {
	return OnF(func(m_ mode.Mode) bool { return m_&m == m }, f)
}

func OnExact(m mode.Mode, f HandlerFunc) Handler {
	return OnF(func(m_ mode.Mode) bool { return m_ == m }, f)
}

func OnHelp(f HandlerFunc) Handler     { return OnExact(mode.Help, f) }
func OnTap(f HandlerFunc) Handler      { return OnExact(mode.Tab, f) }
func OnRun(f HandlerFunc) Handler      { return OnExact(mode.Run, f) }
func OnHelpPass(f HandlerFunc) Handler { return OnExact(mode.Help|mode.Pass, f) }
func OnTapPass(f HandlerFunc) Handler  { return OnExact(mode.Tab|mode.Pass, f) }
func OnRunPass(f HandlerFunc) Handler  { return OnExact(mode.Run|mode.Pass, f) }
