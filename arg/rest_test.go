package arg_test

import (
	"testing"

	"github.com/lesomnus/xli/arg"
	"github.com/stretchr/testify/require"
)

func TestRest(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		p := arg.RestStrings{}.Parser
		vs, n, err := p.Prase(t.Context(), []string{})
		require.NoError(t, err)
		require.Zero(t, n)
		require.Empty(t, vs)
	})
	t.Run("values", func(t *testing.T) {
		p := arg.RestStrings{}.Parser
		vs, n, err := p.Prase(t.Context(), []string{"foo", "bar", "baz"})
		require.NoError(t, err)
		require.Equal(t, 3, n)
		require.Equal(t, []string{"foo", "bar", "baz"}, vs)
	})
}
