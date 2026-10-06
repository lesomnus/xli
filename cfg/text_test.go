package cfg

import (
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/lesomnus/xli/internal/x"
)

func newResolver(env map[string]string) *resolver {
	return &resolver{
		lookup: func(name string) (string, bool) {
			v, ok := env[name]
			return v, ok
		},
		schemes: map[string]Resolver{
			"upper": func(ref string) ([]byte, error) {
				if ref == "fail" {
					return nil, errors.New("boom")
				}
				return []byte("UP:" + ref), nil
			},
		},
	}
}

// textInto reads s into a new T with setText.
func textInto[T any](s string) (T, error) {
	var v T
	err := setText(reflect.ValueOf(&v).Elem(), s, newResolver(nil))
	return v, err
}

func TestSetTextScalars(t *testing.T) {
	x := x.New(t)

	s, err := textInto[string]("a,b ${env:X} $$")
	x.NoError(err)
	x.Equal("a,b ${env:X} $$", s, "strings are taken as they are")

	i, err := textInto[int]("0x1F")
	x.NoError(err)
	x.Equal(31, i)

	_, err = textInto[int8]("300")
	x.ErrorContains(err, "not an integer of 8 bits")

	u, err := textInto[uint16]("42")
	x.NoError(err)
	x.Equal(uint16(42), u)

	f, err := textInto[float64]("1.5")
	x.NoError(err)
	x.Equal(1.5, f)

	b, err := textInto[bool]("true")
	x.NoError(err)
	x.True(b)
	_, err = textInto[bool]("yes")
	x.ErrorContains(err, "not a boolean")

	d, err := textInto[time.Duration]("1m30s")
	x.NoError(err)
	x.Equal(90*time.Second, d)

	p, err := textInto[*int]("7")
	x.NoError(err)
	x.Equal(7, *p)

	a, err := textInto[netip.Addr]("10.0.0.1")
	x.NoError(err, "a TextUnmarshaler")
	x.Equal("10.0.0.1", a.String())

	var anything any
	anything, err = textInto[any]("raw")
	x.NoError(err)
	x.Equal("raw", anything)
}

func TestSetTextLists(t *testing.T) {
	t.Run("comma separated", x.F(func(x x.X) {
		v, err := textInto[[]string]("a,b,,c")
		x.NoError(err)
		x.Equal([]string{"a", "b", "", "c"}, v)
	}))
	t.Run("escapes", x.F(func(x x.X) {
		v, err := textInto[[]string](`a\,b,c\\,d`)
		x.NoError(err)
		x.Equal([]string{"a,b", `c\`, "d"}, v)
	}))
	t.Run("a plain value that starts with a bracket", x.F(func(x x.X) {
		v, err := textInto[[]string](`\[a],b`)
		x.NoError(err)
		x.Equal([]string{"[a]", "b"}, v)
	}))
	t.Run("typed elements", x.F(func(x x.X) {
		v, err := textInto[[]int]("1,2,3")
		x.NoError(err)
		x.Equal([]int{1, 2, 3}, v)

		_, err = textInto[[]int]("1,x")
		x.ErrorContains(err, "[1]")
	}))
	t.Run("flow", x.F(func(x x.X) {
		v, err := textInto[[]string](`[a, "b,c", 1.10]`)
		x.NoError(err)
		x.Equal([]string{"a", "b,c", "1.10"}, v)
	}))
	t.Run("flow of structs", x.F(func(x x.X) {
		type P struct {
			Name string `yaml:"name"`
			Port int    `yaml:"port"`
		}
		v, err := textInto[[]P](`[{name: a, port: 1}, {name: b}]`)
		x.NoError(err)
		x.Equal([]P{{"a", 1}, {"b", 0}}, v)

		_, err = textInto[[]P](`[{nmae: a}]`)
		x.ErrorContains(err, "nothing reads this key")
	}))
	t.Run("array", x.F(func(x x.X) {
		v, err := textInto[[2]int]("1,2")
		x.NoError(err)
		x.Equal([2]int{1, 2}, v)

		_, err = textInto[[2]int]("1")
		x.ErrorContains(err, "want 2 values")
	}))
}

func TestSetTextMaps(t *testing.T) {
	t.Run("key=value", x.F(func(x x.X) {
		v, err := textInto[map[string]string]("a=dc=x,b=host:1")
		x.NoError(err)
		x.Equal(map[string]string{"a": "dc=x", "b": "host:1"}, v)
	}))
	t.Run("escapes", x.F(func(x x.X) {
		v, err := textInto[map[string]string](`a\=b=c\,d`)
		x.NoError(err)
		x.Equal(map[string]string{"a=b": "c,d"}, v)
	}))
	t.Run("typed", x.F(func(x x.X) {
		v, err := textInto[map[string]int]("a=1,b=2")
		x.NoError(err)
		x.Equal(map[string]int{"a": 1, "b": 2}, v)
	}))
	t.Run("missing value", x.F(func(x x.X) {
		_, err := textInto[map[string]string]("a")
		x.ErrorContains(err, `"a": want key=value`)
	}))
	t.Run("flow", x.F(func(x x.X) {
		v, err := textInto[map[string]string](`{a: "1,2", b: x}`)
		x.NoError(err)
		x.Equal(map[string]string{"a": "1,2", "b": "x"}, v)
	}))
}

func TestExpand(t *testing.T) {
	r := newResolver(map[string]string{"PW": "secret", "EMPTY": ""})
	cases := []struct {
		in   string
		want string
		err  string
	}{
		{in: "plain", want: "plain"},
		{in: "postgres://u:${env:PW}@h", want: "postgres://u:secret@h"},
		{in: "${env:EMPTY}", want: ""},
		{in: "${env:NOPE:-fallback}", want: "fallback"},
		{in: "${env:NOPE:-}", want: ""},
		{in: "$${env:PW} and $$", want: "${env:PW} and $"},
		{in: "a $ b", want: "a $ b"},
		{in: "end$", want: "end$"},
		{in: "${upper:x}", want: "UP:x"},
		{in: "${env:NOPE}", err: "NOPE is not set"},
		{in: "${env:1A}", err: "is not a variable name"},
		{in: "${file:/run/x}", err: "only read into a secret field"},
		{in: "${vault:x}", err: `unknown scheme "vault"`},
		{in: "${upper:fail}", err: "boom"},
		{in: "${env:PW", err: "unterminated"},
		{in: "${nocolon}", err: "want ${scheme:...}"},
	}
	for _, tc := range cases {
		t.Run(tc.in, x.F(func(x x.X) {
			r.refs = nil
			got, err := r.expand(tc.in)
			if tc.err != "" {
				x.ErrorContains(err, tc.err)
				return
			}
			x.NoError(err)
			x.Equal(tc.want, got)
		}))
	}

	t.Run("references are recorded", x.F(func(x x.X) {
		r.refs = nil
		_, err := r.expand("${env:PW}:${env:NOPE:-x}")
		x.NoError(err)
		x.Equal([]string{"${env:PW}", "${env:NOPE:-x}"}, r.refs)
	}))
}
