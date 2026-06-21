package frm_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/frm"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/xmd"
)

type fakeCmd struct {
	name string
}

func (c fakeCmd) GetName() string     { return c.name }
func (c fakeCmd) GetFlags() flg.Flags { return nil }
func (c fakeCmd) GetArgs() arg.Args   { return nil }

type fakeFrame struct {
	cmd  xmd.Command
	next frm.Frame
}

func (f *fakeFrame) Cmd() xmd.Command { return f.cmd }
func (f *fakeFrame) Prev() frm.Frame  { return nil }
func (f *fakeFrame) Next() frm.Frame {
	if f.next == nil {
		return nil
	}
	return f.next
}

// chain builds a frame sequence from the given names, root first.
func chain(names ...string) frm.Frame {
	var head *fakeFrame
	for i := len(names) - 1; i >= 0; i-- {
		f := &fakeFrame{cmd: fakeCmd{name: names[i]}}
		if head != nil {
			f.next = head
		}
		head = f
	}
	return head
}

func TestFrom(t *testing.T) {
	t.Run("returns nil when absent", x.F(func(x x.X) {
		x.Nil(frm.From(context.Background()))
	}))
	t.Run("Into and From round-trip", x.F(func(x x.X) {
		f := chain("root")
		ctx := frm.Into(context.Background(), f)
		x.Same(f, frm.From(ctx))
	}))
}

func TestHasSeq(t *testing.T) {
	t.Run("matches a prefix of the chain", x.F(func(x x.X) {
		f := chain("root", "foo", "bar")
		x.True(frm.HasSeq(f, "root", "foo"))
		x.True(frm.HasSeq(f, "root", "foo", "bar"))
	}))
	t.Run("returns false on a name mismatch", x.F(func(x x.X) {
		f := chain("root", "foo", "bar")
		x.False(frm.HasSeq(f, "root", "baz"))
		x.False(frm.HasSeq(f, "nope"))
	}))
	t.Run("returns false when names exceed the chain", x.F(func(x x.X) {
		f := chain("root", "foo")
		x.False(frm.HasSeq(f, "root", "foo", "bar"))
	}))
}
