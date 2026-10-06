package cfg_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lesomnus/xli/cfg"
	"github.com/lesomnus/xli/internal/x"
)

type DbConfig struct {
	Dsn     string `yaml:"dsn"`
	MaxConn int    `yaml:"max_conn"`
}

type LdapConfig struct {
	Addr     string            `yaml:"addr"`
	Insecure bool              `yaml:"insecure"`
	Bases    map[string]string `yaml:"bases"`
	Hosts    []string          `yaml:"hosts"`
}

type Config struct {
	Name    string      `yaml:"name"`
	Version string      `yaml:"version"`
	Db      DbConfig    `yaml:"db"`
	Ldap    *LdapConfig `yaml:"ldap"`
}

// write writes content to a file in a temporary directory and returns its path.
func write(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func env(kvs ...string) []string { return kvs }

func TestLoadFile(t *testing.T) {
	t.Run("values", x.F(func(x x.X) {
		p := write(x.T, `
name: app
version: 1.10
db:
  dsn: "file:app.db"
  max_conn: 0x10
ldap:
  addr: ":389"
  bases: {a: "dc=a"}
  hosts: [h1, h2]
`)
		c := Config{}
		l := cfg.New("app", &c)
		s, err := l.Load(p, nil)
		x.NoError(err)
		x.Equal("app", c.Name)
		x.Equal("1.10", c.Version, "scalars are read as written")
		x.Equal(DbConfig{Dsn: "file:app.db", MaxConn: 16}, c.Db)
		x.Equal(&LdapConfig{Addr: ":389", Bases: map[string]string{"a": "dc=a"}, Hosts: []string{"h1", "h2"}}, c.Ldap)
		x.Equal(p, s.Path)
		x.Len(s.Revision, 12)
	}))
	t.Run("unknown keys are errors with their position", x.F(func(x x.X) {
		p := write(x.T, "name: a\ndb:\n  dns: x\n  max_con: 1\nnmae: b\n")
		l := cfg.New("app", &Config{})
		_, err := l.Load(p, nil)
		x.ErrorContains(err, "3 errors in the configuration")
		x.ErrorContains(err, p+":3:3: db.dns: nothing reads this key (did you mean \"dsn\"?)")
		x.ErrorContains(err, p+":4:3: db.max_con: nothing reads this key")
		x.ErrorContains(err, p+":5:1: nmae: nothing reads this key")
	}))
	t.Run("x- keys are ignored, and hold anchors", x.F(func(x x.X) {
		p := write(x.T, `
x-name: &name app
x-db: &db
  dsn: shared
  max_conn: 3
db:
  <<: *db
  max_conn: 5
name: *name
`)
		c := Config{}
		_, err := cfg.New("app", &c).Load(p, nil)
		x.NoError(err)
		x.Equal("app", c.Name)
		x.Equal(DbConfig{Dsn: "shared", MaxConn: 5}, c.Db, "own keys override merged ones")
	}))
	t.Run("null clears", x.F(func(x x.X) {
		p := write(x.T, "name:\ndb: null\n")
		c := Config{Name: "default", Db: DbConfig{Dsn: "default"}}
		l := cfg.New("app", &c)
		_, err := l.Load(p, nil)
		x.NoError(err)
		x.Equal("", c.Name)
		x.Equal("", c.Db.Dsn)

		o, ok := l.Origin(&c.Name)
		x.True(ok)
		x.Equal(cfg.File, o.Source)
		x.True(o.Cleared)
	}))
	t.Run("type errors", x.F(func(x x.X) {
		p := write(x.T, "db:\n  max_conn: many\nldap:\n  hosts: one\n  insecure: yes\n")
		_, err := cfg.New("app", &Config{}).Load(p, nil)
		x.ErrorContains(err, `db.max_conn: "many" is not an integer`)
		x.ErrorContains(err, "ldap.hosts: want a list, not")
		x.ErrorContains(err, `ldap.insecure: "yes" is not a boolean`)
	}))
	t.Run("YAML 1.1 booleans stay strings", x.F(func(x x.X) {
		p := write(x.T, "name: n\nversion: no\n")
		c := Config{}
		_, err := cfg.New("app", &c).Load(p, nil)
		x.NoError(err)
		x.Equal("n", c.Name)
		x.Equal("no", c.Version)
	}))
	t.Run("more than one document", x.F(func(x x.X) {
		p := write(x.T, "name: a\n---\nname: b\n")
		_, err := cfg.New("app", &Config{}).Load(p, nil)
		x.ErrorContains(err, "more than one document")
	}))
	t.Run("an empty file", x.F(func(x x.X) {
		p := write(x.T, "")
		_, err := cfg.New("app", &Config{}).Load(p, nil)
		x.NoError(err)
	}))
	t.Run("a named file must exist", x.F(func(x x.X) {
		_, err := cfg.New("app", &Config{}).Load(filepath.Join(x.T.TempDir(), "nope.yaml"), nil)
		x.True(errors.Is(err, os.ErrNotExist))
	}))
	t.Run("default paths are tried in order", x.F(func(x x.X) {
		dir := x.T.TempDir()
		yml := filepath.Join(dir, "app.yml")
		x.NoError(os.WriteFile(yml, []byte("name: yml\n"), 0o600))

		c := Config{}
		s, err := cfg.New("app", &c, cfg.WithPaths(filepath.Join(dir, "app.yaml"), yml)).Load("", nil)
		x.NoError(err)
		x.Equal("yml", c.Name)
		x.Equal(yml, s.Path)
	}))
	t.Run("no file at all", x.F(func(x x.X) {
		c := Config{}
		s, err := cfg.New("app", &c, cfg.WithPaths(filepath.Join(x.T.TempDir(), "app.yaml"))).Load("", env("APP_NAME=env"))
		x.NoError(err)
		x.Equal("env", c.Name)
		x.Equal("", s.Path)
	}))
}

func TestLoadReferences(t *testing.T) {
	t.Run("${env:} anywhere in a string", x.F(func(x x.X) {
		p := write(x.T, `
db:
  dsn: "postgres://u:${env:PW}@h/db"
  max_conn: ${env:CONNS}
name: ${env:NOPE:-fallback}
version: $${env:LITERAL}
`)
		c := Config{}
		l := cfg.New("app", &c)
		_, err := l.Load(p, env("PW=pw", "CONNS=4"))
		x.NoError(err)
		x.Equal("postgres://u:pw@h/db", c.Db.Dsn)
		x.Equal(4, c.Db.MaxConn)
		x.Equal("fallback", c.Name)
		x.Equal("${env:LITERAL}", c.Version)

		o, _ := l.Origin(&c.Db.Dsn)
		x.Equal([]string{"${env:PW}"}, o.Refs)
	}))
	t.Run("an unset variable is an error", x.F(func(x x.X) {
		p := write(x.T, "name: ${env:NOPE}\n")
		_, err := cfg.New("app", &Config{}).Load(p, nil)
		x.ErrorContains(err, "name: ${env:NOPE}: NOPE is not set")
	}))
	t.Run("a scheme", x.F(func(x x.X) {
		p := write(x.T, "name: ${upper:x}\n")
		c := Config{}
		_, err := cfg.New("app", &c, cfg.WithScheme("upper", func(ref string) ([]byte, error) {
			return []byte(strings.ToUpper(ref)), nil
		})).Load(p, nil)
		x.NoError(err)
		x.Equal("X", c.Name)
	}))
}

func TestLoadLayers(t *testing.T) {
	p := write(t, "name: file\ndb:\n  dsn: file\n  max_conn: 1\n")
	newLoader := func(c *Config) *cfg.Loader[Config] {
		// What the root holds when the loader is made is the defaults.
		*c = Config{Name: "default", Version: "default", Db: DbConfig{MaxConn: 9}}
		return cfg.New("app", c)
	}

	t.Run("default < file < env", x.F(func(x x.X) {
		c := Config{}
		l := newLoader(&c)
		_, err := l.Load(p, env("APP_DB_DSN=env", "OTHER=1"))
		x.NoError(err)
		x.Equal("file", c.Name)
		x.Equal("default", c.Version)
		x.Equal(DbConfig{Dsn: "env", MaxConn: 1}, c.Db)

		o, _ := l.Origin(&c.Version)
		x.Equal(cfg.Default, o.Source)
		x.False(o.IsSet())
		o, _ = l.Origin(&c.Name)
		x.Equal(cfg.File, o.Source)
		x.Equal(1, o.Line)
		x.Equal(p+":1 (name)", o.String())
		o, _ = l.Origin(&c.Db.Dsn)
		x.Equal(cfg.Env, o.Source)
		x.Equal("APP_DB_DSN", o.Name)
		x.True(o.IsSet())
	}))
	t.Run("an empty variable clears, an unset one does not", x.F(func(x x.X) {
		c := Config{}
		l := newLoader(&c)
		_, err := l.Load(p, env("APP_NAME=", "APP_DB_MAX_CONN="))
		x.NoError(err)
		x.Equal("", c.Name)
		x.Equal(0, c.Db.MaxConn)
		x.Equal("file", c.Db.Dsn)

		o, _ := l.Origin(&c.Name)
		x.Equal(cfg.Env, o.Source)
		x.True(o.Cleared)
	}))
	t.Run("lists and maps from the environment", x.F(func(x x.X) {
		c := Config{}
		l := newLoader(&c)
		_, err := l.Load(p, env("APP_LDAP_HOSTS=a,b", "APP_LDAP_BASES={x: y}"))
		x.NoError(err)
		x.Equal([]string{"a", "b"}, c.Ldap.Hosts)
		x.Equal(map[string]string{"x": "y"}, c.Ldap.Bases)
	}))
	t.Run("a nested pointer stays nil until something is read into it", x.F(func(x x.X) {
		c := Config{}
		_, err := newLoader(&c).Load(p, nil)
		x.NoError(err)
		x.Nil(c.Ldap)
	}))
	t.Run("bad environment values", x.F(func(x x.X) {
		_, err := newLoader(&Config{}).Load(p, env("APP_DB_MAX_CONN=x", "APP_LDAP_INSECURE=maybe"))
		x.ErrorContains(err, `APP_DB_MAX_CONN: "x" is not an integer`)
		x.ErrorContains(err, `APP_LDAP_INSECURE: "maybe" is not a boolean`)
	}))
	t.Run("environment values are not expanded", x.F(func(x x.X) {
		c := Config{}
		l := newLoader(&c)
		_, err := l.Load(p, env("APP_NAME=${env:X}", `APP_LDAP_HOSTS=["${env:X}", "a$$b", "${env:NOPE}"]`, "X=x"))
		x.NoError(err)
		x.Equal("${env:X}", c.Name)
		x.Equal([]string{"${env:X}", "a$$b", "${env:NOPE}"}, c.Ldap.Hosts, "nor in flow syntax")

		o, _ := l.Origin(&c.Ldap.Hosts)
		x.Len(o.Refs, 0)
	}))
}

var sharedLdap = &LdapConfig{Addr: ":389", Hosts: []string{"default"}}

func TestDefaults(t *testing.T) {
	t.Run("what the root holds when the loader is made", x.F(func(x x.X) {
		c := Config{Name: "default", Db: DbConfig{Dsn: "default", MaxConn: 3}}
		l := cfg.New("app", &c)
		s, err := l.Load(write(x.T, "db:\n  dsn: file\n"), nil)
		x.NoError(err)
		x.Equal(Config{Name: "default", Db: DbConfig{Dsn: "file", MaxConn: 3}}, c)
		x.Same(&c, s.Config, "the first load is the root")

		o, _ := l.Origin(&c.Db.MaxConn)
		x.Equal(cfg.Default, o.Source)
	}))
	t.Run("are never written into", x.F(func(x x.X) {
		c := Config{Ldap: sharedLdap}
		l := cfg.New("app", &c)
		_, err := l.Load(write(x.T, "ldap:\n  addr: file\n  hosts: [file]\n"), env("APP_LDAP_INSECURE=true"))
		x.NoError(err)
		x.Equal(&LdapConfig{Addr: "file", Insecure: true, Hosts: []string{"file"}}, c.Ldap)
		x.Equal(&LdapConfig{Addr: ":389", Hosts: []string{"default"}}, sharedLdap)

		s, err := l.Read("", nil)
		x.NoError(err)
		x.Equal(":389", s.Config.Ldap.Addr, "every load starts from them")
	}))
	t.Run("nor is the root but by Load", x.F(func(x x.X) {
		c := Config{Ldap: &LdapConfig{Addr: "default"}}
		l := cfg.New("app", &c)
		p := write(x.T, "ldap:\n  addr: one\n")
		_, err := l.Load(p, nil)
		x.NoError(err)
		ldap := c.Ldap

		_, err = l.Read(write(x.T, "ldap:\n  addr: two\n"), nil)
		x.NoError(err)
		x.Equal("one", ldap.Addr)
		x.Same(ldap, c.Ldap)
	}))
	t.Run("a nil block stays nil when a variable clears a field in it", x.F(func(x x.X) {
		c := Config{}
		_, err := cfg.New("app", &c, cfg.WithPaths()).Load("", env("APP_LDAP_ADDR="))
		x.NoError(err)
		x.Nil(c.Ldap)
	}))
}

func TestUnknownEnv(t *testing.T) {
	c := Config{}
	l := cfg.New("app", &c, cfg.Claims("KEY_"), cfg.WithPaths())
	s, err := l.Load("", env(
		"APP_DB_DNS=typo",
		"APP_KEY_ALICE=claimed",
		"APP_DATA_SERVICE_HOST=10.0.0.1",
		"APP_DATA_PORT_5432_TCP_ADDR=10.0.0.1",
		"APP_NAME=fine",
		"APPLE=other",
	))
	x := x.New(t)
	x.NoError(err)
	x.Equal([]string{"APP_DB_DNS"}, s.Unknown)

	s, err = cfg.New("app", &c, cfg.KeepServiceLinks(), cfg.WithPaths()).Load("", env("APP_DATA_SERVICE_HOST=x"))
	x.NoError(err)
	x.Equal([]string{"APP_DATA_SERVICE_HOST"}, s.Unknown)
}

type Validated struct {
	Port  int             `yaml:"port"`
	Peers []Peer          `yaml:"peers"`
	ByID  map[string]Peer `yaml:"by_id"`
}

func (v *Validated) Validate() error {
	if v.Port == 0 {
		return errors.New("port is required")
	}
	return nil
}

type Peer struct {
	Addr string `yaml:"addr"`
}

func (p Peer) Validate() error {
	if p.Addr == "" {
		return errors.New("addr is required")
	}
	return nil
}

func TestValidate(t *testing.T) {
	p := write(t, "peers: [{addr: a}, {}]\nby_id: {x: {}}\n")
	_, err := cfg.New("app", &Validated{}).Load(p, nil)
	x := x.New(t)

	var le *cfg.LoadError
	x.True(errors.As(err, &le))
	x.Len(le.Errs, 3)
	x.ErrorContains(err, "port is required")
	x.ErrorContains(err, "peers[1]: addr is required")
	x.ErrorContains(err, "by_id.x: addr is required")

	var fe *cfg.FieldError
	x.True(errors.As(le.Errs[0], &fe))

	p = write(t, "port: 1\n")
	_, err = cfg.New("app", &Validated{}).Load(p, env("APP_PORT=0"))
	x.ErrorContains(err, "port is required")
}

func TestOrigin(t *testing.T) {
	c := Config{}
	l := cfg.New("app", &c, cfg.WithPaths())
	x := x.New(t)

	_, ok := l.Origin(&c.Name)
	x.False(ok, "before the first load")

	_, err := l.Load("", env("APP_NAME=a"))
	x.NoError(err)

	o, ok := l.Origin(&c.Name)
	x.True(ok)
	x.Equal("APP_NAME", o.String())

	o, ok = l.Origin(&c.Version)
	x.True(ok)
	x.Equal(cfg.Unset, o.Source)
	x.Equal("unset", o.String())

	copied := c.Db
	_, ok = l.Origin(&copied.Dsn)
	x.False(ok, "a field of a copy is not the root's")
	_, ok = l.Origin(&c.Db)
	x.False(ok, "not a leaf")

	s := l.Current()
	o, ok = s.Origin(&s.Config.Name)
	x.True(ok)
	x.Equal(cfg.Env, o.Source)
	x.Len(s.Origins(), 8)
}

func TestNewPanics(t *testing.T) {
	x := x.New(t)
	panics := func(f func()) (p any) {
		defer func() { p = recover() }()
		f()
		return nil
	}
	x.NotNil(panics(func() { cfg.New("", &Config{}) }))
	x.NotNil(panics(func() { cfg.New[Config]("app", nil) }))
	x.NotNil(panics(func() { cfg.New("app", new(int)) }))
	x.NotNil(panics(func() { cfg.NewFile[Config]("") }))
}

func TestNewFile(t *testing.T) {
	p := write(t, "name: policy\n")
	l := cfg.NewFile[Config](p)
	s, err := l.Load("", env("APP_NAME=ignored"))
	x := x.New(t)
	x.NoError(err)
	x.Equal("policy", s.Config.Name)
	x.Equal([]string(nil), s.Unknown)

	_, err = cfg.NewFile[Config](filepath.Join(t.TempDir(), "nope.yaml")).Load("", nil)
	x.True(errors.Is(err, os.ErrNotExist))
}
