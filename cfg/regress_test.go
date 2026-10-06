package cfg_test

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lesomnus/xli/cfg"
	"github.com/lesomnus/xli/internal/x"
)

func TestMergeKeyCycle(t *testing.T) {
	p := write(t, "x-a: &x\n  <<: *x\n  dsn: q\ndb:\n  <<: *x\n")
	_, err := cfg.New("app", &Config{}).Load(p, nil)
	x.New(t).ErrorContains(err, "merge keys nest too deeply")
}

func TestAnchorRedefined(t *testing.T) {
	type C struct {
		A string `yaml:"a"`
		B string `yaml:"b"`
		C string `yaml:"c"`
		D string `yaml:"d"`
	}
	p := write(t, "a: &x one\nb: *x\nc: &x two\nd: *x\n")
	c := C{}
	_, err := cfg.New("app", &c).Load(p, nil)
	x := x.New(t)
	x.NoError(err)
	x.Equal(C{"one", "one", "two", "two"}, c, "an alias names the nearest anchor before it")
}

func TestAnyResolvesReferences(t *testing.T) {
	type C struct {
		Any map[string]any `yaml:"any"`
		One any            `yaml:"one"`
	}
	p := write(t, "any:\n  e: ${env:X}\n  n: 3\n  l: [a, \"$${env:X}\"]\n  m: {k: \"${env:X}\"}\none: ${env:X}\n")
	c := C{}
	_, err := cfg.New("app", &c).Load(p, env("X=x"))
	x := x.New(t)
	x.NoError(err)
	x.Equal(map[string]any{
		"e": "x",
		"n": uint64(3),
		"l": []any{"a", "${env:X}"},
		"m": map[string]any{"k": "x"},
	}, c.Any)
	x.Equal("x", c.One)
}

func TestUnquotedReferenceInFlow(t *testing.T) {
	p := write(t, "ldap:\n  hosts: [${env:H}]\n")
	_, err := cfg.New("app", &Config{}).Load(p, env("H=h"))
	x := x.New(t)
	x.ErrorContains(err, "a reference inside [...] or {...} must be quoted")

	c := Config{}
	_, err = cfg.New("app", &c).Load(write(t, "ldap:\n  hosts: [\"${env:H}\", b]\n"), env("H=h"))
	x.NoError(err)
	x.Equal([]string{"h", "b"}, c.Ldap.Hosts)
}

func TestPendingSecretOrigin(t *testing.T) {
	type C struct {
		S cfg.Secret `yaml:"s"`
	}
	x := x.New(t)
	missing := filepath.Join(t.TempDir(), "missing")

	c := C{}
	l := cfg.New("app", &c)
	s, err := l.Load(write(t, "s: ${file:"+missing+"}\n"), nil)
	x.NoError(err)
	x.Len(s.Warnings, 1)
	o, _ := l.Origin(&c.S)
	x.Equal(cfg.SourceFile, o.Source, "the value is the file's even though it cannot be read yet")

	c = C{}
	l = cfg.New("app", &c)
	_, err = l.Load(write(t, "s: from-file\n"), env("APP_S=${file:"+missing+"}"))
	x.NoError(err)
	o, _ = l.Origin(&c.S)
	x.Equal(cfg.SourceEnv, o.Source, "the environment's reference overrode the file")
	x.Equal([]string{"${file:" + missing + "}"}, o.Refs)
}

func TestPrintNilSecret(t *testing.T) {
	type C struct {
		S *cfg.Secret `yaml:"s"`
	}
	c := C{}
	l := cfg.New("app", &c, cfg.WithPaths())
	s, err := l.Load("", env("APP_S="))
	x := x.New(t)
	x.NoError(err)

	b := &strings.Builder{}
	x.NoError(s.Print(b))
	x.Equal("s: null  # APP_S (cleared)\n", b.String())
}

func TestNullBlockClearsOrigins(t *testing.T) {
	c := Config{Db: DbConfig{Dsn: "def"}}
	l := cfg.New("app", &c)
	p := write(t, "db:\n")
	_, err := l.Load(p, nil)
	x := x.New(t)
	x.NoError(err)
	x.Equal("", c.Db.Dsn)

	o, _ := l.Origin(&c.Db.Dsn)
	x.Equal(cfg.SourceFile, o.Source)
	x.True(o.Cleared)
	x.Equal("db.dsn", o.Key)
}

type Inner struct{ Bad bool }

func (i Inner) Validate() error {
	if i.Bad {
		return errors.New("inner bad")
	}
	return nil
}

func TestPromotedValidateRunsOnce(t *testing.T) {
	type Outer struct {
		Inner `yaml:",inline"`
	}
	c := Outer{Inner: Inner{Bad: true}}
	_, err := cfg.New("app", &c, cfg.WithPaths()).Load("", nil)
	x := x.New(t)
	var le *cfg.LoadError
	x.True(errors.As(err, &le))
	x.Len(le.Errs, 1)
}

func TestWatchKeepsToTheFileItFound(t *testing.T) {
	x := x.New(t)
	dir := t.TempDir()
	yaml, yml := filepath.Join(dir, "app.yaml"), filepath.Join(dir, "app.yml")
	writeAt(t, yaml, "name: one\n")

	c := Config{}
	l := cfg.New("app", &c, cfg.WithPaths(yaml, yml), cfg.WithInterval(10*time.Millisecond))
	_, err := l.Load("", nil)
	x.NoError(err)

	ch := watch(t, l)
	writeAt(t, yml, "name: other\n")
	x.NoError(os.Remove(yaml))

	e := next(t, ch)
	x.True(errors.Is(e.err, os.ErrNotExist), "a file that disappeared is an error")
	quiet(t, ch)
	x.Equal("one", l.Current().Config.Name, "not the defaults, nor the other default path")
}

func TestNumbers(t *testing.T) {
	type C struct {
		I  int     `yaml:"i"`
		H  int     `yaml:"h"`
		O  uint    `yaml:"o"`
		F  float64 `yaml:"f"`
		N  float64 `yaml:"n"`
		E  float64 `yaml:"e"`
		U  int     `yaml:"u"`
		Ue int     `yaml:"ue"`
	}
	c := C{}
	p := write(t, "i: 010\nh: 0x1F\no: 0o17\nf: -.inf\nn: .NaN\n")
	_, err := cfg.New("app", &c).Load(p, env("APP_E=.inf", "APP_UE=-010"))
	x := x.New(t)
	x.NoError(err)
	x.Equal(10, c.I, "a leading zero does not make a number octal")
	x.Equal(31, c.H)
	x.Equal(uint(15), c.O)
	x.True(math.IsInf(c.F, -1))
	x.True(math.IsNaN(c.N))
	x.True(math.IsInf(c.E, 1))
	x.Equal(-10, c.Ue)

	_, err = cfg.New("app", &C{}).Load(write(t, "u: 1_000\n"), nil)
	x.ErrorContains(err, `"1_000" is not an integer`)
}

type Level string

func TestEmbeddedNonStruct(t *testing.T) {
	type C struct {
		Level
		Name string `yaml:"name"`
	}
	c := C{}
	_, err := cfg.New("app", &c).Load(write(t, "level: debug\nname: a\n"), nil)
	x := x.New(t)
	x.NoError(err)
	x.Equal(Level("debug"), c.Level)
}
