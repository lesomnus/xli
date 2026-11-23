package cfg

import (
	"os"

	"github.com/lesomnus/xli/cfg/internal/reference"
	"github.com/lesomnus/xli/cfg/ref"
)

// Decoder turns the bytes of a secret into its value. It is ref.Decoder.
type Decoder[T any] = ref.Decoder[T]

// StringDecoder reads a secret as a string without one trailing newline ("\n"
// or "\r\n"), which is what `echo` and editors leave at the end of a file.
// Anything else, whitespace included, is the secret's. It is
// ref.StringDecoder.
type StringDecoder = ref.StringDecoder

// TrimSpaceDecoder reads a secret as a string without the whitespace around
// it: the rule for a credential a tool writes with a newline, or a person
// pastes with a space either side, where nothing at its edges can be part of
// it, as with a token. A secret that is nothing but whitespace is empty. It is
// ref.TrimSpaceDecoder.
type TrimSpaceDecoder = ref.TrimSpaceDecoder

// BytesDecoder reads a secret as it is. It is ref.BytesDecoder.
type BytesDecoder = ref.BytesDecoder

// Secret is a string credential: a literal, `${env:NAME}`, `${file:/path}` or a
// reference of a scheme registered with WithScheme. See SecretOf.
type Secret = SecretOf[string, StringDecoder]

// SecretBytes is a credential read as bytes, e.g. a binary key. See SecretOf.
type SecretBytes = SecretOf[[]byte, BytesDecoder]

// SecretOf is a credential of type T, read by D: a ref.Of marked secret.
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
// configuration can be shown without its secrets. That, and the smaller cap on
// a file, is all that sets it apart from a ref.Of.
//
// Copies of a SecretOf share their state.
type SecretOf[T any, D Decoder[T]] struct {
	r ref.Of[T, D]
}

// maxSecretFile is the most read from a secret file.
const maxSecretFile = 64 << 10

// asSecret is how a SecretOf is read.
var asSecret = reference.Reading{Name: "secret", MaxFile: maxSecretFile}

const redacted = "<redacted>"

// secretField is implemented by *SecretOf.
type secretField interface {
	refField
	secret()
}

// Value is the secret as it is now. A []byte is a copy of its own, which the
// caller may clear after use.
func (s SecretOf[T, D]) Value() (T, error) {
	return s.r.Value()
}

// IsZero reports whether the secret was not given.
func (s SecretOf[T, D]) IsZero() bool {
	return s.r.IsZero()
}

// Ref is the reference the secret was given as, e.g. "${file:/run/key}", or ""
// for a literal.
func (s SecretOf[T, D]) Ref() string {
	return s.r.Ref()
}

// String is the reference, or "<redacted>" for a literal, or "" if not given.
func (s SecretOf[T, D]) String() string {
	switch {
	case s.r.IsZero():
		return ""
	case s.r.Ref() != "":
		return s.r.Ref()
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
	err := s.setRef(string(b), &resolver{lookup: os.LookupEnv, verbatim: true})
	if isPending(err) {
		return nil
	}
	return err
}

// SetFile makes s the secret in the file at path, read as `${file:path}` is:
// now, and again by Value whenever the file changed. It is for an application
// whose configuration names the file rather than holding a reference to it,
// such as a `token_file`. A file that cannot be read yet is not an error;
// Value reports it until it can be read.
func (s *SecretOf[T, D]) SetFile(path string) error {
	return s.target().LoadFile(path, asSecret)
}

func (s *SecretOf[T, D]) secret() {}

// target is the ref.Of s holds, as the loader reads into it.
func (s *SecretOf[T, D]) target() reference.Target {
	t, _ := reference.TargetOf(&s.r)
	return t
}

func (s *SecretOf[T, D]) setRef(text string, r *resolver) error {
	return loadRef(s.target(), text, r, asSecret)
}

func (s *SecretOf[T, D]) refs() []string {
	return refsOf1(s.r)
}

// refField is a field the loader reads references into: a SecretOf, or a
// ref.Of as an ofField.
type refField interface {
	setRef(text string, r *resolver) error
	refs() []string
}

// ofField is a ref.Of as a refField.
type ofField struct {
	o reference.Target
}

func (f ofField) setRef(text string, r *resolver) error {
	return loadRef(f.o, text, r, reference.Reading{})
}

func (f ofField) refs() []string {
	return refsOf1(f.o)
}

// asRefField is what ptr points to as a refField, if it is one.
func asRefField(ptr any) (refField, bool) {
	if p, ok := ptr.(refField); ok {
		return p, true
	}
	if t, ok := reference.TargetOf(ptr); ok {
		return ofField{t}, true
	}
	return nil, false
}

// loadRef reads text into o by r, and records the reference it is read
// through.
func loadRef(o reference.Target, text string, r *resolver, how reference.Reading) error {
	if reference.IsRef(text) {
		r.refs = append(r.refs, text)
	}
	return o.Load(text, r, how)
}

// refsOf1 is the reference o was given as, if it was given as one.
func refsOf1(o interface{ Ref() string }) []string {
	if s := o.Ref(); s != "" {
		return []string{s}
	}
	return nil
}
