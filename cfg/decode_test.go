package cfg_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lesomnus/xli/cfg"
	"github.com/lesomnus/xli/internal/x"
)

type Rule struct {
	Name string `yaml:"name"`
	Any  []Rule `yaml:"any"`
}

type Tree map[string]Tree

func TestDecodeAliasLoops(t *testing.T) {
	type C struct {
		Rules []Rule `yaml:"rules"`
		Tree  Tree   `yaml:"tree"`
		Any   any    `yaml:"any"`
	}
	for name, doc := range map[string]string{
		"a list":   "rules:\n  - &r\n    name: a\n    any: [*r]\n",
		"a map":    "tree: &t {x: *t}\n",
		"any":      "any: &a [*a]\n",
		"a merge":  "rules:\n  - &r {name: a, any: [{<<: *r}]}\n",
		"from env": "",
	} {
		t.Run(name, x.F(func(x x.X) {
			l := cfg.New("app", &C{}, cfg.WithPaths())
			var err error
			if doc == "" {
				_, err = l.Load("", env("APP_RULES=[&r {name: a, any: [*r]}]"))
			} else {
				_, err = l.Load(write(x.T, doc), nil)
			}
			x.ErrorContains(err, "an alias refers to a value it is in")
		}))
	}

	t.Run("aliases that multiply", x.F(func(x x.X) {
		b := strings.Builder{}
		b.WriteString("x-0: &a0 [a, a, a, a, a, a, a, a, a, a]\n")
		for i := 1; i < 9; i++ {
			prev := "*a" + string(rune('0'+i-1))
			b.WriteString("x-" + string(rune('0'+i)) + ": &a" + string(rune('0'+i)) + " [" + strings.Repeat(prev+", ", 9) + prev + "]\n")
		}
		b.WriteString("any: *a8\n")
		_, err := cfg.New("app", &C{}).Load(write(x.T, b.String()), nil)
		x.ErrorContains(err, "aliases expand to too many values")
	}))
}

func TestDecodeMergeKeys(t *testing.T) {
	t.Run("a key of the mapping replaces a merged one whole", x.F(func(x x.X) {
		c := Config{}
		_, err := cfg.New("app", &c).Load(write(x.T, "x-b: &b\n  db: {dsn: a, max_conn: 3}\n<<: *b\ndb:\n  dsn: b\n"), nil)
		x.NoError(err)
		x.Equal(DbConfig{Dsn: "b"}, c.Db)
	}))
	t.Run("an earlier mapping overrides a later one", x.F(func(x x.X) {
		c := Config{}
		_, err := cfg.New("app", &c).Load(write(x.T, "x-a: &a {name: a}\nx-b: &b {name: b, version: b}\n<<: [*a, *b]\n"), nil)
		x.NoError(err)
		x.Equal("a", c.Name)
		x.Equal("b", c.Version)
	}))
}

func TestDecodeScalars(t *testing.T) {
	t.Run("integers in any are read as YAML 1.2 reads them", x.F(func(x x.X) {
		type C struct {
			A any `yaml:"a"`
			B any `yaml:"b"`
			U any `yaml:"u"`
			S any `yaml:"s"`
		}
		c := C{}
		_, err := cfg.New("app", &c).Load(write(x.T, "a: 010\nb: -010\nu: 1_000\ns: !!str ~\n"), nil)
		x.NoError(err)
		x.Equal(uint64(10), c.A)
		x.Equal(int64(-10), c.B)
		x.Equal("1_000", c.U)
		x.Equal("~", c.S)
	}))
	t.Run("underscores are not digits", x.F(func(x x.X) {
		type C struct {
			I int `yaml:"i"`
		}
		_, err := cfg.New("app", &C{}).Load(write(x.T, "i: 0x_1F\n"), nil)
		x.ErrorContains(err, `"0x_1F" is not an integer`)
	}))
	t.Run("!!str and !!binary", x.F(func(x x.X) {
		type C struct {
			Name  string `yaml:"name"`
			Bytes []byte `yaml:"bytes"`
		}
		c := C{}
		_, err := cfg.New("app", &c).Load(write(x.T, "name: !!str ~\nbytes: !!binary aGk=\n"), nil)
		x.NoError(err)
		x.Equal("~", c.Name)
		x.Equal([]byte("hi"), c.Bytes)
	}))
	t.Run("map keys that read the same", x.F(func(x x.X) {
		type C struct {
			M map[int]string `yaml:"m"`
		}
		_, err := cfg.New("app", &C{}).Load(write(x.T, "m: {1: a, 01: b}\n"), nil)
		x.ErrorContains(err, "m.01: given twice")
	}))
	t.Run("a reference in a reference", x.F(func(x x.X) {
		_, err := cfg.New("app", &Config{}).Load(write(x.T, "name: ${env:A:-${env:B}}\n"), nil)
		x.ErrorContains(err, "a reference cannot hold another")
	}))
}

func TestDecodeDocuments(t *testing.T) {
	for name, doc := range map[string]string{
		"two markers around a comment": "---\n# x\n---\nname: b\n",
		"two markers in a row":         "name: a\n---\n---\nname: b\n",
	} {
		t.Run(name, x.F(func(x x.X) {
			_, err := cfg.New("app", &Config{}).Load(write(x.T, doc), nil)
			x.ErrorContains(err, "more than one document")
		}))
	}
	for name, doc := range map[string]string{
		"a directive": "%YAML 1.2\n---\nname: a\n",
		"a BOM":       "\xef\xbb\xbfname: a\n",
		"a marker":    "---\nname: a\n...\n",
	} {
		t.Run(name, x.F(func(x x.X) {
			c := Config{}
			_, err := cfg.New("app", &c).Load(write(x.T, doc), nil)
			x.NoError(err)
			x.Equal("a", c.Name)
		}))
	}
}

func TestDecodeErrors(t *testing.T) {
	t.Run("an alias with no anchor is located", x.F(func(x x.X) {
		p := write(x.T, "version: v\nname: *nope\n")
		_, err := cfg.New("app", &Config{}).Load(p, nil)
		x.ErrorContains(err, p+":2:7: name: alias *nope names no anchor")
	}))
	t.Run("an error about the whole file", x.F(func(x x.X) {
		p := write(x.T, "name: [\n")
		_, err := cfg.New("app", &Config{}).Load(p, nil)
		x.ErrorContains(err, p+": [1:7]")
		x.NotContains(err.Error(), "must be quoted")
	}))
	t.Run("the hint about quotes is for the line it is about", x.F(func(x x.X) {
		p := write(x.T, "name: \"${env:X}\"\nversion: [\n")
		_, err := cfg.New("app", &Config{}).Load(p, nil)
		x.NotContains(err.Error(), "must be quoted")
	}))
}

func TestDecodePendingSecretInMap(t *testing.T) {
	type C struct {
		Keys map[string]cfg.Secret `yaml:"keys"`
	}
	missing := filepath.Join(t.TempDir(), "missing")
	c := C{}
	s, err := cfg.New("app", &c).Load(write(t, "keys: {k1: \"${file:"+missing+"}\", k2: lit}\n"), nil)
	x := x.New(t)
	x.NoError(err)
	x.Len(s.Warnings, 1)
	x.Len(c.Keys, 2, "the entry is kept, and read when used")
	x.Equal("${file:"+missing+"}", c.Keys["k1"].Ref())
}
