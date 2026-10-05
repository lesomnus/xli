package suggest

import (
	"testing"

	"github.com/lesomnus/xli/internal/x"
)

func TestOf(t *testing.T) {
	names := []string{"deploy", "delete", "describe", "status", "serve"}
	cases := []struct {
		input string
		want  []string
	}{
		{"deplyo", []string{"deploy"}},
		{"delte", []string{"delete"}},
		{"dep", []string{"deploy"}},
		{"de", []string{"deploy", "delete", "describe"}},
		{"staus", []string{"status"}},
		{"xyz", []string{}},
		{"s", []string{"serve", "status"}},
		{"", nil},
	}
	for _, tc := range cases {
		t.Run(tc.input, x.F(func(x x.X) {
			x.Equal(tc.want, Of(tc.input, names))
		}))
	}

	t.Run("exact match and duplicates are skipped", x.F(func(x x.X) {
		x.Equal([]string{"serve"}, Of("serv", []string{"serve", "serve", "serv"}))
	}))
	t.Run("closest first", x.F(func(x x.X) {
		x.Equal([]string{"pot", "port"}, Of("prot", []string{"port", "pot"}))
	}))
}

func TestDistance(t *testing.T) {
	x := x.New(t)
	x.Equal(0, distance("abc", "abc"))
	x.Equal(3, distance("", "abc"))
	x.Equal(2, distance("deplyo", "deploy"))
	x.Equal(1, distance("한글", "한굴"))
}

func TestHint(t *testing.T) {
	x := x.New(t)
	x.Equal("", Hint(nil))
	x.Equal(` (did you mean "a"?)`, Hint([]string{"a"}))
	x.Equal(` (did you mean "a" or "b"?)`, Hint([]string{"a", "b"}))
}
