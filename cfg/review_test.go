package cfg_test

import (
	"math/big"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/cfg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/xlitest"
)

func TestPrintShowsWhatIsInForce(t *testing.T) {
	type C struct {
		Addr string `yaml:"addr"`
	}
	p := write(t, "addr: ${env:HOST}:389\n")
	c := C{}
	l := cfg.New("app", &c, cfg.WithEnviron(func() []string { return env("HOST=h", "APP_ADDR=override:636") }))
	root := &xli.Command{Name: "app", Flags: flg.Flags{cfg.ConfigFlag()}, Commands: xli.Commands{cfg.NewCmdConfig(l)}, Handler: cfg.Load(l)}
	res := xlitest.Run(t, root, "--config", p, "config")
	x := x.New(t)
	x.NoError(res.Err)
	x.Equal("addr: override:636  # APP_ADDR\n", res.Stdout, "not the file's, which the environment set over")
}

func TestPrintSecretBlocks(t *testing.T) {
	type C struct {
		Keys struct {
			Signing string `yaml:"signing"`
		} `yaml:"keys"`
		Credentials *struct {
			Github string `yaml:"github"`
		} `yaml:"credentials"`
	}
	c := C{}
	s, err := cfg.New("app", &c).Load(write(t, "keys: {signing: SIGNKEY}\ncredentials: {github: GHTOKEN}\n"), nil)
	x := x.New(t)
	x.NoError(err)
	out := printed(t, s)
	x.NotContains(out, "SIGNKEY")
	x.NotContains(out, "GHTOKEN")
}

func TestFileSnapshotsAreTheirOwn(t *testing.T) {
	p := write(t, "name: one\n")
	f := cfg.NewFile[Config](p)
	s1, err := f.Load()
	x := x.New(t)
	x.NoError(err)
	rotate(t, p, "name: two\n")
	s2, err := f.Load()
	x.NoError(err)
	x.Equal("one", s1.Config.Name)
	x.Equal("two", s2.Config.Name)
}

func TestPrintReadsBackTrickyText(t *testing.T) {
	type C struct {
		Map   map[string]string `yaml:"map"`
		Q     string            `yaml:"q"`
		Dash  string            `yaml:"dash"`
		Hash  string            `yaml:"hash"`
		Null  string            `yaml:"null_"`
		Tilde string            `yaml:"tilde"`
		Uni   string            `yaml:"uni"`
		List  []string          `yaml:"list"`
		Exp   any               `yaml:"exp"`
		Bytes []byte            `yaml:"bytes"`
		Nil   []string          `yaml:"nil"`
		Re    *regexp.Regexp    `yaml:"re"`
		Big   big.Int           `yaml:"big"`
		PBig  *big.Int          `yaml:"pbig"`
	}
	c := C{
		Map: map[string]string{
			"db:5432": "primary", "addr": "10.0.0.1:80", "time": "12:30", "v6": "::1",
			"<<": "merge?", "price$$": "x}", "${env:X}": "}", "?": "?",
		},
		Q:     "?",
		Dash:  "-a",
		Hash:  "#x",
		Null:  "null",
		Tilde: "~",
		Uni:   "é ünï",
		List:  []string{"a: b", "[x]", "{y}", "- z", "& w", "* v", "! u", "% t", "@ s", "` r"},
		Exp:   1e21,
		Bytes: []byte{},
		Re:    regexp.MustCompile(`^a+$`),
		Big:   *big.NewInt(42),
		PBig:  big.NewInt(-7),
	}
	l := cfg.New("app", &c, cfg.WithPaths())
	s, err := l.Load("", env("APP_NIL="))
	x := x.New(t)
	x.NoError(err)
	want := *s.Config

	out := printed(t, s)
	got := C{}
	_, err = cfg.New("app", &got).Load(write(t, out), nil)
	x.NoError(err, out)
	x.Equal(want.Map, got.Map, out)
	x.Equal(want.List, got.List, out)
	x.Equal(want.Exp, got.Exp, out)
	x.Equal(want.Re.String(), got.Re.String(), out)
	x.Equal(want.Big.String(), got.Big.String(), out)
	x.Equal(want.PBig.String(), got.PBig.String(), out)
	x.Equal([]byte{}, got.Bytes, out)
	x.Nil(got.Nil, out)
	for _, f := range [][2]string{{want.Q, got.Q}, {want.Dash, got.Dash}, {want.Hash, got.Hash}, {want.Null, got.Null}, {want.Tilde, got.Tilde}, {want.Uni, got.Uni}} {
		x.Equal(f[0], f[1], out)
	}
}

