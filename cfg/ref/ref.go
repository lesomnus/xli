// Package ref is a value of a configuration that may be written as a
// reference to where it is kept: a file a deployment mounts and replaces, a
// variable of the environment, or a scheme the application registers.
//
//	type TlsConfig struct {
//		Cert ref.Bytes  `yaml:"cert"` // ${file:/tls/tls.crt}, or the PEM itself
//		Ca   ref.String `yaml:"ca"`
//	}
//
// A field of type Of holds a literal, or exactly one reference:
//
//	${env:NAME}  ${env:NAME:-default}  ${file:/path}  ${scheme:...}
//
// In a configuration file a literal may hold `$` as `$$`. From the environment
// or a flag, anything but exactly one reference is a literal as it is given. A
// literal written as a reference without its `${}`, `file:/path` or `env:NAME`,
// is an error that says how to write one.
//
// Value is the value as it is now. A `${file:}` is read when the value is set,
// and checked on every call after:
//
//   - the file changed if the path names a different file (a deployment renames
//     a new one into place) or its size or modification time differ, and then
//     it is read again;
//   - a read that fails after a good one keeps the value in hand, because a
//     rotation may rename first and fix permissions after;
//   - a file that cannot be read yet when the value is set is not an error
//     then: Value tries again on every call, and fails until the file has been
//     read once;
//   - an empty file is a failed read, only a regular file is read, and no more
//     than 1 MiB of it.
//
// An Of prints as its reference as written, or as its literal as it was given.
// It is no secret: a credential is a cfg.Secret, which is an Of that prints a
// literal as "<redacted>" and reads no more than 64 KiB of a file. A cfg.Secret
// is declared in package cfg, which imports xli; this package imports neither,
// so a package that only declares the shape of a configuration can use it.
//
// Package cfg's loader reads an Of from a file, the environment and flags, and
// resolves the schemes registered with it. Without a loader, UnmarshalText
// reads an Of as the environment gives one, and SetFile makes one from a path.
package ref

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/lesomnus/xli/cfg/internal/reference"
)

// Decoder turns the bytes of a value into it.
type Decoder[T any] interface {
	Decode(raw []byte) (T, error)
}

// StringDecoder reads a value as a string without one trailing newline ("\n"
// or "\r\n"), which is what `echo` and editors leave at the end of a file.
// Anything else, whitespace included, is the value's.
type StringDecoder struct{}

func (StringDecoder) Decode(raw []byte) (string, error) {
	s := string(raw)
	if v, ok := strings.CutSuffix(s, "\r\n"); ok {
		s = v
	} else {
		s = strings.TrimSuffix(s, "\n")
	}
	if s == "" {
		return "", errEmpty
	}
	return s, nil
}

// TrimSpaceDecoder reads a value as a string without the whitespace around
// it: the rule for a value a tool writes with a newline, or a person pastes
// with a space either side, where nothing at its edges can be part of it, as
// with a token. A value that is nothing but whitespace is empty.
type TrimSpaceDecoder struct{}

func (TrimSpaceDecoder) Decode(raw []byte) (string, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return "", errEmpty
	}
	return s, nil
}

// BytesDecoder reads a value as it is.
type BytesDecoder struct{}

func (BytesDecoder) Decode(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errEmpty
	}
	return raw, nil
}

var errEmpty = errors.New("empty")

// String is a string that may be written as a reference. See Of.
type String = Of[string, StringDecoder]

// Bytes is a value read as bytes, e.g. a certificate. See Of.
type Bytes = Of[[]byte, BytesDecoder]

// Of is a value of type T, read by D, that may be written as a reference. See
// the package's documentation.
//
// Copies of an Of share their state.
type Of[T any, D Decoder[T]] struct {
	s *state[T, D]
}

type state[T any, D Decoder[T]] struct {
	ref  string // as written; "" for a literal
	lit  string // a literal as it was given
	path string // for ${file:}
	how  reference.Reading

	mu    sync.Mutex
	value T
	seen  os.FileInfo
}

// environ resolves against the process environment, with no schemes.
type environ struct{}

func (environ) LookupEnv(name string) (string, bool) { return os.LookupEnv(name) }

func (environ) Scheme(name string, _ string) ([]byte, error) {
	return nil, fmt.Errorf("unknown scheme %q", name)
}

