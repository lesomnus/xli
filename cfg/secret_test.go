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

func TestSecretAsGiven(t *testing.T) {
	t.Run("the environment's value is taken as it is", x.F(func(x x.X) {
		for _, pw := range []string{"Pa$$w0rd", "ab${cd", "x}${"} {
			c, err := loadSecrets(x.T, "", "APP_PASSWORD="+pw)
			x.NoError(err)
			v, _ := c.Password.Value()
			x.Equal(pw, v)
		}
	}))
	t.Run("but for exactly one reference", x.F(func(x x.X) {
		c, err := loadSecrets(x.T, "", "APP_PASSWORD=${env:PW}", "PW=from-env")
		x.NoError(err)
		v, _ := c.Password.Value()
		x.Equal("from-env", v)
	}))
	t.Run("in a list from the environment", x.F(func(x x.X) {
		type C struct {
			Peers []struct {
				Key cfg.Secret `yaml:"key"`
			} `yaml:"peers"`
		}
		c := C{}
		_, err := cfg.New("app", &c, cfg.WithPaths()).Load("", env(`APP_PEERS=[{key: "${env:K}"}, {key: "a$$b"}]`, "K=k"))
		x.NoError(err)
		k0, _ := c.Peers[0].Key.Value()
		k1, _ := c.Peers[1].Key.Value()
		x.Equal("k", k0)
		x.Equal("a$$b", k1)
	}))
	t.Run("UnmarshalText takes a value as the environment gives it", x.F(func(x x.X) {
		var s cfg.Secret
		x.NoError(s.UnmarshalText([]byte("Pa$$w0rd")))
		v, _ := s.Value()
		x.Equal("Pa$$w0rd", v)
	}))
	t.Run("a reference written without ${} is an error", x.F(func(x x.X) {
		_, err := loadSecrets(x.T, "password: file:/run/pw\n")
		x.ErrorContains(err, `"file:/run/pw" would be taken as it is; a reference is written ${file:/run/pw}`)
		_, err = loadSecrets(x.T, "", "APP_PASSWORD=env:PW")
		x.ErrorContains(err, "APP_PASSWORD: \"env:PW\" would be taken as it is")
	}))
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
	t.Run("a file not there yet is a warning, and read when used", x.F(func(x x.X) {
		late := filepath.Join(dir, "late")
		c := &Secrets{}
		s, err := cfg.New("app", c).Load(write(x.T, "password: ${file:"+late+"}\n"), nil)
		x.NoError(err)
		x.Len(s.Warnings, 1)
		x.ErrorContains(s.Warnings[0], "password: secret file "+late)
		x.True(errors.Is(s.Warnings[0], os.ErrNotExist))

		_, err = c.Password.Value()
		x.True(errors.Is(err, os.ErrNotExist), "never read: an error")

		writeAt(x.T, late, "\n")
		_, err = c.Password.Value()
		x.ErrorContains(err, "empty", "an empty file is a failed read")

		rotate(x.T, late, "minted\n")
		v, err := c.Password.Value()
		x.NoError(err)
		x.Equal("minted", v)
	}))
	t.Run("a file over the cap is refused", x.F(func(x x.X) {
		big := filepath.Join(dir, "big")
		writeAt(x.T, big, strings.Repeat("a", 64<<10+1))
		c := &Secrets{}
		s, err := cfg.New("app", c).Load(write(x.T, "password: ${file:"+big+"}\n"), nil)
		x.NoError(err)
		x.ErrorContains(s.Warnings[0], "over the")
		_, err = c.Password.Value()
		x.ErrorContains(err, "over the")
	}))
	t.Run("bytes are a copy of their own", x.F(func(x x.X) {
		writeAt(x.T, path, "key")
		c, err := loadSecrets(x.T, "raw: ${file:"+path+"}\n")
		x.NoError(err)
		b, _ := c.Raw.Value()
		clear(b)
		b, _ = c.Raw.Value()
		x.Equal([]byte("key"), b)
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

func TestSecretPendingInCollections(t *testing.T) {
	type Peer struct {
		Key  cfg.Secret `yaml:"key"`
		Port int        `yaml:"port"`
	}
	type C struct {
		List  []cfg.Secret          `yaml:"list"`
		Map   map[string]cfg.Secret `yaml:"map"`
		Peers []Peer                `yaml:"peers"`
	}
	missing := "${file:" + filepath.Join(t.TempDir(), "missing") + "}"

	t.Run("items are kept and read when used", x.F(func(x x.X) {
		c := C{}
		l := cfg.New("app", &c)
		s, err := l.Load(write(x.T, "list: [from-file]\n"), env(
			"APP_LIST="+missing+",lit",
			"APP_MAP=a="+missing+",b=lit",
		))
		x.NoError(err)
		x.Len(s.Warnings, 2)
		x.Len(c.List, 2)
		x.Equal(missing, c.List[0].Ref())
		x.Len(c.Map, 2)

		o, _ := l.Origin(&c.List)
		x.Equal(cfg.SourceEnv, o.Source)
		x.Equal([]string{missing}, o.Refs)
	}))
	t.Run("an error beside one is not lost", x.F(func(x x.X) {
		_, err := cfg.New("app", &C{}, cfg.WithPaths()).Load("", env(`APP_PEERS=[{key: "`+missing+`"}, {port: notanint}]`))
		x.ErrorContains(err, `APP_PEERS: [1].port: "notanint" is not an integer`)
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

// TestSecretTrimSpace is the rule for a credential nothing at the edges of
// can be part of, as a token: the whitespace around it goes, from a file or as
// it is written, and nothing but whitespace is no credential.
func TestSecretTrimSpace(t *testing.T) {
	type C struct {
		Token cfg.SecretOf[string, cfg.TrimSpaceDecoder] `yaml:"token"`
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "token")

	t.Run("from a file and as written", x.F(func(x x.X) {
		writeAt(x.T, path, " \tdckr_pat_x \r\n")
		for given, want := range map[string]string{
			"token: ${file:" + path + "}\n": "dckr_pat_x",
			"token: '  hunter2  '\n":        "hunter2",
		} {
			c := &C{}
			_, err := cfg.New("app", c).Load(write(x.T, given), nil)
			x.NoError(err)
			v, err := c.Token.Value()
			x.NoError(err)
			x.Equal(want, v, given)
		}
	}))
	t.Run("whitespace alone is empty", x.F(func(x x.X) {
		_, err := cfg.New("app", &C{}).Load(write(x.T, "token: '  '\n"), nil)
		x.ErrorContains(err, "token: empty")

		// From a file it is a failed read, which keeps what was read before.
		writeAt(x.T, path, "first\n")
		c := &C{}
		_, err = cfg.New("app", c).Load(write(x.T, "token: ${file:"+path+"}\n"), nil)
		x.NoError(err)
		rotate(x.T, path, " \n")
		v, err := c.Token.Value()
		x.NoError(err)
		x.Equal("first", v)
	}))
}

// TestSecretSetFile is a secret made from a path rather than read from a
// configuration: the same file rules as `${file:}`, for an application whose
// configuration names the file.
func TestSecretSetFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("read now and when rotated", x.F(func(x x.X) {
		path := filepath.Join(dir, "token")
		writeAt(x.T, path, "one\n")
		var s cfg.Secret
		x.NoError(s.SetFile(path))
		v, err := s.Value()
		x.NoError(err)
		x.Equal("one", v)
		x.Equal("${file:"+path+"}", s.Ref())

		rotate(x.T, path, "two\n")
		v, err = s.Value()
		x.NoError(err)
		x.Equal("two", v)
	}))
	t.Run("a file not there yet fails until it is", x.F(func(x x.X) {
		path := filepath.Join(dir, "late")
		var s cfg.Secret
		x.NoError(s.SetFile(path))
		_, err := s.Value()
		x.ErrorContains(err, "secret file "+path)

		writeAt(x.T, path, "here\n")
		v, err := s.Value()
		x.NoError(err)
		x.Equal("here", v)
	}))
	t.Run("a path is anything, braces too", x.F(func(x x.X) {
		path := filepath.Join(dir, "a}b")
		writeAt(x.T, path, "braced\n")
		var s cfg.Secret
		x.NoError(s.SetFile(path))
		v, err := s.Value()
		x.NoError(err)
		x.Equal("braced", v)
	}))
	t.Run("no path is an error", x.F(func(x x.X) {
		var s cfg.Secret
		x.ErrorContains(s.SetFile(""), "no path")
	}))
}
