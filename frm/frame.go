// Package frm is the path of a run, as frames: one for each command from the
// root to the one being run, which a handler finds in its context.
//
//	if frm.HasSeq(frm.From(ctx), "app", "db", "migrate") { ... }
package frm

import (
	"context"

	"github.com/lesomnus/xli/xmd"
)

// Frame is one command on the path of a run, with the frames before and after
// it; Prev of the root's frame and Next of the last are nil.
type Frame interface {
	Cmd() xmd.Command
	Prev() Frame
	Next() Frame
}

type ctxKey struct{}

// From is the frame of the command whose handler is given ctx, or nil.
func From(ctx context.Context) Frame {
	v, ok := ctx.Value(ctxKey{}).(Frame)
	if !ok {
		return nil
	}

	return v
}

// Into is ctx carrying v, for [From].
func Into(ctx context.Context, v Frame) context.Context {
	return context.WithValue(ctx, ctxKey{}, v)
}

// HasSeq reports whether the commands from f on are named names, in order: the
// path goes on as names says.
func HasSeq(f Frame, names ...string) bool {
	for _, v := range names {
		if f == nil {
			return false
		}

		c := f.Cmd()
		if c == nil || c.GetName() != v {
			return false
		}

		f = f.Next()
	}
	return true
}
