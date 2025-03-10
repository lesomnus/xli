package flg_test

import (
	"testing"

	"github.com/lesomnus/xli/flg"
	"github.com/stretchr/testify/require"
)

func TestInt(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		p := flg.Int{}.Parser
		v, err := p.Parse("-42")
		require.NoError(t, err)
		require.Equal(t, -42, v)

		s := p.ToString(v)
		require.Equal(t, "-42", s)
	})
	t.Run("int32", func(t *testing.T) {
		p := flg.Int32{}.Parser
		v, err := p.Parse("-42")
		require.NoError(t, err)
		require.Equal(t, int32(-42), v)

		s := p.ToString(v)
		require.Equal(t, "-42", s)
	})
	t.Run("int64", func(t *testing.T) {
		p := flg.Int64{}.Parser
		v, err := p.Parse("-42")
		require.NoError(t, err)
		require.Equal(t, int64(-42), v)

		s := p.ToString(v)
		require.Equal(t, "-42", s)
	})
}

func TestUint(t *testing.T) {
	t.Run("uint", func(t *testing.T) {
		p := flg.Uint{}.Parser
		v, err := p.Parse("42")
		require.NoError(t, err)
		require.Equal(t, uint(42), v)

		s := p.ToString(v)
		require.Equal(t, "42", s)
	})
	t.Run("uint32", func(t *testing.T) {
		p := flg.Uint32{}.Parser
		v, err := p.Parse("42")
		require.NoError(t, err)
		require.Equal(t, uint32(42), v)

		s := p.ToString(v)
		require.Equal(t, "42", s)
	})
	t.Run("uint64", func(t *testing.T) {
		p := flg.Uint64{}.Parser
		v, err := p.Parse("42")
		require.NoError(t, err)
		require.Equal(t, uint64(42), v)

		s := p.ToString(v)
		require.Equal(t, "42", s)
	})
}
