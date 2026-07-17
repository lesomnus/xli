package arg_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/mode"
)

func TestBaseString(t *testing.T) {
	t.Run("required", x.F(func(x x.X) {
		a := &arg.String{Name: "SRC"}
		x.Equal("<SRC>", a.String())
	}))
	t.Run("optional", x.F(func(x x.X) {
		a := &arg.String{Name: "DST", Optional: true}
		x.Equal("[DST]", a.String())
	}))
}

func TestBaseIsMany(t *testing.T) {
	x := x.New(t)
	a := &arg.String{Name: "x"}
	x.False(a.IsMany())
}

func TestBaseInfoHandle(t *testing.T) {
	bg := context.Background()

	t.Run("run mode with parsed value invokes handler", x.F(func(x x.X) {
		called := false
		seen := ""
		a := &arg.String{Name: "x", Handler: arg.OnRun[string](func(ctx context.Context, v string) error {
			called = true
			seen = v
			return nil
		})}

		n, err := a.Parse([]string{"val"})
		x.NoError(err)
		x.Equal(1, n)

		a.Info().Handle(mode.Into(bg, mode.Run))
		x.True(called)
		x.Equal("val", seen)
	}))

	t.Run("run mode without parsed value is a no-op", x.F(func(x x.X) {
		called := false
		a := &arg.String{Name: "x", Handler: arg.OnRun[string](func(ctx context.Context, v string) error {
			called = true
			return nil
		})}

		a.Info().Handle(mode.Into(bg, mode.Run))
		x.False(called)
	}))

	t.Run("tab mode passes zero value", x.F(func(x x.X) {
		called := false
		seen := "unset"
		a := &arg.String{Name: "x", Handler: arg.Handle(func(ctx context.Context, v string) error {
			called = true
			seen = v
			return nil
		})}

		a.Info().Handle(mode.Into(bg, mode.Tab))
		x.True(called)
		x.Equal("", seen)
	}))

	t.Run("nil handler is a no-op", x.F(func(x x.X) {
		a := &arg.String{Name: "x"}
		a.Info().Handle(mode.Into(bg, mode.Run))
	}))
}
