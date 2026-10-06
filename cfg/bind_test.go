package cfg_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/cfg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/xlitest"
)

type TlsConfig struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

type ServeConfig struct {
	Addr     string            `yaml:"addr"`
	Insecure bool              `yaml:"insecure"`
	Bind     string            `yaml:"bind"`
	Bases    map[string]string `yaml:"bases"`
	Tls      TlsConfig         `yaml:"tls"`
	Key      cfg.Secret        `yaml:"key"`
}

type AppConfig struct {
	Ldap ServeConfig `yaml:"ldap"`
}

func parseCertPair(v string) (TlsConfig, error) {
	cert, key, ok := strings.Cut(v, ",")
	if !ok {
		return TlsConfig{}, errors.New("want cert.pem,key.pem")
	}
	return TlsConfig{Cert: cert, Key: key}, nil
}

func parseAliasMap(vs []string) (map[string]string, error) {
	m := map[string]string{}
	for _, v := range vs {
		k, s, ok := strings.Cut(v, "=")
		if !ok {
			return nil, fmt.Errorf("%q: want alias=suffix", v)
		}
		m[k] = s
	}
	return m, nil
}

// app is a root command with `ldap serve`, shaped like roster's. got is the
// configuration serve ran with.
func app(c *AppConfig, environ []string, got *ServeConfig) (*xli.Command, *cfg.Loader[AppConfig]) {
	l := cfg.New("app", c, cfg.WithEnviron(func() []string { return environ }), cfg.WithPaths())
	lc := &c.Ldap
	listen := ":389"

	serve := &xli.Command{
		Name: "serve",
		Flags: flg.Flags{
			cfg.Bind(l, &lc.Addr, &flg.String{Name: "listen", Brief: "where to listen", Default: &listen}),
			cfg.Bind(l, &lc.Insecure, &flg.Switch{Name: "insecure"}),
			cfg.Bind(l, &lc.Bind, &flg.Choice{Name: "bind", Parser: flg.ChoiceParser{"key", "password"}}),
			cfg.BindFunc(l, &lc.Tls, &flg.String{Name: "tls"}, parseCertPair),
			cfg.BindFunc(l, &lc.Bases, &flg.Strings{Name: "base"}, parseAliasMap),
			cfg.BindText(l, &lc.Key, &flg.String{Name: "deployment-key"}),
		},
		Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			*got = *lc
			return next(ctx)
		}),
	}
	root := &xli.Command{
		Name:     "app",
		Flags:    flg.Flags{cfg.ConfigFlag()},
		Commands: xli.Commands{{Name: "ldap", Commands: xli.Commands{serve}}, cfg.NewCmdConfig(l)},
		Handler:  xli.Chain(cfg.Load(l), xli.RequireSubcommand()),
	}
	return root, l
}

