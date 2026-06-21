package xli_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/internal/x"
)

// Both S and D must satisfy the Stringer interface.
var (
	_ xli.Stringer = xli.S("")
	_ xli.Stringer = xli.D(nil)
)

func TestStringer(t *testing.T) {
	t.Run("S returns its literal value", x.F(func(x x.X) {
		var s xli.Stringer = xli.S("hello")
		x.Equal("hello", s.String(context.Background(), nil))
	}))
	t.Run("D delegates to its function", x.F(func(x x.X) {
		var d xli.Stringer = xli.D(func(ctx context.Context, cmd *xli.Command) string {
			return "world"
		})
		x.Equal("world", d.String(context.Background(), nil))
	}))
}
