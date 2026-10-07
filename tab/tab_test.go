package tab_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/tab"
)

// sink is a Tab that keeps nothing.
type sink struct{}

func (sink) Value(string)           {}
func (sink) ValueD(string, string)  {}
func (s sink) Group(string) tab.Tab { return s }
func (sink) Files(string)           {}
func (sink) Dirs()                  {}

func TestTabContext(t *testing.T) {
	t.Run("From returns nil when absent", x.F(func(x x.X) {
		x.Nil(tab.From(context.Background()))
	}))
	t.Run("Into and From round-trip", x.F(func(x x.X) {
		z := &sink{}
		ctx := tab.Into(context.Background(), z)
		x.Same(z, tab.From(ctx))
	}))
}
