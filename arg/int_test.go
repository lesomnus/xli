package arg_test

import (
	"testing"

	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/internal/x"
)

func TestIntArgParser(t *testing.T) {
	t.Run("Int", x.F(func(x x.X) {
		p := arg.Int{}.Parser
		x.Equal("int", p.String())

		v, n, err := p.Parse([]string{"42"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal(int(42), v)

		v, n, err = p.Parse([]string{"-42"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal(int(-42), v)

		_, _, err = p.Parse([]string{"notnum"})
		x.NotNil(err)
	}))
	t.Run("Int32", x.F(func(x x.X) {
		p := arg.Int32{}.Parser
		x.Equal("int32", p.String())

		v, n, err := p.Parse([]string{"42"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal(int32(42), v)

		v, n, err = p.Parse([]string{"-42"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal(int32(-42), v)

		_, _, err = p.Parse([]string{"notnum"})
		x.NotNil(err)

		_, _, err = p.Parse([]string{"9999999999999"})
		x.NotNil(err)
	}))
	t.Run("Int64", x.F(func(x x.X) {
		p := arg.Int64{}.Parser
		x.Equal("int64", p.String())

		v, n, err := p.Parse([]string{"42"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal(int64(42), v)

		v, n, err = p.Parse([]string{"-42"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal(int64(-42), v)

		_, _, err = p.Parse([]string{"notnum"})
		x.NotNil(err)
	}))
	t.Run("Uint", x.F(func(x x.X) {
		p := arg.Uint{}.Parser
		x.Equal("uint", p.String())

		v, n, err := p.Parse([]string{"42"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal(uint(42), v)

		_, _, err = p.Parse([]string{"notnum"})
		x.NotNil(err)
	}))
	t.Run("Uint32", x.F(func(x x.X) {
		p := arg.Uint32{}.Parser
		x.Equal("uint32", p.String())

		v, n, err := p.Parse([]string{"42"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal(uint32(42), v)

		_, _, err = p.Parse([]string{"notnum"})
		x.NotNil(err)

		_, _, err = p.Parse([]string{"9999999999999"})
		x.NotNil(err)
	}))
	t.Run("Uint64", x.F(func(x x.X) {
		p := arg.Uint64{}.Parser
		x.Equal("uint64", p.String())

		v, n, err := p.Parse([]string{"42"})
		x.NoError(err)
		x.Equal(1, n)
		x.Equal(uint64(42), v)

		_, _, err = p.Parse([]string{"notnum"})
		x.NotNil(err)
	}))
}
