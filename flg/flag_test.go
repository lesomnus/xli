package flg_test

import (
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
	"github.com/stretchr/testify/require"
)

func TestFlagCount(t *testing.T) {
	c := &xli.Command{
		Flags: flg.Flags{
			&flg.String{Name: "foo"},
		},
	}
	v := c.Flags.Get("foo")
	require.Equal(t, 0, v.Count())

	err := c.Run(t.Context(), []string{"--foo=bar", "--foo", "baz"})
	require.NoError(t, err)
	require.Equal(t, 2, v.Count())
}
