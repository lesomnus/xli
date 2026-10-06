package cfg_test

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/lesomnus/xli/cfg"
	"github.com/lesomnus/xli/internal/x"
)

// Key is a custom secret: a base64-encoded 4-byte key.
type Key = cfg.SecretOf[[4]byte, KeyDecoder]

type KeyDecoder struct{}

func (KeyDecoder) Decode(raw []byte) ([4]byte, error) {
	var k [4]byte
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return k, err
	}
	if len(b) != 4 {
		return k, fmt.Errorf("want 4 bytes, got %d", len(b))
	}
	copy(k[:], b)
	return k, nil
}

type Secrets struct {
	Password cfg.Secret      `yaml:"password"`
	Raw      cfg.SecretBytes `yaml:"raw"`
	Key      Key             `yaml:"key"`
	Token    string          `cfg:",secret" yaml:"token"`
}

func writeAt(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// rotate replaces the file at path the way a rotation does: a new file
// renamed into place.
func rotate(t *testing.T, path, content string) {
	t.Helper()
	tmp := path + ".new"
	writeAt(t, tmp, content)
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

func loadSecrets(t *testing.T, content string, environ ...string) (*Secrets, error) {
	t.Helper()
	c := &Secrets{}
	_, err := cfg.New("app", c).Load(write(t, content), environ)
	return c, err
}

func TestSecretLiteral(t *testing.T) {
	x := x.New(t)
	c, err := loadSecrets(t, "password: hunter2\n")
	x.NoError(err)

	v, err := c.Password.Value()
	x.NoError(err)
	x.Equal("hunter2", v)
	x.False(c.Password.IsZero())
	x.Equal("", c.Password.Ref())
	x.Equal("<redacted>", c.Password.String())
	x.Equal("<redacted>", fmt.Sprint(c.Password))

	b, err := yaml.Marshal(c.Password)
	x.NoError(err)
	x.Equal("<redacted>\n", string(b), "encoders never write a secret out")

	x.True(c.Raw.IsZero())
	x.Equal("", c.Raw.String())
}

func TestSecretEscapes(t *testing.T) {
	x := x.New(t)
	c, err := loadSecrets(t, "password: \"$${file:/x} costs $$5\"\n")
	x.NoError(err)
	v, _ := c.Password.Value()
	x.Equal("${file:/x} costs $5", v)

	_, err = loadSecrets(t, "password: \"a${env:X}\"\n", "X=1")
	x.ErrorContains(err, "a secret is a literal or exactly one reference")
}

func TestSecretEnv(t *testing.T) {
	x := x.New(t)
	c, err := loadSecrets(t, "password: ${env:PW}\nraw: ${env:NOPE:-dflt}\n", "PW=from-env\n")
	x.NoError(err)
	v, _ := c.Password.Value()
	x.Equal("from-env", v, "one trailing newline is dropped")
	x.Equal("${env:PW}", c.Password.String(), "a reference prints as written")
	r, _ := c.Raw.Value()
	x.Equal([]byte("dflt"), r)

	_, err = loadSecrets(t, "password: ${env:NOPE}\n")
	x.ErrorContains(err, "NOPE is not set")
}

func TestSecretFromEnvironment(t *testing.T) {
	x := x.New(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "pw")
	writeAt(t, path, "from-file\n")

	c, err := loadSecrets(t, "password: literal\n", "APP_PASSWORD=${file:"+path+"}")
	x.NoError(err, "secret fields read references from any source")
	v, _ := c.Password.Value()
	x.Equal("from-file", v)

	c, err = loadSecrets(t, "password: literal\n", "APP_PASSWORD=")
	x.NoError(err)
	x.True(c.Password.IsZero(), "an empty variable clears a secret")
}

func TestSecretFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pw")

	t.Run("read at load and when rotated", x.F(func(x x.X) {
		writeAt(x.T, path, "one\n")
		c, err := loadSecrets(x.T, "password: ${file:"+path+"}\n")
		x.NoError(err)
		v, err := c.Password.Value()
		x.NoError(err)
		x.Equal("one", v)
		x.Equal("${file:"+path+"}", c.Password.String())

		rotate(x.T, path, "two\n")
		v, err = c.Password.Value()
		x.NoError(err)
		x.Equal("two", v)

		// The same length, rewritten in place: caught by the new file
		// identity a rename gives, or by size and time otherwise.
		rotate(x.T, path, "TWO\n")
		v, _ = c.Password.Value()
		x.Equal("TWO", v)
	}))
	t.Run("only the trailing newline is dropped", x.F(func(x x.X) {
		writeAt(x.T, path, " spaced \r\n")
		c, err := loadSecrets(x.T, "password: ${file:"+path+"}\nraw: ${file:"+path+"}\n")
		x.NoError(err)
		v, _ := c.Password.Value()
		x.Equal(" spaced ", v)
		r, _ := c.Raw.Value()
		x.Equal([]byte(" spaced \r\n"), r, "bytes are as read")
	}))
	t.Run("a failed read keeps the value in hand", x.F(func(x x.X) {
		writeAt(x.T, path, "good\n")
		c, err := loadSecrets(x.T, "password: ${file:"+path+"}\n")
		x.NoError(err)

		x.NoError(os.Remove(path))
		v, err := c.Password.Value()
		x.NoError(err)
		x.Equal("good", v)

		writeAt(x.T, path, "\n")
		v, err = c.Password.Value()
		x.NoError(err)
		x.Equal("good", v, "an empty file is a failed read")

		rotate(x.T, path, "better\n")
		v, _ = c.Password.Value()
		x.Equal("better", v)
	}))
	t.Run("a missing or empty file fails the load", x.F(func(x x.X) {
		_, err := loadSecrets(x.T, "password: ${file:"+filepath.Join(dir, "nope")+"}\n")
		x.True(errors.Is(err, os.ErrNotExist))

		empty := filepath.Join(dir, "empty")
		writeAt(x.T, empty, "\n")
		_, err = loadSecrets(x.T, "password: ${file:"+empty+"}\n")
		x.ErrorContains(err, "empty")
	}))
	t.Run("a file over the cap is refused", x.F(func(x x.X) {
		big := filepath.Join(dir, "big")
		writeAt(x.T, big, strings.Repeat("a", 64<<10+1))
		_, err := loadSecrets(x.T, "password: ${file:"+big+"}\n")
		x.ErrorContains(err, "over the")
	}))
	t.Run("copies share the state", x.F(func(x x.X) {
		writeAt(x.T, path, "a\n")
		c, err := loadSecrets(x.T, "password: ${file:"+path+"}\n")
		x.NoError(err)
		cp := *c

		rotate(x.T, path, "b\n")
		_, _ = c.Password.Value()
		v, _ := cp.Password.Value()
		x.Equal("b", v)
	}))
}

func TestSecretCustom(t *testing.T) {
	x := x.New(t)
	c, err := loadSecrets(t, "key: AQIDBA==\n")
	x.NoError(err)
	k, err := c.Key.Value()
	x.NoError(err)
	x.Equal([4]byte{1, 2, 3, 4}, k)

	_, err = loadSecrets(t, "key: AQID\n")
	x.ErrorContains(err, "key: want 4 bytes, got 3")
}

func TestSecretScheme(t *testing.T) {
	x := x.New(t)
	c := &Secrets{}
	l := cfg.New("app", c, cfg.WithScheme("vault", func(ref string) ([]byte, error) {
		if ref != "db#pw" {
			return nil, errors.New("no such secret")
		}
		return []byte("from-vault"), nil
	}))
	_, err := l.Load(write(t, "password: ${vault:db#pw}\n"), nil)
	x.NoError(err)
	v, _ := c.Password.Value()
	x.Equal("from-vault", v)

	_, err = l.Load(write(t, "password: ${vault:other}\n"), nil)
	x.ErrorContains(err, "${vault:other}: no such secret")

	_, err = l.Load(write(t, "password: ${nope:x}\n"), nil)
	x.ErrorContains(err, `unknown scheme "nope"`)
}

func TestSecretUnmarshalText(t *testing.T) {
	x := x.New(t)
	var s cfg.Secret
	x.NoError(s.UnmarshalText([]byte("lit")))
	v, _ := s.Value()
	x.Equal("lit", v)

	x.NoError(s.UnmarshalText(nil))
	x.True(s.IsZero())
}

func TestSecretOrigin(t *testing.T) {
	x := x.New(t)
	path := filepath.Join(t.TempDir(), "pw")
	writeAt(t, path, "x")

	c := &Secrets{}
	l := cfg.New("app", c)
	_, err := l.Load(write(t, "password: ${file:"+path+"}\ntoken: plain\n"), nil)
	x.NoError(err)

	o, ok := l.Origin(&c.Password)
	x.True(ok)
	x.Equal([]string{"${file:" + path + "}"}, o.Refs)
}
