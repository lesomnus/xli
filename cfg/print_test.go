package cfg_test

import (
	"io"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/lesomnus/xli/cfg"
	"github.com/lesomnus/xli/internal/x"
)

func printed(t *testing.T, s interface{ Print(w io.Writer) error }) string {
	t.Helper()
	b := &strings.Builder{}
	if err := s.Print(b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestPrintRedacts(t *testing.T) {
	type Backend struct {
		Name     string `yaml:"name"`
		Password string `cfg:",secret"`
		Hidden   string `cfg:"-"`
		Token    string `cfg:"token" yaml:"tok"`
	}
	type Client struct {
		Addr         string `yaml:"addr"`
		Token        string `yaml:"token"`
		ClientSecret string `yaml:"client_secret"`
		KeyFile      string `yaml:"key_file"`
	}
	type C struct {
		Backends []Backend         `yaml:"backends"`
		Client   Client            `yaml:"client"`
		Keys     map[string]string `yaml:"keys"`
		Seed     map[string]string `yaml:"seed"`
		Dsn      string            `yaml:"dsn"`
		DevDsn   string            `yaml:"dev_dsn"`
		Pg       struct {
			Dsn string `yaml:"dsn"`
		} `yaml:"pg"`
		Creds struct {
			User string `yaml:"user"`
		} `cfg:",secret" yaml:"creds"`
	}
	p := write(t, `
backends: [{name: a, password: hunter2, token: rt_LISTSECRET}]
client:
  addr: roster:8080
  token: rt_REVIEWSECRET
  client_secret: oidc_SECRET
  key_file: /etc/tls/key.pem
keys: {contoso: rt_ALSOSECRET}
seed: {password: admin123, holder: admin}
dsn: postgres://khala:PGSECRET@khala-db:5432/khala?sslmode=disable
dev_dsn: postgres://khala:DEVSECRET@khala-dev:5432/dev
pg:
  dsn: host=db user=khala password=PGSECRET2 dbname=khala
creds: {user: admin_SECRET}
`)
	c := C{}
	s, err := cfg.New("app", &c).Load(p, nil)
	x := x.New(t)
	x.NoError(err)
	c.Backends[0].Hidden = "hidden_VALUE"

	out := printed(t, s)
	for _, secret := range []string{"hunter2", "rt_LISTSECRET", "rt_REVIEWSECRET", "oidc_SECRET", "rt_ALSOSECRET", "admin123", "PGSECRET", "PGSECRET2", "DEVSECRET", "admin_SECRET", "hidden_VALUE"} {
		x.NotContains(out, secret)
	}
	for _, kept := range []string{"roster:8080", "/etc/tls/key.pem", "holder: admin", "khala-db:5432", "khala-dev:5432", "user=khala", "token: <redacted>", "{name: a, password: <redacted>, token: <redacted>}"} {
		x.Contains(out, kept)
	}
}

func TestPrintAsWritten(t *testing.T) {
	type C struct {
		Dsn      string   `yaml:"dsn"`
		Password string   `yaml:"password"`
		Hosts    []string `yaml:"hosts"`
	}
	p := write(t, "dsn: postgres://u:${env:PW}@h/db\npassword: \"abc${env:PW}\"\nhosts: [\"${env:H}\", b]\n")
	c := C{}
	s, err := cfg.New("app", &c).Load(p, env("PW=s3cret", "H=h"))
	x := x.New(t)
	x.NoError(err)

	out := printed(t, s)
	x.NotContains(out, "s3cret")
	x.Contains(out, "dsn: postgres://u:${env:PW}@h/db  #")
	x.Contains(out, "password: <redacted>  #", "a secret written with more than a reference")
	x.Contains(out, `hosts: ["${env:H}", b]  #`)
}

func TestPrintReadsBack(t *testing.T) {
	type Inner struct {
		A string `yaml:"a"`
	}
	type C struct {
		Multi    string            `yaml:"multi"`
		Dollar   string            `yaml:"dollar"`
		Tab      string            `yaml:"tab"`
		Spaced   string            `yaml:"spaced"`
		Looks    string            `yaml:"looks"`
		Any      any               `yaml:"any"`
		Wait     time.Duration     `yaml:"wait"`
		Inf      float64           `yaml:"inf"`
		Whole    float64           `yaml:"whole"`
		Bytes    []byte            `yaml:"bytes"`
		Map      map[string]string `yaml:"map"`
		Gone     *Inner            `yaml:"gone"`
		Empty    *Inner            `yaml:"empty"`
		Inner    Inner             `yaml:"inner"`
		Nums     []int             `yaml:"nums"`
		Anything map[string]any    `yaml:"anything"`
	}
	c := C{
		Multi:    "line1\nline2",
		Tab:      "a\tb",
		Spaced:   " x ",
		Looks:    "true",
		Any:      "010",
		Wait:     90 * time.Second,
		Inf:      math.Inf(1),
		Whole:    1,
		Bytes:    []byte{0, 1, 2},
		Map:      map[string]string{"a b": "c: d", "#": "[x]", "k": ""},
		Gone:     &Inner{A: "default"},
		Empty:    &Inner{},
		Inner:    Inner{A: "${not a reference}"},
		Nums:     []int{1, 2},
		Anything: map[string]any{"n": uint64(1), "l": []any{"x", true}},
	}
	l := cfg.New("app", &c, cfg.WithPaths())
	s, err := l.Load(write(t, "gone: null\n"), env("APP_DOLLAR=a$$b ${c}"))
	x := x.New(t)
	x.NoError(err)
	want := *s.Config

	out := printed(t, s)
	x.Contains(out, "gone: null  #", "a block that is not there")
	x.Contains(out, "empty: {}")

	got := C{}
	_, err = cfg.New("app", &got).Load(write(t, out), nil)
	x.NoError(err, out)
	x.Equal(want, got, out)
}

func TestPrintLeavesOutBlocksWithNothingInThem(t *testing.T) {
	type Tls struct {
		Cert string `yaml:"cert"`
	}
	type Client struct {
		Addr string `yaml:"addr"`
		Tls  Tls    `yaml:"tls"`
	}
	type C struct {
		Name   string `yaml:"name"`
		Client Client `yaml:"client"`
	}
	c := C{Client: Client{Addr: "default"}}
	s, err := cfg.New("app", &c, cfg.WithPaths()).Load("", env("APP_NAME=n", "APP_CLIENT_ADDR="))
	x := x.New(t)
	x.NoError(err)
	x.Equal("name: n  # APP_NAME\nclient:\n  addr: \"\"  # APP_CLIENT_ADDR (cleared)\n", printed(t, s))

	c = C{}
	s, err = cfg.New("app", &c, cfg.WithPaths()).Load("", env("APP_NAME=n"))
	x.NoError(err)
	x.Equal("name: n  # APP_NAME\n", printed(t, s), "not client: alone, which reads back as null")
}

// UnimplementedExporter is a type embedded for its methods, as mkot's
// UnimplementedExporterConfig is: it holds nothing.
type UnimplementedExporter struct{}

func (UnimplementedExporter) Export() error { return nil }

type OtlpExporter struct {
	UnimplementedExporter
	Endpoint string `yaml:"endpoint"`
}

// Telemetry decodes itself, as mkot's Config does.
type Telemetry struct {
	UnimplementedExporter
	Exporters map[string]*OtlpExporter `yaml:"exporters"`
}

func (t *Telemetry) UnmarshalYAML(b []byte) error {
	type plain Telemetry
	return yaml.UnmarshalWithOptions(b, (*plain)(t), yaml.Strict())
}

// TestPrintLeavesOutWhatIsEmbeddedForItsMethods is a type that decodes itself,
// printed as it marshals -- but for the embedded structs that hold nothing,
// which goccy writes as keys of their own (`unimplementedexporter: {}`). They
// say nothing, and an exporter with nothing set is kept under its name: it says
// the exporter is there.
func TestPrintLeavesOutWhatIsEmbeddedForItsMethods(t *testing.T) {
	type C struct {
		Otel Telemetry `yaml:"otel"`
	}
	x := x.New(t)
	p := write(t, `
otel:
  exporters:
    otlp: {endpoint: "collector:4317"}
    pretty: {}
`)
	c := &C{}
	s, err := cfg.New("app", c).Load(p, nil)
	x.NoError(err)

	out := printed(t, s)
	x.NotContains(out, "unimplementedexporter")
	x.Contains(out, "otlp: {endpoint: ")
	x.Contains(out, "pretty: {endpoint: \"\"}")

	// And it reads back to the same configuration.
	back := &C{}
	_, err = cfg.New("app", back).Load(write(t, out), nil)
	x.NoError(err)
	x.Equal("collector:4317", back.Otel.Exporters["otlp"].Endpoint)
	x.NotNil(back.Otel.Exporters["pretty"])
}
