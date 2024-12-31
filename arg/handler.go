package arg

import (
	"context"

	"github.com/lesomnus/xli/mode"
	"github.com/lesomnus/xli/tab"
)

type HandlerFunc[T any] func(ctx context.Context, v T) error

type Handler[T any] interface {
	Handle(ctx context.Context, v T) error
}

func Handle[T any](f HandlerFunc[T]) Handler[T] {
	return handler[T](f)
}

type handler[T any] HandlerFunc[T]

func (h handler[T]) Handle(ctx context.Context, v T) error {
	return h(ctx, v)
}

func Wrap[T any](hs ...Handler[T]) Handler[T] {
	return handler[T](func(ctx context.Context, v T) error {
		for _, h := range hs {
			if err := h.Handle(ctx, v); err != nil {
				return err
			}
		}
		return nil
	})
}

func OnF[T any](f func(m mode.Mode) bool, a HandlerFunc[T]) Handler[T] {
	return handler[T](func(ctx context.Context, v T) error {
		m := mode.From(ctx)
		if !f(m) {
			return nil
		}
		return a(ctx, v)
	})
}

func On[T any](m mode.Mode, f HandlerFunc[T]) Handler[T] {
	return OnF(func(m_ mode.Mode) bool { return m_&m == m }, f)
}

func OnExact[T any](m mode.Mode, f HandlerFunc[T]) Handler[T] {
	return OnF(func(m_ mode.Mode) bool { return m_ == m }, f)
}

func OnHelp[T any](f HandlerFunc[T]) Handler[T]     { return OnExact(mode.Help, f) }
func OnRun[T any](f HandlerFunc[T]) Handler[T]      { return OnExact(mode.Run, f) }
func OnHelpPass[T any](f HandlerFunc[T]) Handler[T] { return OnExact(mode.Help|mode.Pass, f) }
func OnTapPass[T any](f HandlerFunc[T]) Handler[T]  { return OnExact(mode.Tab|mode.Pass, f) }
func OnRunPass[T any](f HandlerFunc[T]) Handler[T]  { return OnExact(mode.Run|mode.Pass, f) }

type TabHandlerFunc[T any] func(ctx context.Context, tab tab.Tab)

func OnTap[T any](f TabHandlerFunc[T]) Handler[T] {
	return OnExact(mode.Tab, func(ctx context.Context, v T) error {
		t := tab.From(ctx)
		if t != nil {
			f(ctx, t)
		}

		return nil
	})
}
