package cfg

import (
	"reflect"
	"testing"

	"github.com/lesomnus/xli/internal/x"
)

func keysAndEnvs(s *schema) (keys []string, envs []string) {
	for _, f := range s.fields {
		keys = append(keys, f.key)
		envs = append(envs, f.env)
	}
	return keys, envs
}

func TestSchemaNames(t *testing.T) {
	type Inner struct {
		Addr string `yaml:"addr"`
	}
	type Embedded struct {
		Region string
	}
	type C struct {
		ByCfg      string `cfg:"by_cfg" yaml:"ignored_a" json:"ignored_a"`
		ByYaml     string `yaml:"by_yaml" json:"ignored_b"`
		ByJson     string `json:"by_json"`
		ListenAddr string
		AddrTLS    string
		TLSConfig  string
		Skipped    string `yaml:"-"`
		Kept       string `cfg:"kept" yaml:"-"`
		Override   string `env:"CUSTOM_NAME"`
		NoEnv      string `env:"-"`
		Inner      Inner  `yaml:"inner"`
		Ptr        *Inner `yaml:"ptr"`
		Flat       Inner  `yaml:",inline"`
		Embedded
		unexported string
	}

	s, err := newSchema(reflect.TypeFor[C](), "APP")
	x := x.New(t)
	x.NoError(err)

	keys, envs := keysAndEnvs(s)
	x.Equal([]string{
		"by_cfg", "by_yaml", "by_json", "listen_addr", "addr_tls", "tls_config",
		"kept", "override", "no_env", "inner.addr", "ptr.addr", "addr", "region",
	}, keys)
	x.Equal([]string{
		"APP_BY_CFG", "APP_BY_YAML", "APP_BY_JSON", "APP_LISTEN_ADDR", "APP_ADDR_TLS", "APP_TLS_CONFIG",
		"APP_KEPT", "CUSTOM_NAME", "", "APP_INNER_ADDR", "APP_PTR_ADDR", "APP_ADDR", "APP_REGION",
	}, envs)
}

func TestSnake(t *testing.T) {
	x := x.New(t)
	for in, want := range map[string]string{
		"Addr":       "addr",
		"ListenAddr": "listen_addr",
		"AddrTLS":    "addr_tls",
		"TLSConfig":  "tls_config",
		"HTTP2Port":  "http2_port",
		"ID":         "id",
	} {
		x.Equal(want, snake(in), in)
	}
}

func TestEnvPrefix(t *testing.T) {
	x := x.New(t)
	x.Equal("GO_APP", envPrefix("go-app"))
	x.Equal("GO_APP", envPrefix("go.app"))
	x.Equal("APP2", envPrefix("App2"))
}

func TestSchemaErrors(t *testing.T) {
	t.Run("two fields with one variable", x.F(func(x x.X) {
		type C struct {
			A struct {
				B string `yaml:"b"`
			} `yaml:"a"`
			AB string `yaml:"a_b"`
		}
		_, err := newSchema(reflect.TypeFor[C](), "APP")
		x.ErrorContains(err, "are both read from APP_A_B")
	}))
	t.Run("two fields with one name", x.F(func(x x.X) {
		type C struct {
			A string `yaml:"a"`
			B string `yaml:"a" env:"B"`
		}
		_, err := newSchema(reflect.TypeFor[C](), "APP")
		x.ErrorContains(err, `two fields are named "a"`)
	}))
	t.Run("two inlined structs with one block", x.F(func(x x.X) {
		type A struct {
			Db struct {
				Dsn string `yaml:"dsn"`
			} `yaml:"db"`
		}
		type B struct {
			Db struct {
				MaxConn int `yaml:"max_conn"`
			} `yaml:"db"`
		}
		type C struct {
			A `yaml:",inline"`
			B `yaml:",inline"`
		}
		_, err := newSchema(reflect.TypeFor[C](), "APP")
		x.ErrorContains(err, `two fields are named "db"`)
	}))
	t.Run("a field and a block with one name", x.F(func(x x.X) {
		type A struct {
			Db string `yaml:"db"`
		}
		type C struct {
			A  `yaml:",inline"`
			Db struct {
				Dsn string `yaml:"dsn"`
			} `yaml:"db"`
		}
		_, err := newSchema(reflect.TypeFor[C](), "APP")
		x.ErrorContains(err, `two fields are named "db"`)
	}))
	t.Run("a name a file cannot give", x.F(func(x x.X) {
		type C struct {
			Key string `yaml:"x-api-key"`
		}
		_, err := newSchema(reflect.TypeFor[C](), "APP")
		x.ErrorContains(err, `"x-api-key" is never read from a file`)
	}))
	t.Run("an environment variable that is not a name", x.F(func(x x.X) {
		type C struct {
			Dsn string `env:"DB_DSN,required"`
		}
		_, err := newSchema(reflect.TypeFor[C](), "APP")
		x.ErrorContains(err, `env:"DB_DSN,required" is not a variable name`)
	}))
	t.Run("a pointer to a pointer", x.F(func(x x.X) {
		type C struct {
			P **struct{ A string }
		}
		_, err := newSchema(reflect.TypeFor[C](), "APP")
		x.ErrorContains(err, "a pointer to a pointer")
	}))
	t.Run("a recursive type", x.F(func(x x.X) {
		type Node struct {
			Next *Node
		}
		_, err := newSchema(reflect.TypeFor[Node](), "APP")
		x.ErrorContains(err, "refers to itself")
	}))
	t.Run("not a struct", x.F(func(x x.X) {
		_, err := newSchema(reflect.TypeFor[int](), "APP")
		x.ErrorContains(err, "must be a struct")
	}))
}

func TestSchemaLeaves(t *testing.T) {
	type C struct {
		Secret   Secret            `yaml:"secret"`
		List     []string          `yaml:"list"`
		Map      map[string]string `yaml:"map"`
		Duration *int              `yaml:"duration"`
		Token    string            `cfg:",secret" yaml:"token"`
	}
	s, err := newSchema(reflect.TypeFor[C](), "APP")
	x := x.New(t)
	x.NoError(err)

	keys, _ := keysAndEnvs(s)
	x.Equal([]string{"secret", "list", "map", "duration", "token"}, keys)
	x.True(s.byKey["secret"].secret)
	x.True(s.byKey["token"].secret)
	x.False(s.byKey["list"].secret)
}

func TestSchemaFind(t *testing.T) {
	type Inner struct {
		A string `yaml:"a"`
	}
	type C struct {
		Inner Inner  `yaml:"inner"`
		Ptr   *Inner `yaml:"ptr"`
		B     int    `yaml:"b"`
	}
	s, err := newSchema(reflect.TypeFor[C](), "APP")
	x := x.New(t)
	x.NoError(err)

	c := &C{}
	root := reflect.ValueOf(c).Elem()

	f, ok := s.find(root, &c.Inner.A)
	x.True(ok)
	x.Equal("inner.a", f.key)

	f, ok = s.find(root, &c.B)
	x.True(ok)
	x.Equal("b", f.key)

	_, ok = s.find(root, &c.Inner)
	x.False(ok, "not a leaf")

	copied := c.Inner
	_, ok = s.find(root, &copied.A)
	x.False(ok, "a field of a copy")

	c.Ptr = &Inner{}
	f, ok = s.find(root, &c.Ptr.A)
	x.True(ok)
	x.Equal("ptr.a", f.key)
}
