package arg_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli/arg"
	"github.com/stretchr/testify/require"
)

func TestRest(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		p := arg.RestStrings{}.Parser
		vs, n, err := p.Prase(context.TODO(), []string{})
		require.NoError(t, err)
		require.Zero(t, n)
		require.Empty(t, vs)
	})
	t.Run("values", func(t *testing.T) {
		p := arg.RestStrings{}.Parser
		vs, n, err := p.Prase(context.TODO(), []string{"foo", "bar", "baz"})
		require.NoError(t, err)
		require.Equal(t, 3, n)
		require.Equal(t, []string{"foo", "bar", "baz"}, vs)
	})
}
