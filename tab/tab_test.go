package tab_test

import (
	"context"
	"strings"
	"testing"

	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/tab"
)

func TestZshTab(t *testing.T) {
	t.Run("Value writes an ungrouped entry", x.F(func(x x.X) {
		b := &strings.Builder{}
		z := tab.NewZshTab(b)
		z.Value("foo")
		x.Equal("\x1ffoo\n", b.String())
	}))
	t.Run("ValueD writes the value and description", x.F(func(x x.X) {
		b := &strings.Builder{}
		z := tab.NewZshTab(b)
		z.ValueD("foo", "the foo")
		x.Equal("\x1ffoo:the foo\n", b.String())
	}))
	t.Run("Group prefixes entries with the group name", x.F(func(x x.X) {
		b := &strings.Builder{}
		z := tab.NewZshTab(b)
		g := z.Group("net")
		g.Value("host")
		g.ValueD("port", "the port")
		x.Equal("net\x1fhost\nnet\x1fport:the port\n", b.String())
	}))
}

func TestTabContext(t *testing.T) {
	t.Run("From returns nil when absent", x.F(func(x x.X) {
		x.Nil(tab.From(context.Background()))
	}))
	t.Run("Into and From round-trip", x.F(func(x x.X) {
		z := tab.NewZshTab(&strings.Builder{})
		ctx := tab.Into(context.Background(), z)
		x.Same(z, tab.From(ctx))
	}))
}