func TestPrintAsWrittenKeepsScalars(t *testing.T) {
	type C struct {
		List []string `yaml:"list"`
	}
	c := C{}
	s, err := cfg.New("app", &c).Load(write(t, "list: [\"${env:V}\", 1.10, 0x1F, 010]\n"), env("V=v"))
	x := x.New(t)
	x.NoError(err)
	x.Equal([]string{"v", "1.10", "0x1F", "010"}, c.List)

	got := C{}
	_, err = cfg.New("app", &got).Load(write(t, printed(t, s)), env("V=v"))
	x.NoError(err)
	x.Equal(c.List, got.List)
}

func TestPendingSecretPointers(t *testing.T) {
	type C struct {
		Pw   *cfg.Secret   `yaml:"pw"`
		List []*cfg.Secret `yaml:"list"`
	}
	missing := "${file:" + filepath.Join(t.TempDir(), "missing") + "}"
	c := C{}
	s, err := cfg.New("app", &c, cfg.WithPaths()).Load("", env("APP_PW="+missing, "APP_LIST=lit,"+missing))
	x := x.New(t)
	x.NoError(err)
	x.Len(s.Warnings, 2)
	x.NotNil(c.Pw)
	x.Equal(missing, c.Pw.Ref())
	x.Len(c.List, 2)
	x.NotNil(c.List[1])
	_, err = c.List[1].Value()
	x.ErrorContains(err, "secret file")
}

func TestRedactDsn(t *testing.T) {
	type C struct {
		Dsn string `yaml:"dsn"`
	}
	for _, dsn := range []string{
		"host=db password='my secret pw' dbname=x",
		"host=db password = hunter2 dbname=x",
		"postgres://u:p@ss@h/db",
		"user:hunter2@tcp(db:3306)/app",
		"user:hunter2@/app",
	} {
		t.Run(dsn, x.F(func(x x.X) {
			c := C{}
			s, err := cfg.New("app", &c, cfg.WithPaths()).Load("", env("APP_DSN="+dsn))
			x.NoError(err)
			out := printed(x.T, s)
			for _, secret := range []string{"secret", "hunter2", "p@ss", "ss@"} {
				x.NotContains(out, secret)
			}
			x.Contains(out, "<redacted>")
		}))
	}
}

// Wrapped decodes itself, holding a secret.
type Wrapped struct {
	Token cfg.Secret `yaml:"token"`
}

func (w *Wrapped) UnmarshalYAML(unmarshal func(any) error) error {
	type plain Wrapped
	return unmarshal((*plain)(w))
}

func TestSecretFileInATypeThatDecodesItself(t *testing.T) {
	type C struct {
		W Wrapped `yaml:"w"`
	}
	path := filepath.Join(t.TempDir(), "token")
	writeAt(t, path, "s3cret\n")
	c := C{}
	_, err := cfg.New("app", &c).Load(write(t, "w: {token: \"${file:"+path+"}\"}\n"), nil)
	x := x.New(t)
	x.NoError(err)
	v, err := c.W.Token.Value()
	x.NoError(err)
	x.Equal("s3cret", v)
}

func TestNewRefusesInListElements(t *testing.T) {
	type Ext struct {
		Name string `yaml:"name"`
		Ext  string `yaml:"x-ext"`
	}
	type Dup struct {
		A string `yaml:"a"`
		B string `yaml:"a"`
	}
	type C1 struct {
		Items []Ext `yaml:"items"`
	}
	type C2 struct {
		Items map[string]Dup `yaml:"items"`
	}
	x := x.New(t)
	panics := func(f func()) (p any) {
		defer func() { p = recover() }()
		f()
		return nil
	}
	x.NotNil(panics(func() { cfg.New("app", &C1{}) }))
	x.NotNil(panics(func() { cfg.New("app", &C2{}) }))
}

func TestMergedKeysThatReadTheSame(t *testing.T) {
	type C struct {
		M map[int]string `yaml:"m"`
	}
	c := C{}
	_, err := cfg.New("app", &c).Load(write(t, "x-m: &m {1: merged}\nm: {<<: *m, 01: own}\n"), nil)
	x := x.New(t)
	x.NoError(err)
	x.Equal(map[int]string{1: "own"}, c.M)
}
