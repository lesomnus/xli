package xli

import (
	"context"

	"github.com/lesomnus/xli/frm"
)

// RequireSubcommand is a handler for a command that does nothing of its own:
// run with no subcommand, it returns a [UsageError] of [ErrNeedCmd], and
// otherwise it calls next.
func RequireSubcommand() Handler {
	return OnRun(func(ctx context.Context, cmd *Command, next Next) error {
		f := frm.From(ctx)
		if f.Next() == nil {
			return &UsageError{Cmd: cmd, Err: ErrNeedCmd}
		}

		return next(ctx)
	})
}
