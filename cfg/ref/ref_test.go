package ref_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/lesomnus/xli/cfg/ref"
	"github.com/lesomnus/xli/internal/x"
)

func writeAt(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// rotate replaces the file at path the way a deployment does: a new file
// renamed into place.
func rotate(t *testing.T, path, content string) {
	t.Helper()
	tmp := path + ".new"
	writeAt(t, tmp, content)
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

// TestLiteral is a value given as itself, which prints as itself: it is no
// secret.
func TestLiteral(t *testing.T) {
	x := x.New(t)
	var s ref.String
	x.True(s.IsZero())
	x.Equal("", s.String())

	x.NoError(s.UnmarshalText([]byte("Pa$$w0rd")))
	v, err := s.Value()
	x.NoError(err)
	x.Equal("Pa$$w0rd", v, "taken as the environment gives it")
	x.False(s.IsZero())
	x.Equal("", s.Ref())
	x.Equal("Pa$$w0rd", s.String())
	x.Equal("Pa$$w0rd", fmt.Sprint(s))

	b, err := yaml.Marshal(s)
	x.NoError(err)
	x.Equal("Pa$$w0rd\n", string(b))

	x.NoError(s.UnmarshalText(nil))
	x.True(s.IsZero())

	x.ErrorContains(s.UnmarshalText([]byte("file:/run/x")), `"file:/run/x" would be taken as it is; a reference is written ${file:/run/x}`)
}

func TestEnv(t *testing.T) {
	x := x.New(t)
	t.Setenv("REF_TEST_CA", "pem\n")
	var s ref.String
	x.NoError(s.UnmarshalText([]byte("${env:REF_TEST_CA}")))
	v, _ := s.Value()
	x.Equal("pem", v, "one trailing newline is dropped")
	x.Equal("${env:REF_TEST_CA}", s.String(), "a reference prints as written")

	x.NoError(s.UnmarshalText([]byte("${env:REF_TEST_NOPE:-dflt}")))
	v, _ = s.Value()
	x.Equal("dflt", v)

	x.ErrorContains(s.UnmarshalText([]byte("${env:REF_TEST_NOPE}")), "REF_TEST_NOPE is not set")
	x.ErrorContains(s.UnmarshalText([]byte("${vault:x}")), `unknown scheme "vault"`, "schemes are a loader's")
}

func TestFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ca.crt")

	t.Run("read when set and when rotated", x.F(func(x x.X) {
		writeAt(x.T, path, "one\n")
		var s ref.String
		x.NoError(s.UnmarshalText([]byte("${file:" + path + "}")))
		v, err := s.Value()
		x.NoError(err)
		x.Equal("one", v)
		x.Equal("${file:"+path+"}", s.String())

		rotate(x.T, path, "two\n")
		v, err = s.Value()
		x.NoError(err)
		x.Equal("two", v)

		cp := s
		rotate(x.T, path, "TWO\n")
		v, _ = cp.Value()
		x.Equal("TWO", v, "copies share the state")
	}))
	t.Run("a failed read keeps the value in hand", x.F(func(x x.X) {
		writeAt(x.T, path, "good\n")
		var s ref.String
		x.NoError(s.SetFile(path))
		x.NoError(os.Remove(path))
		v, err := s.Value()
		x.NoError(err)
		x.Equal("good", v)

		writeAt(x.T, path, "\n")
		v, _ = s.Value()
		x.Equal("good", v, "an empty file is a failed read")
	}))
	t.Run("a file not there yet fails until it is", x.F(func(x x.X) {
		late := filepath.Join(dir, "late")
		var s ref.String
		x.NoError(s.SetFile(late))
		_, err := s.Value()
		x.True(errors.Is(err, os.ErrNotExist))
		x.ErrorContains(err, "file "+late)
		x.NotContains(err.Error(), "secret", "an Of is no secret")

		writeAt(x.T, late, "here\n")
		v, err := s.Value()
		x.NoError(err)
		x.Equal("here", v)
	}))
	t.Run("up to 1 MiB is read", x.F(func(x x.X) {
		big := filepath.Join(dir, "bundle")
		writeAt(x.T, big, strings.Repeat("a", 1<<20))
		var s ref.Bytes
		x.NoError(s.SetFile(big))
		b, err := s.Value()
		x.NoError(err)
		x.Len(b, 1<<20)

		writeAt(x.T, big, strings.Repeat("a", 1<<20+1))
		x.NoError(s.SetFile(big))
		_, err = s.Value()
		x.ErrorContains(err, "1 MiB")
	}))
	t.Run("no path is an error", x.F(func(x x.X) {
		var s ref.String
		x.ErrorContains(s.SetFile(""), "no path")
	}))
}

// TestImports holds what the package may import: nothing of xli outside it
// and the module's internal package it shares with the loader, which imports
// nothing of xli either, and no template engine, so that a package that
// declares a configuration's shape can use it where xli cannot go, as in a
// WebAssembly sandbox.
func TestImports(t *testing.T) {
	const (
		self     = "github.com/lesomnus/xli/cfg/ref"
		internal = "github.com/lesomnus/xli/cfg/internal/reference"
	)
	for pkg, allowed := range map[string][]string{
		".":                     {self, internal},
		"../internal/reference": {internal},
	} {
		out, err := exec.Command("go", "list", "-deps", pkg).Output()
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				t.Fatalf("go list %s: %v\n%s", pkg, err, ee.Stderr)
			}
			t.Skip("no go command here:", err)
		}
		x := x.New(t)
		deps := strings.Fields(string(out))
		x.True(slices.Contains(deps, allowed[0]), "lists itself")
		for _, d := range deps {
			x.False(d == "text/template", pkg+" imports text/template")
			if slices.Contains(allowed, d) {
				continue
			}
			x.False(strings.HasPrefix(d, "github.com/lesomnus/xli"), pkg+" imports "+d)
		}
	}
}
