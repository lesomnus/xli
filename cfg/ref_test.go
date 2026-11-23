package cfg_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lesomnus/xli/cfg"
	"github.com/lesomnus/xli/cfg/ref"
	"github.com/lesomnus/xli/internal/x"
)

type Tls struct {
	Cert ref.String `yaml:"cert"`
	Ca   ref.Bytes  `yaml:"ca"`
	Key  ref.String `yaml:"key"`
}

func loadTls(t *testing.T, content string, environ ...string) (*Tls, *cfg.Snapshot[Tls], error) {
	t.Helper()
	c := &Tls{}
	s, err := cfg.New("app", c).Load(write(t, content), environ)
	return c, s, err
}

// TestRef is a value that is no secret read as a secret is: a literal, or a
// reference to the environment, a file or a scheme.
func TestRef(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tls.crt")

	t.Run("a literal", x.F(func(x x.X) {
		c, _, err := loadTls(x.T, "cert: \"PEM costs $$5\"\n")
		x.NoError(err)
		v, err := c.Cert.Value()
		x.NoError(err)
		x.Equal("PEM costs $5", v)
		x.Equal("", c.Cert.Ref())
		x.Equal("PEM costs $5", c.Cert.String(), "no secret: a literal is itself")

		_, _, err = loadTls(x.T, "cert: \"a${env:X}\"\n", "X=1")
		x.ErrorContains(err, "cert: a value is a literal or exactly one reference")
	}))
	t.Run("from the environment", x.F(func(x x.X) {
		c, _, err := loadTls(x.T, "cert: ${env:CERT}\n", "CERT=pem\n")
		x.NoError(err)
		v, _ := c.Cert.Value()
		x.Equal("pem", v)
		x.Equal("${env:CERT}", c.Cert.String())

		c, _, err = loadTls(x.T, "", "APP_CERT=Pa$$w0rd")
		x.NoError(err)
		v, _ = c.Cert.Value()
		x.Equal("Pa$$w0rd", v, "the environment's value is taken as it is")
	}))
	t.Run("from a file, again when it is rotated", x.F(func(x x.X) {
		writeAt(x.T, path, "one\n")
		c, _, err := loadTls(x.T, "cert: ${file:"+path+"}\n")
		x.NoError(err)
		v, err := c.Cert.Value()
		x.NoError(err)
		x.Equal("one", v)

		rotate(x.T, path, "two\n")
		v, err = c.Cert.Value()
		x.NoError(err)
		x.Equal("two", v)

		c, _, err = loadTls(x.T, "", "APP_CERT=${file:"+path+"}")
		x.NoError(err)
		v, _ = c.Cert.Value()
		x.Equal("two", v, "a reference from the environment")
	}))
	t.Run("a file not there yet is a warning", x.F(func(x x.X) {
		late := filepath.Join(dir, "late")
		c, s, err := loadTls(x.T, "cert: ${file:"+late+"}\n")
		x.NoError(err)
		x.Len(s.Warnings, 1)
		x.ErrorContains(s.Warnings[0], "cert: file "+late)

		writeAt(x.T, late, "minted\n")
		v, err := c.Cert.Value()
		x.NoError(err)
		x.Equal("minted", v)
	}))
	t.Run("from a scheme", x.F(func(x x.X) {
		c := &Tls{}
		l := cfg.New("app", c, cfg.WithScheme("vault", func(ref string) ([]byte, error) {
			if ref != "pki#ca" {
				return nil, errors.New("no such value")
			}
			return []byte("from-vault"), nil
		}))
		_, err := l.Load(write(x.T, "ca: ${vault:pki#ca}\n"), nil)
		x.NoError(err)
		v, _ := c.Ca.Value()
		x.Equal([]byte("from-vault"), v)

		o, _ := l.Origin(&c.Ca)
		x.Equal([]string{"${vault:pki#ca}"}, o.Refs)

		_, err = l.Load(write(x.T, "ca: ${vault:other}\n"), nil)
		x.ErrorContains(err, "ca: ${vault:other}: no such value")
	}))
}

