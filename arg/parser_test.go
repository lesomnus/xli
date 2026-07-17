package arg_test

import (
	"testing"
	"time"

	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
)

func TestFloat32Parser(t *testing.T) {
	t.Run("parse", x.F(func(x x.X) {
		p := arg.Float32{}.Parser
		v, n, err := p.Parse([]string{"0.25"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal(float32(0.25), v)
		x.Equal("float32", p.String())
	}))
	t.Run("invalid", x.F(func(x x.X) {
		p := arg.Float32{}.Parser
		_, n, err := p.Parse([]string{"foo"})
		x.NotNil(err)
		x.Equal(1, n)
	}))
}

func TestFloat64Parser(t *testing.T) {
	t.Run("parse", x.F(func(x x.X) {
		p := arg.Float64{}.Parser
		v, n, err := p.Parse([]string{"0.5"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal(float64(0.5), v)
		x.Equal("float64", p.String())
	}))
	t.Run("invalid", x.F(func(x x.X) {
		p := arg.Float64{}.Parser
		_, n, err := p.Parse([]string{"foo"})
		x.NotNil(err)
		x.Equal(1, n)
	}))
}

func TestStringParser(t *testing.T) {
	t.Run("parse", x.F(func(x x.X) {
		p := arg.String{}.Parser
		v, n, err := p.Parse([]string{"hello"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal("hello", v)
		x.Equal("string", p.String())
	}))
}

func TestDurationParser(t *testing.T) {
	t.Run("parse", x.F(func(x x.X) {
		p := arg.Duration{}.Parser
		v, n, err := p.Parse([]string{"90s"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal(time.Duration(90*time.Second), v)
		x.Equal("duration", p.String())
	}))
	t.Run("invalid", x.F(func(x x.X) {
		p := arg.Duration{}.Parser
		_, n, err := p.Parse([]string{"foo"})
		x.NotNil(err)
		x.Equal(1, n)
	}))
}

func TestRemainsParser(t *testing.T) {
	t.Run("parse", x.F(func(x x.X) {
		p := arg.Remains{}.Parser
		v, n, err := p.Parse([]string{"--", "a", "b"})
		x.NoError(err)
		x.Equal(3, n)
		x.Equal([]string{"a", "b"}, v)
		x.Equal("--", p.String())
	}))
	t.Run("no double dash", x.F(func(x x.X) {
		p := arg.Remains{}.Parser
		v, n, err := p.Parse([]string{"a", "b"})
		x.NotNil(err)
		x.Equal(0, n)
		x.Nil(v)
	}))
}

func TestMonoParser(t *testing.T) {
	t.Run("parse", x.F(func(x x.X) {
		p := arg.MonoParser[int, flg.IntParser]{P: flg.Int{}.Parser}
		v, n, err := p.Parse([]string{"42"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal(42, v)
		x.Equal("int", p.String())
	}))
	t.Run("invalid", x.F(func(x x.X) {
		p := arg.MonoParser[int, flg.IntParser]{P: flg.Int{}.Parser}
		_, n, err := p.Parse([]string{"foo"})
		x.NotNil(err)
		x.Equal(0, n)
	}))
}
