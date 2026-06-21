package arg_test

import (
	"context"
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

func TestRestHandler(t *testing.T) {
	t.Run("handler is invoked with parsed values", x.F(func(x x.X) {
		got := []string{}
		a := &arg.RestStrings{
			Name: "STRING",
			Handler: arg.Handle(func(ctx context.Context, vs []string) error {
				got = vs
				return nil
			}),
		}

		n, err := a.Parse([]string{"foo", "bar"})
		x.NoError(err)
		x.Equal(2, n)

		handle := a.Info().Handle
		x.NotNil(handle)
		handle(context.Background())
		x.Equal([]string{"foo", "bar"}, got)
	}))
}
