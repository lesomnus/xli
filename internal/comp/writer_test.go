package comp_test

import (
	"strings"
	"testing"

	"github.com/lesomnus/xli/internal/comp"
	"github.com/lesomnus/xli/internal/x"
)

func TestWriter(t *testing.T) {
	t.Run("Value writes an ungrouped candidate", x.F(func(x x.X) {
		b := &strings.Builder{}
		z := comp.NewWriter(b)
		z.Value("foo")
		x.Equal("v\x1f\x1ffoo\n", b.String())
	}))
	t.Run("ValueD writes the value and description", x.F(func(x x.X) {
		b := &strings.Builder{}
		z := comp.NewWriter(b)
		z.ValueD("foo", "the foo")
		x.Equal("v\x1f\x1ffoo:the foo\n", b.String())
	}))
	t.Run("colons in values are escaped", x.F(func(x x.X) {
		b := &strings.Builder{}
		z := comp.NewWriter(b)
		z.Value("linux:amd64")
		z.ValueD("a:b", "c:d")
		x.Equal("v\x1f\x1flinux\\:amd64\nv\x1f\x1fa\\:b:c:d\n", b.String())
	}))
	t.Run("Group prefixes candidates with the group name", x.F(func(x x.X) {
		b := &strings.Builder{}
		z := comp.NewWriter(b)
		g := z.Group("net")
		g.Value("host")
		g.ValueD("port", "the port")
		x.Equal("v\x1fnet\x1fhost\nv\x1fnet\x1fport:the port\n", b.String())
	}))
	t.Run("Files requests file completion", x.F(func(x x.X) {
		b := &strings.Builder{}
		z := comp.NewWriter(b)
		z.Files("*.go")
		x.Equal("f\x1f*.go\n", b.String())
	}))
	t.Run("Dirs requests directory completion", x.F(func(x x.X) {
		b := &strings.Builder{}
		z := comp.NewWriter(b)
		z.Dirs()
		x.Equal("d\x1f\n", b.String())
	}))
}
