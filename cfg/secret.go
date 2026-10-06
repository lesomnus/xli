package cfg

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

// Decoder turns the bytes of a secret into its value.
type Decoder[T any] interface {
	Decode(raw []byte) (T, error)
}

// StringDecoder reads a secret as a string without one trailing newline ("\n"
// or "\r\n"), which is what `echo` and editors leave at the end of a file.
// Anything else, whitespace included, is the secret's.
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

// BytesDecoder reads a secret as it is.
type BytesDecoder struct{}

func (BytesDecoder) Decode(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errEmpty
	}
	return raw, nil
}

// Secret is a string credential: a literal, `${env:NAME}`, `${file:/path}` or a
// reference of a scheme registered with WithScheme. See SecretOf.
type Secret = SecretOf[string, StringDecoder]

// SecretBytes is a credential read as bytes, e.g. a binary key. See SecretOf.
type SecretBytes = SecretOf[[]byte, BytesDecoder]

// SecretOf is a credential of type T, read by D.
//
// It is given as a literal, or as exactly one reference: `${env:NAME}`,
// `${env:NAME:-default}`, `${file:/path}`, or `${scheme:...}` for a scheme
// registered with WithScheme. `$$` is a literal `$`.
//
// A `${file:}` secret is read when the configuration is loaded. A file that
// cannot be read yet is not an error then, only a warning: a credential may be
// minted after the process starts. Value tries again on every call, and fails
// until the file has been read once. After that it checks the file on every
// call:
//
//   - the file changed if the path names a different file (a rotation renames
//     a new one into place) or its size or modification time differ;
//   - a read that fails after a good one keeps the value in hand, because a
//     rotation may rename first and fix permissions after;
//   - an empty file is a failed read, and a file over 64 KiB is refused.
//
// A secret prints as its reference, or as "<redacted>" for a literal, so a
// configuration can be shown without its secrets.
//
// Copies of a SecretOf share their state.
type SecretOf[T any, D Decoder[T]] struct {
	s *secretState[T, D]
}

type secretState[T any, D Decoder[T]] struct {
	ref  string // as written; "" for a literal
	path string // for ${file:}

	mu    sync.Mutex
	value T
	seen  os.FileInfo
}

// maxSecretFile is the most read from a secret file.
const maxSecretFile = 64 << 10

const redacted = "<redacted>"

// secretField is implemented by *SecretOf.
type secretField interface {
	setSecret(text string, r *resolver) error
	refs() []string
}

// Value is the secret as it is now.
func (s SecretOf[T, D]) Value() (T, error) {
	if s.s == nil {
		var z T
		return z, nil
	}
	return s.s.get()
}

// IsZero reports whether the secret was not given.
func (s SecretOf[T, D]) IsZero() bool {
	return s.s == nil
}

// Ref is the reference the secret was given as, e.g. "${file:/run/key}", or ""
// for a literal.
func (s SecretOf[T, D]) Ref() string {
	if s.s == nil {
		return ""
	}
	return s.s.ref
}

// String is the reference, or "<redacted>" for a literal, or "" if not given.
func (s SecretOf[T, D]) String() string {
	switch {
	case s.s == nil:
		return ""
	case s.s.ref != "":
		return s.s.ref
	default:
		return redacted
	}
}

// MarshalText is String, so that encoders never write a secret out.
func (s SecretOf[T, D]) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText reads a secret as the environment gives one: exactly one
// reference, `${env:}` resolved against the process environment, or else a
// literal taken as it is. Schemes registered with WithScheme are known only to
// a Loader. A `${file:}` that cannot be read yet is not an error; Value
// reports it.
func (s *SecretOf[T, D]) UnmarshalText(b []byte) error {
	err := s.setSecret(string(b), &resolver{lookup: os.LookupEnv, verbatim: true})
	if isPending(err) {
		return nil
	}
	return err
}

// pendingError is a secret file that could not be read at load. It is a
// warning, not an error: Value keeps trying.
type pendingError struct {
	err error
}

