package xli_test

import (
	"errors"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/internal/x"
)

// twoArg is a positional argument whose parser consumes two tokens at once.
// It is used to reach the "required argument not given" path in frame.prepare,
// which is only reachable when an earlier parser consumes more than one token.
type twoArg struct {
	name string
}

func (a *twoArg) Info() *arg.Info { return &arg.Info{Name: a.name} }
func (a *twoArg) Parse(rest []string) (int, error) {
	if len(rest) < 2 {
		return 0, nil
	}
	return 2, nil
}
func (a *twoArg) IsOptional() bool { return false }
func (a *twoArg) IsMany() bool     { return false }

func TestPrepareMissingArg(t *testing.T) {
	t.Run("missing required arg returns error instead of panicking", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&twoArg{name: "PAIR"},
				&arg.String{Name: "TAIL"},
			},
		}

		err := c.Run(t.Context(), []string{"x", "y"})
		x.True(errors.Is(err, xli.ErrNeedArgs))
		x.ErrorContains(err, "TAIL")
	}))
}
