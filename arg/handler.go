package arg

import (
	"context"

	"github.com/lesomnus/xli/mode"
	"github.com/lesomnus/xli/tab"
)

// HandlerFunc is a handler as a function, called with the value of the argument.
type HandlerFunc[T any] func(ctx context.Context, v T) error

// Handler is called with the value of the argument as the line is parsed. It is not
// middleware: there is no next, and an error is the argument's value being
// refused, reported as a usage error.
type Handler[T any] interface {
	Handle(ctx context.Context, v T) error
}

// Handle is f as a Handler, in every mode.
func Handle[T any](f HandlerFunc[T]) Handler[T] {
	return handler[T](f)
}

type handler[T any] HandlerFunc[T]

func (h handler[T]) Handle(ctx context.Context, v T) error {
	return h(ctx, v)
}

// Wrap is hs as one handler, called in order until one returns an error.
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

// OnF calls a when f reports true for the mode, and does nothing otherwise.
func OnF[T any](f func(m mode.Mode) bool, a HandlerFunc[T]) Handler[T] {
	return handler[T](func(ctx context.Context, v T) error {
		m := mode.From(ctx)
		if !f(m) {
			return nil
		}
		return a(ctx, v)
	})
}

// On calls f when the mode has every bit of m set.
func On[T any](m mode.Mode, f HandlerFunc[T]) Handler[T] {
	return OnF(func(m_ mode.Mode) bool { return m_&m == m }, f)
}

// OnExact calls f when the mode is m.
func OnExact[T any](m mode.Mode, f HandlerFunc[T]) Handler[T] {
	return OnF(func(m_ mode.Mode) bool { return m_ == m }, f)
}

// OnRun calls f when the argument's command is the one run.
func OnRun[T any](f HandlerFunc[T]) Handler[T] { return OnExact(mode.Run, f) }

// OnRunPass calls f when the argument's command is on the way to the one run.
func OnRunPass[T any](f HandlerFunc[T]) Handler[T] { return OnExact(mode.Run|mode.Pass, f) }

// OnHelp calls f when --help is given to the argument's command.
func OnHelp[T any](f HandlerFunc[T]) Handler[T] { return OnExact(mode.Help, f) }

// OnHelpPass calls f when --help is given to a command below the argument's.
func OnHelpPass[T any](f HandlerFunc[T]) Handler[T] { return OnExact(mode.Help|mode.Pass, f) }

// OnTabPass calls f when the shell asks a command below the argument's for
// completions.
func OnTabPass[T any](f HandlerFunc[T]) Handler[T] { return OnExact(mode.Tab|mode.Pass, f) }

// TabHandlerFunc offers completion candidates for the argument's value to tab.
type TabHandlerFunc[T any] func(ctx context.Context, tab tab.Tab) error

// OnTab calls f with the context's [tab.Tab] when the shell asks for a value
// of the argument.
func OnTab[T any](f TabHandlerFunc[T]) Handler[T] {
	return OnExact(mode.Tab, func(ctx context.Context, v T) error {
		t := tab.From(ctx)
		if t == nil {
			return nil
		}

		return f(ctx, t)
	})
}
