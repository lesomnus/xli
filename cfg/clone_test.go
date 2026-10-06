package cfg

import (
	"reflect"
	"testing"

	"github.com/lesomnus/xli/internal/x"
)

func TestClone(t *testing.T) {
	type Node struct {
		Name string
		Next *Node
		Tags []string
		Meta map[string][]int
		Any  any
		Arr  [2]*int
		priv *int
	}
	one, two := 1, 2
	n := &Node{
		Name: "a",
		Tags: []string{"t"},
		Meta: map[string][]int{"m": {1}},
		Any:  map[string]any{"k": []any{"v"}},
		Arr:  [2]*int{&one, &two},
		priv: &one,
	}
	n.Next = n

	c := clone(reflect.ValueOf(n)).Interface().(*Node)
	x := x.New(t)
	x.Equal("a", c.Name)
	x.Same(c, c.Next, "a pointer to itself is copied as one")
	x.True(n != c)

	c.Tags[0] = "changed"
	c.Meta["m"][0] = 9
	c.Any.(map[string]any)["k"].([]any)[0] = "changed"
	*c.Arr[0] = 7
	x.Equal("t", n.Tags[0])
	x.Equal(1, n.Meta["m"][0])
	x.Equal("v", n.Any.(map[string]any)["k"].([]any)[0])
	x.Equal(1, one)
	x.Same(n.priv, c.priv, "fields not exported are copied as they are")
}
