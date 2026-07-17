package flg_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/mode"
)

func TestSwitchParser(t *testing.T) {
	p := flg.Switch{}.Parser

	t.Run("parse", x.F(func(x x.X) {
		v, err := p.Parse("")
		x.NoError(err)
		x.True(v)

		v, err = p.Parse("true")
		x.NoError(err)
		x.True(v)

		v, err = p.Parse("false")
		x.NoError(err)
		x.False(v)

		_, err = p.Parse("x")
		x.NotNil(err)
	}))

	t.Run("to string", x.F(func(x x.X) {
		x.Equal("true", p.ToString(true))
		x.Equal("false", p.ToString(false))
	}))
}

func TestFloatParser(t *testing.T) {
	t.Run("float32", x.F(func(x x.X) {
		p := flg.Float32{}.Parser
		x.Equal("0.25", p.ToString(float32(0.25)))
	}))
	t.Run("float64", x.F(func(x x.X) {
		p := flg.Float64{}.Parser
		x.Equal("0.25", p.ToString(0.25))
	}))
}

func TestDurationParser(t *testing.T) {
	t.Run("to string", x.F(func(x x.X) {
		p := flg.Duration{}.Parser
		x.Equal("1m30s", p.ToString(time.Duration(90*time.Second)))
	}))
}

func TestBaseHandle(t *testing.T) {
	t.Run("tab mode is no-op for state", x.F(func(x x.X) {
		f := &flg.String{Name: "x"}
		ctx := mode.Into(context.Background(), mode.Tab)
		x.NoError(f.Handle(ctx, "ignored"))
		x.Equal(0, f.Count())
		_, ok := f.Get()
		x.False(ok)
	}))

	t.Run("handler is invoked", x.F(func(x x.X) {
		errBoom := errors.New("boom")
		observed := ""
		f := &flg.String{Name: "x", Handler: flg.OnRun[string](func(ctx context.Context, v string) error {
			observed = v
			return errBoom
		})}
		ctx := mode.Into(context.Background(), mode.Run)
		err := f.Handle(ctx, "hi")
		x.ErrorContains(err, "boom")
		x.Equal("hi", observed)
	}))
}