// TestRefFileCap is how much of a file is read: a secret keeps its 64 KiB, and
// a ref.Of reads up to 1 MiB, for a CA bundle of a system is some 200 KiB.
func TestRefFileCap(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "ca-bundle.crt")
	writeAt(t, bundle, strings.Repeat("a", 200<<10))

	t.Run("a ref.Bytes reads it", x.F(func(x x.X) {
		c, s, err := loadTls(x.T, "ca: ${file:"+bundle+"}\n")
		x.NoError(err)
		x.Len(s.Warnings, 0)
		v, err := c.Ca.Value()
		x.NoError(err)
		x.Len(v, 200<<10)
	}))
	t.Run("a SecretBytes refuses it", x.F(func(x x.X) {
		c := &Secrets{}
		s, err := cfg.New("app", c).Load(write(x.T, "raw: ${file:"+bundle+"}\n"), nil)
		x.NoError(err)
		x.Len(s.Warnings, 1)
		x.ErrorContains(s.Warnings[0], "raw: secret file "+bundle+": over the 65536 byte cap (64 KiB)")
		_, err = c.Raw.Value()
		x.ErrorContains(err, "64 KiB")
	}))
	t.Run("a ref.Bytes refuses over 1 MiB", x.F(func(x x.X) {
		huge := filepath.Join(dir, "huge")
		writeAt(x.T, huge, strings.Repeat("a", 1<<20+1))
		_, s, err := loadTls(x.T, "ca: ${file:"+huge+"}\n")
		x.NoError(err)
		x.Len(s.Warnings, 1)
		x.ErrorContains(s.Warnings[0], "ca: file "+huge+": over the 1048576 byte cap (1 MiB)")
	}))
}

// TestPrintRef is what config prints of a ref.Of: its reference as written,
// as a secret does, and a literal as itself unless the field would be redacted
// anyway.
func TestPrintRef(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tls.crt")
	writeAt(t, path, "PEM\n")
	type C struct {
		Tls      Tls        `yaml:"tls"`
		Tagged   ref.String `cfg:",secret" yaml:"tagged"`
		Password cfg.Secret `yaml:"password"`
		Ca       *ref.Bytes `yaml:"ca"`
	}
	c := C{}
	s, err := cfg.New("app", &c).Load(write(t, `
tls:
  cert: ${file:`+path+`}
  ca: "-----BEGIN CERTIFICATE-----\nMIIB$$\n"
  key: KEY_SECRET
tagged: TAGGED_SECRET
password: hunter2
`), env("APP_CA=bundle"))
	x := x.New(t)
	x.NoError(err)

	out := printed(t, s)
	for _, secret := range []string{"KEY_SECRET", "TAGGED_SECRET", "hunter2"} {
		x.NotContains(out, secret)
	}
	x.Contains(out, "cert: ${file:"+path+"}  #", "a reference as written")
	x.Contains(out, `ca: "-----BEGIN CERTIFICATE-----\nMIIB$$\n"  #`, "a literal as itself")
	x.Contains(out, "key: <redacted>  #", "a name that says secret")
	x.Contains(out, "tagged: <redacted>  #", "a tag that says secret")
	x.Contains(out, "password: <redacted>  #", "a Secret as before")
	x.Contains(out, "ca: bundle  #")
}

// TestRefFileOnly is where a file may be read: into a ref.Of or a secret, and
// nowhere else.
func TestRefFileOnly(t *testing.T) {
	type C struct {
		Cert string `yaml:"cert"`
	}
	_, err := cfg.New("app", &C{}).Load(write(t, "cert: ${file:/tls/tls.crt}\n"), nil)
	x := x.New(t)
	x.ErrorContains(err, "cert: ${file:/tls/tls.crt}: a file is only read into a ref.String or a cfg.Secret field, which read it again when it changes")
}
