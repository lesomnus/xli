package xli

import (
	"context"
	"errors"

	"github.com/lesomnus/xli/frm"
)

func RequireSubcommand() Handler {
	return OnRun(func(ctx context.Context, cmd *Command, next Next) error {
		f := frm.From(ctx)
		if f.Next() == nil {
			return errors.New("subcommand is required")
		}

		return next(ctx)
	})
}
