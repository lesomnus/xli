package arg_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli/arg"
	"github.com/stretchr/testify/require"
)

func TestRemains(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		p := arg.Remains{}.Parser
		v, n, err := p.Parse(context.TODO(), []string{"--"})
		require.NoError(t, err)
		require.Equal(t, 1, n)
		require.Equal(t, []string{}, v)
	})
	t.Run("values", func(t *testing.T) {
		p := arg.Remains{}.Parser
		v, n, err := p.Parse(context.TODO(), []string{"--", "foo", "bar", "baz"})
		require.NoError(t, err)
		require.Equal(t, 4, n)
		require.Equal(t, []string{"foo", "bar", "baz"}, v)
	})
	t.Run("not starts with two dashes", func(t *testing.T) {
		p := arg.Remains{}.Parser
		_, _, err := p.Parse(context.TODO(), []string{"foo", "bar", "baz"})
		require.ErrorContains(t, err, `it must start with "--"`)
	})
}
