package flg_test

import (
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
)

func TestFlagCount(t *testing.T) {
	x := x.New(t)

	c := &xli.Command{
		Flags: flg.Flags{
			&flg.String{Name: "foo"},
		},
	}
	v := c.Flags.Get("foo")
	x.Equal(0, v.Count())

	err := c.Run(t.Context(), []string{"--foo=bar", "--foo", "baz"})
	x.NoError(err)
	x.Equal(2, v.Count())
}