func (environ) Verbatim() bool { return true }

// Value is the value as it is now. A []byte is a copy of its own, which the
// caller may clear after use.
func (o Of[T, D]) Value() (T, error) {
	if o.s == nil {
		var z T
		return z, nil
	}
	v, err := o.s.get()
	if b, ok := any(v).([]byte); ok {
		v = any(bytes.Clone(b)).(T)
	}
	return v, err
}

// IsZero reports whether the value was not given.
func (o Of[T, D]) IsZero() bool {
	return o.s == nil
}

// Ref is the reference the value was given as, e.g. "${file:/tls/tls.crt}",
// or "" for a literal.
func (o Of[T, D]) Ref() string {
	if o.s == nil {
		return ""
	}
	return o.s.ref
}

// String is the reference as written, or the literal as it was given, or "" if
// not given.
func (o Of[T, D]) String() string {
	switch {
	case o.s == nil:
		return ""
	case o.s.ref != "":
		return o.s.ref
	default:
		return o.s.lit
	}
}

// MarshalText is String.
func (o Of[T, D]) MarshalText() ([]byte, error) {
	return []byte(o.String()), nil
}

var (
	_ encoding.TextMarshaler   = String{}
	_ encoding.TextUnmarshaler = (*String)(nil)
)

// UnmarshalText reads a value as the environment gives one: exactly one
// reference, `${env:}` resolved against the process environment, or else a
// literal taken as it is. Schemes are known only to a loader. A `${file:}`
// that cannot be read yet is not an error; Value reports it.
func (o *Of[T, D]) UnmarshalText(b []byte) error {
	err := o.load(string(b), environ{}, reference.Reading{})
	if reference.IsPending(err) {
		return nil
	}
	return err
}

// SetFile makes o the value in the file at path, read as `${file:path}` is:
// now, and again by Value whenever the file changed. It is for an application
// whose configuration names the file rather than holding a reference to it,
// such as a `cert_file`. A file that cannot be read yet is not an error; Value
// reports it until it can be read.
func (o *Of[T, D]) SetFile(path string) error {
	return o.loadFile(path, reference.Reading{})
}

// targeter is implemented by *Of, whatever its T and D.
type targeter interface {
	target() reference.Target
}

func init() {
	reference.TargetOf = func(ptr any) (reference.Target, bool) {
		t, ok := ptr.(targeter)
		if !ok {
			return nil, false
		}
		return t.target(), true
	}
	tt := reflect.TypeFor[targeter]()
	reference.IsTarget = func(t reflect.Type) bool {
		return reflect.PointerTo(t).Implements(tt)
	}
}

func (o *Of[T, D]) target() reference.Target {
	return target[T, D]{o}
}

// target is an Of as the loader of package cfg reads into it.
type target[T any, D Decoder[T]] struct {
	o *Of[T, D]
}

func (t target[T, D]) Load(text string, r reference.Resolver, how reference.Reading) error {
	return t.o.load(text, r, how)
}

func (t target[T, D]) LoadFile(path string, how reference.Reading) error {
	return t.o.loadFile(path, how)
}

func (t target[T, D]) Ref() string {
	return t.o.Ref()
}

// loadFile is SetFile, read as how says.
func (o *Of[T, D]) loadFile(path string, how reference.Reading) error {
	if path == "" {
		return errors.New(how.File() + ": no path")
	}
	st := &state[T, D]{ref: "${file:" + path + "}", path: path, how: how}
	o.s = st
	// Read now, as a load would; until it can be, Value says why.
	st.get()
	return nil
}

