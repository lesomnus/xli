package arg_test

import (
	"testing"

	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/internal/x"
)

func TestRemains(t *testing.T) {
	t.Run("empty", x.F(func(x x.X) {
		p := arg.Remains{}.Parser
		v, n, err := p.Parse([]string{"--"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal([]string{}, v)
	}))
	t.Run("values", x.F(func(x x.X) {
		p := arg.Remains{}.Parser
		v, n, err := p.Parse([]string{"--", "foo", "bar", "baz"})
		x.NoError(err)
		x.Equal(4, n)
		x.Equal([]string{"foo", "bar", "baz"}, v)
	}))
	t.Run("not starts with two dashes", x.F(func(x x.X) {
		p := arg.Remains{}.Parser
		_, _, err := p.Parse([]string{"foo", "bar", "baz"})
		x.ErrorContains(err, `it must start with "--"`)
	}))
}
