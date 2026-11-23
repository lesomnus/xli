// Package reference is what package ref and the loader of package cfg share
// and an application does not see: the grammar of a reference, and the way
// the loader reads into a ref.Of. It imports neither xli nor cfg nor ref.
package reference

import (
	"fmt"
	"reflect"
	"strings"
)

// IsRef reports whether text is exactly one reference.
func IsRef(text string) bool {
	return strings.HasPrefix(text, "${") && strings.IndexByte(text, '}') == len(text)-1
}

// Parse splits "${scheme:rest}".
func Parse(ref string) (scheme string, rest string, err error) {
	body, ok := strings.CutPrefix(ref, "${")
	if ok {
		body, ok = strings.CutSuffix(body, "}")
	}
	if !ok {
		return "", "", fmt.Errorf("%q: not a reference", ref)
	}
	scheme, rest, ok = strings.Cut(body, ":")
	if !ok || scheme == "" {
		return "", "", fmt.Errorf("%q: want ${scheme:...}", ref)
	}
	if strings.Contains(rest, "${") {
		// `${env:A:-${env:B}}`: the first "}" would end the outer one.
		return "", "", fmt.Errorf("%q: a reference cannot hold another", ref)
	}
	return scheme, rest, nil
}

// Env resolves the body of `${env:NAME}` or `${env:NAME:-default}` by lookup.
func Env(lookup func(string) (string, bool), body string) (string, error) {
	name, def, hasDef := strings.Cut(body, ":-")
	if !ValidEnvName(name) {
		return "", fmt.Errorf("${env:%s}: %q is not a variable name", body, name)
	}
	if v, ok := lookup(name); ok {
		return v, nil
	}
	if hasDef {
		return def, nil
	}
	return "", fmt.Errorf("${env:%s}: %s is not set", body, name)
}

// ValidEnvName reports whether s is a name of a variable of the environment.
func ValidEnvName(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		switch {
		case c == '_', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// Resolver is what a loader resolves references by: the environment, and the
// schemes registered with it.
type Resolver interface {
	// LookupEnv is the variable name of the environment, as os.LookupEnv.
	LookupEnv(name string) (string, bool)
	// Scheme is the value `${name:ref}` names, for a registered scheme.
	Scheme(name string, ref string) ([]byte, error)
	// Verbatim reports that the text is from the environment or a flag,
	// where only exactly one reference is one, and a literal is as it is.
	Verbatim() bool
}

// Reading is how a ref.Of is read: what its errors call it, and how much of a
// file it reads. The zero Reading is a ref.Of's own; cfg's Secret is read as
// a secret, of no more than 64 KiB.
type Reading struct {
	// Name is what an error calls the value: "secret" says "secret file
	// /run/pw: ...". Empty is "file /run/pw: ...".
	Name string
	// MaxFile is the most read from a file, in bytes: 1 MiB when 0.
	MaxFile int
}

// MaxFile is the most read from a file into a ref.Of.
const MaxFile = 1 << 20

func (h Reading) Max() int {
	if h.MaxFile > 0 {
		return h.MaxFile
	}
	return MaxFile
}

// File is how an error names the file: "file", or "secret file".
func (h Reading) File() string {
	if h.Name == "" {
		return "file"
	}
	return h.Name + " file"
}

// Noun is how an error names the value: "a value", or "a secret".
func (h Reading) Noun() string {
	if h.Name == "" {
		return "a value"
	}
	return "a " + h.Name
}

// Target is a ref.Of as the loader reads into it.
type Target interface {
	// Load reads text into the value by r, as how says. Empty text makes it
	// zero. A `${file:}` that cannot be read yet sets it and returns a
	// Pending; any other error leaves it as it was.
	Load(text string, r Resolver, how Reading) error
	// LoadFile makes it the value in the file at path, as how says.
	LoadFile(path string, how Reading) error
	// Ref is the reference it was given as, or "" for a literal.
	Ref() string
}

// TargetOf is the ref.Of ptr points to as a Target, if it points to one. And
// IsTarget reports whether a value of type t is a ref.Of. Package ref sets
// both when it is initialised.
var (
	TargetOf func(ptr any) (Target, bool)
	IsTarget func(t reflect.Type) bool
)

// Pending is a file that could not be read when the value was set. It is a
// warning, not an error: the value keeps trying.
type Pending struct {
	Err error
}

func (e *Pending) Error() string {
	return e.Err.Error() + "; it is read again when used"
}

func (e *Pending) Unwrap() error {
	return e.Err
}

// IsPending reports whether err is a Pending, or wraps one through Unwrap()
// error. Errors joined are not looked into: whether they are all pending is
// the caller's to say.
func IsPending(err error) bool {
	for err != nil {
		if _, ok := err.(*Pending); ok {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
