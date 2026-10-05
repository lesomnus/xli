package xli

import (
	"context"

	"github.com/lesomnus/xli/frm"
)

func RequireSubcommand() Handler {
	return OnRun(func(ctx context.Context, cmd *Command, next Next) error {
		f := frm.From(ctx)
		if f.Next() == nil {
			return &UsageError{Cmd: cmd, Err: ErrNeedCmd}
		}

		return next(ctx)
	})
}
