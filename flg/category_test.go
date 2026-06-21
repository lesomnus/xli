package flg_test

import (
	"testing"

	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
)

func TestFlagsWithCategory(t *testing.T) {
	t.Run("WithCategory sets a category visible via Info", x.F(func(x x.X) {
		fs := flg.Flags{}.WithCategory("net",
			&flg.String{Name: "host"},
			&flg.Int{Name: "port"},
		)
		x.Equal("net", fs.Get("host").Info().Category)
		x.Equal("net", fs.Get("port").Info().Category)
	}))
	t.Run("ByCategory groups flags by category", x.F(func(x x.X) {
		fs := flg.Flags{&flg.String{Name: "verbose"}}.
			WithCategory("net", &flg.String{Name: "host"})
		groups := fs.ByCategory()
		x.Equal(2, len(groups))
	}))
}