func TestBind(t *testing.T) {
	t.Run("flags of the subcommand override the file and the environment", x.F(func(x x.X) {
		p := write(x.T, "ldap:\n  addr: file\n  bind: password\n  bases: {a: from-file}\n")
		got := ServeConfig{}
		c := AppConfig{}
		root, l := app(&c, env("APP_LDAP_ADDR=env", "APP_LDAP_INSECURE=true"), &got)

		res := xlitest.Run(x.T, root, "--config", p, "ldap", "serve",
			"--listen=flag", "--tls=c.pem,k.pem", "--base", "x=dc=x", "--base", "y=dc=y", "--deployment-key=lit")
		x.NoError(res.Err)

		x.Equal("flag", got.Addr)
		x.True(got.Insecure, "from the environment; the flag was not given")
		x.Equal("password", got.Bind, "from the file")
		x.Equal(TlsConfig{Cert: "c.pem", Key: "k.pem"}, got.Tls)
		x.Equal(map[string]string{"x": "dc=x", "y": "dc=y"}, got.Bases)
		key, _ := got.Key.Value()
		x.Equal("lit", key)

		o, _ := l.Origin(&c.Ldap.Addr)
		x.Equal(cfg.Flag, o.Source)
		x.Equal("--listen", o.Name)
		o, _ = l.Origin(&c.Ldap.Insecure)
		x.Equal(cfg.Env, o.Source)
		o, _ = l.Origin(&c.Ldap.Bind)
		x.Equal(cfg.File, o.Source)
	}))
	t.Run("a flag's default is the lowest layer", x.F(func(x x.X) {
		got := ServeConfig{}
		c := AppConfig{}
		root, l := app(&c, nil, &got)
		res := xlitest.Run(x.T, root, "ldap", "serve")
		x.NoError(res.Err)
		x.Equal(":389", got.Addr)

		o, _ := l.Origin(&c.Ldap.Addr)
		x.Equal(cfg.Default, o.Source)

		root, _ = app(&AppConfig{}, env("APP_LDAP_ADDR=env"), &got)
		x.NoError(xlitest.Run(x.T, root, "ldap", "serve").Err)
		x.Equal("env", got.Addr, "the environment is over the default")
	}))
	t.Run("an empty flag clears a text field", x.F(func(x x.X) {
		p := write(x.T, "ldap:\n  key: from-file\n")
		got := ServeConfig{}
		root, _ := app(&AppConfig{}, nil, &got)
		x.NoError(xlitest.Run(x.T, root, "--config", p, "ldap", "serve", "--deployment-key=").Err)
		x.True(got.Key.IsZero())
	}))
	t.Run("a secret flag reads references", x.F(func(x x.X) {
		got := ServeConfig{}
		root, _ := app(&AppConfig{}, env("KEY=from-env"), &got)
		x.NoError(xlitest.Run(x.T, root, "ldap", "serve", "--deployment-key=${env:KEY}").Err)
		key, _ := got.Key.Value()
		x.Equal("from-env", key)

		x.NoError(xlitest.Run(x.T, root, "ldap", "serve", "--deployment-key=Pa$$${w0rd").Err)
		key, _ = got.Key.Value()
		x.Equal("Pa$$${w0rd", key, "anything else is taken as it is")
	}))
	t.Run("conversion errors name the flag", x.F(func(x x.X) {
		got := ServeConfig{}
		root, _ := app(&AppConfig{}, nil, &got)
		res := xlitest.Run(x.T, root, "ldap", "serve", "--tls=oops", "--base=nope")
		x.ErrorContains(res.Err, "--tls: want cert.pem,key.pem")
		x.ErrorContains(res.Err, `--base: "nope": want alias=suffix`)
	}))
	t.Run("help shows the variable", x.F(func(x x.X) {
		root, _ := app(&AppConfig{}, nil, &ServeConfig{})
		res := xlitest.Run(x.T, root, "ldap", "serve", "-h")
		x.NoError(res.Err)
		x.Contains(res.Stdout, `where to listen (default: ":389") [$APP_LDAP_ADDR]`)
		x.Contains(res.Stdout, "--base string...   [$APP_LDAP_BASES]")
		x.NotContains(res.Stdout, "APP_LDAP_TLS", "a block has no one variable")
	}))
	t.Run("help does not load the configuration", x.F(func(x x.X) {
		root, l := app(&AppConfig{}, nil, &ServeConfig{})
		res := xlitest.Run(x.T, root, "--config", "/does/not/exist", "ldap", "serve", "-h")
		x.NoError(res.Err)
		x.Nil(l.Current())
	}))
	t.Run("a bound flag reads like any other", x.F(func(x x.X) {
		c := AppConfig{}
		l := cfg.New("app", &c)
		def := "d"
		f := cfg.Bind(l, &c.Ldap.Addr, &flg.String{Name: "listen", Default: &def})
		cmd := &xli.Command{Name: "x", Flags: flg.Flags{}.WithCategory("net", f)}

		x.Equal("d", flg.MustGet[string](cmd, "listen"), "MustGet falls back to the default")
		_, ok := flg.Get[string](cmd, "listen")
		x.False(ok)
		x.Equal("net", flg.Unwrap(f).Info().Category, "WithCategory reaches the flag")
		x.Equal("APP_LDAP_ADDR", f.Info().Env)
	}))
	t.Run("two flags bound to one field", x.F(func(x x.X) {
		c := AppConfig{}
		l := cfg.New("app", &c, cfg.WithPaths())
		root := &xli.Command{
			Name:  "app",
			Flags: flg.Flags{cfg.Bind(l, &c.Ldap.Addr, &flg.String{Name: "a"})},
			Commands: xli.Commands{{
				Name:  "sub",
				Flags: flg.Flags{cfg.Bind(l, &c.Ldap.Addr, &flg.String{Name: "b"})},
			}},
			Handler: cfg.Load(l),
		}
		res := xlitest.Run(x.T, root, "sub")
		x.ErrorContains(res.Err, "--a and --b are both bound to ldap.addr")
	}))
	t.Run("a flag bound to a block sets it whole", x.F(func(x x.X) {
		p := write(x.T, "ldap:\n  tls: {cert: file.pem, key: file.key}\n")
		got := ServeConfig{}
		c := AppConfig{}
		root, l := app(&c, nil, &got)
		x.NoError(xlitest.Run(x.T, root, "--config", p, "ldap", "serve", "--tls=c.pem,k.pem").Err)
		x.Equal(TlsConfig{Cert: "c.pem", Key: "k.pem"}, got.Tls)

		o, _ := l.Origin(&c.Ldap.Tls.Key)
		x.Equal(cfg.Flag, o.Source)
		x.Equal("--tls", o.Name)
		x.Equal("ldap.tls.key", o.Key)
	}))
	t.Run("a block and a field in it are both bound", x.F(func(x x.X) {
		c := AppConfig{}
		l := cfg.New("app", &c, cfg.WithPaths())
		root := &xli.Command{
			Name: "app",
			Flags: flg.Flags{
				cfg.BindFunc(l, &c.Ldap.Tls, &flg.String{Name: "tls"}, parseCertPair),
				cfg.Bind(l, &c.Ldap.Tls.Cert, &flg.String{Name: "cert"}),
			},
			Handler: cfg.Load(l),
		}
		res := xlitest.Run(x.T, root)
		x.ErrorContains(res.Err, "--tls and --cert are both bound to ldap.tls.cert")
	}))
	t.Run("a flag not given does not make the block it is bound into", x.F(func(x x.X) {
		type C struct {
			Ldap *ServeConfig `yaml:"ldap"`
		}
		c := C{}
		l := cfg.New("app", &c, cfg.WithPaths())
		// Made after the loader, so that the default stays nil.
		c.Ldap = &ServeConfig{}
		f := cfg.Bind(l, &c.Ldap.Addr, &flg.String{Name: "listen"})

		_, err := l.Load("", nil, f)
		x.NoError(err)
		x.Nil(c.Ldap)
	}))
	t.Run("an empty flag clears", x.F(func(x x.X) {
		p := write(x.T, "ldap:\n  addr: file\n")
		c := AppConfig{}
		l := cfg.New("app", &c, cfg.WithPaths())
		addr := cfg.Bind(l, &c.Ldap.Addr, &flg.String{Name: "listen"})
		root := &xli.Command{Name: "app", Flags: flg.Flags{addr}, Handler: cfg.Load(l)}
		x.NoError(xlitest.Run(x.T, root, "--listen=").Err)
		_, err := l.Load(p, nil, addr)
		x.NoError(err)
		x.Equal("", c.Ldap.Addr)
		o, _ := l.Origin(&c.Ldap.Addr)
		x.Equal(cfg.Flag, o.Source)
		x.True(o.Cleared)
	}))
	t.Run("an empty list flag clears", x.F(func(x x.X) {
		type C struct {
			Hosts []string `yaml:"hosts"`
		}
		c := C{Hosts: []string{"default"}}
		l := cfg.New("app", &c, cfg.WithPaths())
		hosts := cfg.Bind(l, &c.Hosts, &flg.Strings{Name: "host"})
		root := &xli.Command{Name: "app", Flags: flg.Flags{hosts}, Handler: cfg.Load(l)}
		x.NoError(xlitest.Run(x.T, root, "--host=").Err)
		x.Nil(c.Hosts)
	}))
	t.Run("the configuration's list is not the flag's", x.F(func(x x.X) {
		type C struct {
			Hosts []string `yaml:"hosts"`
		}
		c := C{}
		l := cfg.New("app", &c, cfg.WithPaths())
		f := &flg.Strings{Name: "host"}
		root := &xli.Command{Name: "app", Flags: flg.Flags{cfg.Bind(l, &c.Hosts, f)}, Handler: cfg.Load(l)}
		x.NoError(xlitest.Run(x.T, root, "--host=a", "--host=b").Err)
		c.Hosts[0] = "changed"
		v, _ := f.Get()
		x.Equal([]string{"a", "b"}, v)
	}))
	t.Run("a default whose secret file is not there yet", x.F(func(x x.X) {
		missing := "${file:/does/not/exist}"
		c := AppConfig{}
		l := cfg.New("app", &c, cfg.WithPaths())
		f := cfg.BindText(l, &c.Ldap.Key, &flg.String{Name: "key", Default: &missing})
		s, err := l.Load("", nil, f)
		x.NoError(err)
		x.Len(s.Warnings, 1)
		o, _ := l.Origin(&c.Ldap.Key)
		x.Equal(cfg.Default, o.Source)
		x.Equal([]string{missing}, o.Refs)
	}))
	t.Run("binding to something that is not a field panics", x.F(func(x x.X) {
		c := AppConfig{}
		l := cfg.New("app", &c)
		other := ""
		defer func() { x.NotNil(recover()) }()
		cfg.Bind(l, &other, &flg.String{Name: "x"})
	}))
	t.Run("secret files not there yet are reported", x.F(func(x x.X) {
		root, _ := app(&AppConfig{}, env("APP_LDAP_KEY=${file:/does/not/exist}"), &ServeConfig{})
		res := xlitest.Run(x.T, root, "ldap", "serve")
		x.NoError(res.Err)
		x.Contains(res.Stderr, "app: APP_LDAP_KEY: secret file /does/not/exist: ")
		x.Contains(res.Stderr, "; it is read again when used\n")
	}))
	t.Run("unknown variables are reported", x.F(func(x x.X) {
		root, _ := app(&AppConfig{}, env("APP_LDAP_ADRR=x"), &ServeConfig{})
		res := xlitest.Run(x.T, root, "ldap", "serve")
		x.NoError(res.Err)
		x.Contains(res.Stderr, `app: APP_LDAP_ADRR is set and nothing reads it (did you mean "APP_LDAP_ADDR"?)`)
	}))
}
