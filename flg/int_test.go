package flg_test

import (
	"testing"

	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
)

func TestInt(t *testing.T) {
	t.Run("int", x.F(func(x x.X) {
		p := flg.Int{}.Parser
		v, err := p.Parse("-42")
		x.NoError(err)
		x.Equal(-42, v)

		s := p.ToString(v)
		x.Equal("-42", s)
	}))
	t.Run("int32", x.F(func(x x.X) {
		p := flg.Int32{}.Parser
		v, err := p.Parse("-42")
		x.NoError(err)
		x.Equal(int32(-42), v)

		s := p.ToString(v)
		x.Equal("-42", s)
	}))
	t.Run("int64", x.F(func(x x.X) {
		p := flg.Int64{}.Parser
		v, err := p.Parse("-42")
		x.NoError(err)
		x.Equal(int64(-42), v)

		s := p.ToString(v)
		x.Equal("-42", s)
	}))
}

func TestUint(t *testing.T) {
	t.Run("uint", x.F(func(x x.X) {
		p := flg.Uint{}.Parser
		v, err := p.Parse("42")
		x.NoError(err)
		x.Equal(uint(42), v)

		s := p.ToString(v)
		x.Equal("42", s)
	}))
	t.Run("uint32", x.F(func(x x.X) {
		p := flg.Uint32{}.Parser
		v, err := p.Parse("42")
		x.NoError(err)
		x.Equal(uint32(42), v)

		s := p.ToString(v)
		x.Equal("42", s)
	}))
	t.Run("uint64", x.F(func(x x.X) {
		p := flg.Uint64{}.Parser
		v, err := p.Parse("42")
		x.NoError(err)
		x.Equal(uint64(42), v)

		s := p.ToString(v)
		x.Equal("42", s)
	}))
}
