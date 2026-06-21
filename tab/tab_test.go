package tab_test

import (
	"context"
	"strings"
	"testing"

	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/tab"
)

func TestZshTab(t *testing.T) {
	t.Run("Value writes the value with a newline", x.F(func(x x.X) {
		b := &strings.Builder{}
		z := tab.NewZshTab(b)
		z.Value("foo")
		x.Equal("foo\n", b.String())
	}))
	t.Run("ValueD writes the value and description", x.F(func(x x.X) {
		b := &strings.Builder{}
		z := tab.NewZshTab(b)
		z.ValueD("foo", "the foo")
		x.Equal("foo:the foo\n", b.String())
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
