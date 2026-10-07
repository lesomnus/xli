package xli

import (
	"context"

	"github.com/lesomnus/xli/mode"
)

// Next runs what comes after a handler: the command's next handler, if it was
// given several with [Chain], or else the subcommand. The context it is called
// with is the one the rest of the run sees. A handler that does not call it
// ends the run there.
type Next func(ctx context.Context) error

// HandlerFunc is a handler as a function: cmd is the command it is the handler
// of, and next is the rest of the run, which it calls when, and if, the run is
// to go on.
type HandlerFunc func(ctx context.Context, cmd *Command, next Next) error

// Handler is a command's middleware. Run calls the handler of every command on
// the path from the root, each through the next its parent's handler calls;
// see the package documentation.
type Handler interface {
	Handle(ctx context.Context, cmd *Command, next Next) error
}

// Handle is f as a Handler, in every mode. Most handlers want one mode, and
// [OnRun] and the other On functions are those.
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

// Chain is hs as one handler: each is called with a next that calls the one
// after it, and the last with the command's next. A handler in the chain that
// does not call next ends the run there, as one alone would.
func Chain(hs ...Handler) Handler {
	if len(hs) == 0 {
		return noop
	}
	return handler(func(ctx context.Context, cmd *Command, next Next) error {
		return chain(cmd, hs, next)(ctx)
	})
}

// OnF calls a when f reports true for the mode of the run, and next otherwise.
// When a is called, it is a that calls next.
func OnF(f func(m mode.Mode) bool, a HandlerFunc) Handler {
	return handler(func(ctx context.Context, cmd *Command, next Next) error {
		m := mode.From(ctx)
		if !f(m) {
			return next(ctx)
		}
		return a(ctx, cmd, next)
	})
}

// On calls f when the mode of the run has every bit of m set -- On(mode.Run, f)
// in mode.Run and in mode.Run|mode.Pass -- and next otherwise.
func On(m mode.Mode, f HandlerFunc) Handler {
	return OnF(func(m_ mode.Mode) bool { return m_&m == m }, f)
}

// OnExact calls f when the mode of the run is m, and next otherwise.
func OnExact(m mode.Mode, f HandlerFunc) Handler {
	return OnF(func(m_ mode.Mode) bool { return m_ == m }, f)
}

// OnRun calls f when the command is the one the command line names last, and
// so the one being run; on the way to a subcommand, and for help and
// completion, it calls next.
func OnRun(f HandlerFunc) Handler { return OnExact(mode.Run, f) }

// OnRunPass calls f when the command is on the way to the one being run --
// where a parent sets up what its subcommands share -- and next otherwise.
func OnRunPass(f HandlerFunc) Handler { return OnExact(mode.Run|mode.Pass, f) }

// OnHelp calls f when --help is given to the command, and next otherwise.
func OnHelp(f HandlerFunc) Handler { return OnExact(mode.Help, f) }

// OnHelpPass calls f when --help is given to a command below this one, and
// next otherwise.
func OnHelpPass(f HandlerFunc) Handler { return OnExact(mode.Help|mode.Pass, f) }

// OnTab calls f when the shell asks the command for completions, and next
// otherwise; the candidates go to [tab.From] of the context.
func OnTab(f HandlerFunc) Handler { return OnExact(mode.Tab, f) }

// OnTabPass calls f when the shell asks a command below this one for
// completions, and next otherwise.
func OnTabPass(f HandlerFunc) Handler { return OnExact(mode.Tab|mode.Pass, f) }