// load reads text into o as a loader reads a value, resolving its reference
// by r, as how says. Empty text makes o zero. A `${file:}` that cannot be
// read yet sets o and returns a reference.Pending, which is a warning rather
// than a failure. Any other error leaves o as it was.
func (o *Of[T, D]) load(text string, r reference.Resolver, how reference.Reading) error {
	if text == "" {
		o.s = nil
		return nil
	}

	var ref, lit string
	if r.Verbatim() {
		// From the environment or a flag: a reference only as the whole
		// value, so that a password with a "$$" or a "${" in it, as a
		// Kubernetes Secret may hand over, is taken as it is.
		if reference.IsRef(text) {
			ref = text
		} else {
			lit = text
		}
	} else {
		var err error
		if ref, lit, err = split(text, how); err != nil {
			return err
		}
	}

	st := &state[T, D]{ref: ref, how: how}
	if ref == "" {
		if strings.HasPrefix(lit, "file:") || strings.HasPrefix(lit, "env:") {
			return fmt.Errorf("%q would be taken as it is; a reference is written ${%s}", lit, lit)
		}
		v, err := decode[T, D]([]byte(lit))
		if err != nil {
			return err
		}
		st.value, st.lit = v, lit
		o.s = st
		return nil
	}

	scheme, rest, err := reference.Parse(ref)
	if err != nil {
		return err
	}
	var raw []byte
	switch scheme {
	case "file":
		if rest == "" {
			return fmt.Errorf("%s: names no file", ref)
		}
		st.path = rest
		o.s = st
		if _, err := st.get(); err != nil {
			return &reference.Pending{Err: err}
		}
		return nil
	case "env":
		v, err := reference.Env(r.LookupEnv, rest)
		if err != nil {
			return err
		}
		raw = []byte(v)
	default:
		raw, err = r.Scheme(scheme, rest)
		if err != nil {
			return fmt.Errorf("%s: %w", ref, err)
		}
	}

	v, err := decode[T, D](raw)
	if err != nil {
		return fmt.Errorf("%s: %w", ref, err)
	}
	st.value = v
	o.s = st
	return nil
}

func decode[T any, D Decoder[T]](raw []byte) (T, error) {
	var d D
	return d.Decode(raw)
}

// split reads a value as written in a file: exactly one reference, or a
// literal in which `$$` is a `$`.
func split(text string, how reference.Reading) (ref string, lit string, err error) {
	if reference.IsRef(text) {
		return text, "", nil
	}

	b := strings.Builder{}
	for i := 0; i < len(text); i++ {
		if text[i] == '$' && i+1 < len(text) {
			switch text[i+1] {
			case '$':
				i++
			case '{':
				return "", "", fmt.Errorf("%s is a literal or exactly one reference; write $$ for a literal $", how.Noun())
			}
		}
		b.WriteByte(text[i])
	}
	return "", b.String(), nil
}

// get is the value, reading the file of a ${file:} again when it changed.
func (st *state[T, D]) get() (T, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.path == "" {
		return st.value, nil
	}

	fi, err := os.Stat(st.path)
	switch {
	case err != nil:
		return st.failed(err)
	case st.seen != nil && st.unchanged(fi):
		return st.value, nil
	case !fi.Mode().IsRegular():
		// A FIFO or a device would block, or never end.
		return st.failed(errors.New("not a regular file"))
	}

	b, fi, err := readCapped(st.path, st.how.Max())
	if err != nil {
		return st.failed(err)
	}
	v, err := decode[T, D](b)
	if err != nil {
		return st.failed(err)
	}
	st.value, st.seen = v, fi
	return v, nil
}

// readCapped reads the file at path, of at most max bytes, and describes the
// file it read.
func readCapped(path string, max int) ([]byte, os.FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if err != nil {
		return nil, nil, err
	}
	if len(b) > max {
		return nil, nil, fmt.Errorf("over the %d byte cap (%s)", max, size(max))
	}
	return b, fi, nil
}

// size is n bytes in KiB or MiB, when it is a whole number of them.
func size(n int) string {
	switch {
	case n >= 1<<20 && n%(1<<20) == 0:
		return fmt.Sprintf("%d MiB", n>>20)
	case n >= 1<<10 && n%(1<<10) == 0:
		return fmt.Sprintf("%d KiB", n>>10)
	}
	return fmt.Sprintf("%d bytes", n)
}

// failed is a read that gave no value: the one in hand if there is one.
func (st *state[T, D]) failed(err error) (T, error) {
	if st.seen != nil {
		return st.value, nil
	}
	var z T
	return z, fmt.Errorf("%s %s: %w", st.how.File(), st.path, err)
}

func (st *state[T, D]) unchanged(fi os.FileInfo) bool {
	return os.SameFile(st.seen, fi) &&
		fi.Size() == st.seen.Size() &&
		fi.ModTime().Equal(st.seen.ModTime())
}
