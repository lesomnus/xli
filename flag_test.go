package xli_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
	"github.com/stretchr/testify/require"
)

func TestFlagCount(t *testing.T) {
	c := &xli.Command{
		Flags: xli.Flags{
			&flg.String{Name: "foo"},
		},
	}
	v := c.Flags.Get("foo")
	require.Equal(t, 0, v.Count())

	_, err := c.Run(context.TODO(), []string{"--foo=bar", "--foo", "baz"})
	require.NoError(t, err)
	require.Equal(t, 2, v.Count())
}
