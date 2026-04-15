package arg_test

import (
	"testing"

	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/internal/x"
)

func TestRest(t *testing.T) {
	t.Run("empty", x.F(func(x x.X) {
		p := arg.RestStrings{}.Parser
		vs, n, err := p.Parse([]string{})
		x.NoError(err)
		x.Zero(n)
		x.Empty(vs)
	}))
	t.Run("values", x.F(func(x x.X) {
		p := arg.RestStrings{}.Parser
		vs, n, err := p.Parse([]string{"foo", "bar", "baz"})
		x.NoError(err)
		x.Equal(3, n)
		x.Equal([]string{"foo", "bar", "baz"}, vs)
	}))
}
