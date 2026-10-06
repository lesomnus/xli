package cfg_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/cfg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/xlitest"
)

func TestCmdConfig(t *testing.T) {
	pw := filepath.Join(t.TempDir(), "pw")
	writeAt(t, pw, "secret\n")
	p := write(t, `
ldap:
  bind: password
  key: ${file:`+pw+`}
  bases:
    a: dc=a
`)

	t.Run("prints the configuration with origins", x.F(func(x x.X) {
		root, _ := app(&AppConfig{}, env("APP_LDAP_INSECURE=true"), &ServeConfig{})
		res := xlitest.Run(x.T, root, "--config", p, "config")
		x.NoError(res.Err)
		x.Equal(`ldap:
  insecure: true  # APP_LDAP_INSECURE
  bind: password  # `+p+`:3
  bases: {a: dc=a}  # `+p+`:6
  key: ${file:`+pw+`}  # `+p+`:4 via ${file:`+pw+`}
`, res.Stdout)
	}))
	t.Run("redacts literal secrets", x.F(func(x x.X) {
		root, _ := app(&AppConfig{}, env("APP_LDAP_KEY=hunter2"), &ServeConfig{})
		res := xlitest.Run(x.T, root, "config")
		x.NoError(res.Err)
		x.Equal("ldap:\n  key: <redacted>  # APP_LDAP_KEY\n", res.Stdout)
		x.NotContains(res.Stdout, "hunter2")
	}))
	t.Run("lists the variables", x.F(func(x x.X) {
		root, _ := app(&AppConfig{}, env("APP_LDAP_BIND=key"), &ServeConfig{})
		res := xlitest.Run(x.T, root, "config", "env")
		x.NoError(res.Err)
		x.Equal("APP_LDAP_ADDR\nAPP_LDAP_INSECURE\nAPP_LDAP_BIND\nAPP_LDAP_BASES\nAPP_LDAP_TLS_CERT\nAPP_LDAP_TLS_KEY\nAPP_LDAP_KEY\n", res.Stdout)

		root, _ = app(&AppConfig{}, env("APP_LDAP_BIND=key"), &ServeConfig{})
		res = xlitest.Run(x.T, root, "config", "env", "--set")
		x.NoError(res.Err)
		x.Contains(res.Stdout, "APP_LDAP_ADDR\t-\n")
		x.Contains(res.Stdout, "APP_LDAP_BIND\tset\n")
	}))
}

type Required struct {
	Dsn  string `yaml:"dsn"`
	Name string `yaml:"name"`
}

func (r Required) Validate() error {
	if r.Dsn == "" {
		return errors.New("dsn is required")
	}
	return nil
}

func TestLoadWhereNeeded(t *testing.T) {
	newApp := func(environ []string) (*xli.Command, *cfg.Loader[Required]) {
		c := Required{}
		l := cfg.New("app", &c, cfg.WithPaths(), cfg.WithEnviron(func() []string { return environ }))
		comp := xli.NewCmdCompletion()
		root := &xli.Command{
			Name:     "app",
			Flags:    flg.Flags{cfg.ConfigFlag()},
			Commands: xli.Commands{{Name: "serve", Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error { return next(ctx) })}, comp, cfg.NewCmdConfig(l)},
			Handler:  xli.Chain(cfg.Load(l, comp), xli.RequireSubcommand()),
		}
		return root, l
	}

	t.Run("a bad configuration fails what needs it", x.F(func(x x.X) {
		root, _ := newApp(env("APP_NAME=n"))
		x.ErrorContains(xlitest.Run(x.T, root, "serve").Err, "dsn is required")
	}))
	t.Run("and not what does not", x.F(func(x x.X) {
		root, l := newApp(env("APP_NAME=n"))
		x.NoError(xlitest.Run(x.T, root, "completion", "bash").Err)
		x.NoError(xlitest.Run(x.T, root, "config", "env").Err)
		x.Nil(l.Current(), "not loaded at all")
	}))
	t.Run("config prints what it read, and what is wrong", x.F(func(x x.X) {
		root, _ := newApp(env("APP_NAME=n"))
		res := xlitest.Run(x.T, root, "config")
		x.ErrorContains(res.Err, "dsn is required")
		x.Equal("name: n  # APP_NAME\n", res.Stdout)
	}))
	t.Run("--config= reads no file", x.F(func(x x.X) {
		c := Config{}
		p := write(x.T, "name: from-file\n")
		l := cfg.New("app", &c, cfg.WithPaths(p), cfg.WithEnviron(func() []string { return nil }))
		root := &xli.Command{Name: "app", Flags: flg.Flags{cfg.ConfigFlag()}, Handler: cfg.Load(l)}
		x.NoError(xlitest.Run(x.T, root).Err)
		x.Equal("from-file", c.Name)
		x.NoError(xlitest.Run(x.T, root, "--config=").Err)
		x.Equal("", c.Name)
		x.Equal("", l.Current().Path)
	}))
}

func TestLoadOnASubcommand(t *testing.T) {
	c := Required{}
	l := cfg.New("app", &c, cfg.WithPaths(), cfg.WithEnviron(func() []string { return env("APP_TYPO=x") }))
	root := &xli.Command{
		Name:  "app",
		Flags: flg.Flags{cfg.Bind(l, &c.Name, &flg.String{Name: "name"}), cfg.Bind(l, &c.Dsn, &flg.String{Name: "dsn"})},
		Commands: xli.Commands{{
			Name:    "serve",
			Handler: cfg.Load(l),
		}},
		Handler: xli.Chain(cfg.Load(l), xli.RequireSubcommand()),
	}
	res := xlitest.Run(t, root, "--name=given", "--dsn=d", "serve")
	x := x.New(t)
	x.NoError(res.Err)
	x.Equal("given", c.Name, "the flags of the commands above")
	x.Equal(1, strings.Count(res.Stderr, "APP_TYPO"), "loaded once")
}
