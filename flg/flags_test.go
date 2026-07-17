package flg_test

import (
	"testing"

	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
)

func TestFlags(t *testing.T) {
	newFlags := func() flg.Flags {
		return flg.Flags{
			&flg.String{Name: "foo", Alias: 'f'},
			&flg.String{Name: "qux"},
		}
	}

	t.Run("Get miss", x.F(func(x x.X) {
		fs := newFlags()
		x.Nil(fs.Get("nope"))
	}))

	t.Run("Get hit", x.F(func(x x.X) {
		fs := newFlags()
		f := fs.Get("foo")
		x.NotNil(f)
		x.Equal("foo", f.Info().Name)
	}))

	t.Run("GetByAlias", x.F(func(x x.X) {
		fs := newFlags()
		f := fs.GetByAlias('f')
		x.NotNil(f)
		x.Equal("foo", f.Info().Name)
		x.Nil(fs.GetByAlias('z'))
	}))

	t.Run("Info String aliased", x.F(func(x x.X) {
		fs := newFlags()
		x.Equal("-f,--foo string", fs.Get("foo").Info().String())
	}))

	t.Run("Info String no alias", x.F(func(x x.X) {
		fs := newFlags()
		x.Equal("   --qux string", fs.Get("qux").Info().String())
	}))
}