func (e *pendingError) Error() string {
	return e.err.Error() + "; it is read again when used"
}

func (e *pendingError) Unwrap() error {
	return e.err
}

func (s *SecretOf[T, D]) refs() []string {
	if s.s == nil || s.s.ref == "" {
		return nil
	}
	return []string{s.s.ref}
}

func (s *SecretOf[T, D]) setSecret(text string, r *resolver) error {
	if text == "" {
		s.s = nil
		return nil
	}

	var ref, lit string
	if r.verbatim {
		// From the environment or a flag: a reference only as the whole
		// value, so that a password with a "$$" or a "${" in it, as a
		// Kubernetes Secret may hand over, is taken as it is.
		if isRef(text) {
			ref = text
		} else {
			lit = text
		}
	} else {
		var err error
		if ref, lit, err = splitSecret(text); err != nil {
			return err
		}
	}

	st := &secretState[T, D]{ref: ref}
	if ref == "" {
		if p, ok := bareRef(lit); ok {
			return fmt.Errorf("%q would be taken as it is; a reference is written ${%s}", lit, p)
		}
		v, err := decode[T, D]([]byte(lit))
		if err != nil {
			return err
		}
		st.value = v
		s.s = st
		return nil
	}
	r.refs = append(r.refs, ref)

	scheme, rest, err := parseRef(ref)
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
		s.s = st
		if _, err := st.get(); err != nil {
			return &pendingError{err}
		}
		return nil
	case "env":
		v, err := r.env(rest)
		if err != nil {
			return err
		}
		raw = []byte(v)
	default:
		raw, err = r.scheme(scheme, rest)
		if err != nil {
			return fmt.Errorf("%s: %w", ref, err)
		}
	}

	v, err := decode[T, D](raw)
	if err != nil {
		return fmt.Errorf("%s: %w", ref, err)
	}
	st.value = v
	s.s = st
	return nil
}

func decode[T any, D Decoder[T]](raw []byte) (T, error) {
	var d D
	return d.Decode(raw)
}

// isRef reports whether text is exactly one reference.
func isRef(text string) bool {
	return strings.HasPrefix(text, "${") && strings.IndexByte(text, '}') == len(text)-1
}

// bareRef reports whether a literal looks like a reference written as roster
// and shale used to, `file:/path` or `env:NAME`; p is it as the body of one.
func bareRef(lit string) (p string, ok bool) {
	if strings.HasPrefix(lit, "file:") || strings.HasPrefix(lit, "env:") {
		return lit, true
	}
	return "", false
}

// splitSecret reads a secret as written in the file: exactly one reference, or
// a literal in which `$$` is a `$`.
func splitSecret(text string) (ref string, lit string, err error) {
	if isRef(text) {
		return text, "", nil
	}

	b := strings.Builder{}
	for i := 0; i < len(text); i++ {
		if text[i] == '$' && i+1 < len(text) {
			switch text[i+1] {
			case '$':
				i++
			case '{':
				return "", "", errors.New("a secret is a literal or exactly one reference; write $$ for a literal $")
			}
		}
		b.WriteByte(text[i])
	}
	return "", b.String(), nil
}

// get is the value, re-reading the file of a ${file:} secret when it changed.
func (st *secretState[T, D]) get() (T, error) {
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
	case fi.Size() > maxSecretFile:
		return st.failed(fmt.Errorf("%d bytes, over the %d cap", fi.Size(), maxSecretFile))
	}

	b, err := os.ReadFile(st.path)
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

// failed is a read that gave no value: the one in hand if there is one.
func (st *secretState[T, D]) failed(err error) (T, error) {
	if st.seen != nil {
		return st.value, nil
	}
	var z T
	return z, fmt.Errorf("secret file %s: %w", st.path, err)
}

func (st *secretState[T, D]) unchanged(fi os.FileInfo) bool {
	return os.SameFile(st.seen, fi) &&
		fi.Size() == st.seen.Size() &&
		fi.ModTime().Equal(st.seen.ModTime())
}
