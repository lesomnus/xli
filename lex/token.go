package lex

import (
	"fmt"
	"strings"
)

type Token interface {
	Raw() string
	String() string
}

type Err struct {
	raw string
	err error
}

func (e *Err) Raw() string {
	return e.raw
}

func (e *Err) String() string {
	return fmt.Sprintf("[!%s]", e.Error())
}

func (e *Err) Error() string {
	return fmt.Sprintf("%s: %s", e.err.Error(), e.raw)
}

func (e *Err) Unwrap() error {
	return e.err
}

type EndOfCommand string

func (EndOfCommand) Raw() string {
	return "--"
}

func (EndOfCommand) String() string {
	return "--"
}

type Arg string

func (f Arg) Raw() string {
	return string(f)
}

func (f Arg) String() string {
	return fmt.Sprintf("%q", string(f))
}

// Flag holds an argument such as "-*" or "--*" where `*` is any printable character.
// Flag is short if there is single dash or long if there are two dashes.
// Its component can be accessed by:
//
//	  -foo=bar
//	 --foo=bar
//	   ^^^ ^^^
//	Name() Arg()
type Flag string

// Returns index for name and arg.
// `j` will -1 if there is no arg.
//
//	--name=arg
//	  ^    ^
//	  i    j
func (f Flag) indexes() (int, int) {
	i := strings.IndexFunc(string(f), func(r rune) bool { return r != '-' })
	j := strings.IndexRune(string(f)[i:], '=')
	if j < 0 {
		return i, -1
	}
	return i, i + j + 1
}

func (f Flag) Raw() string {
	return string(f)
}

func (f Flag) IsShort() bool {
	return !strings.HasPrefix(string(f), "--")
}

func (f Flag) IsStacked() bool {
	return f.IsShort() && len(string(f)) > 2
}

func (f Flag) Name() string {
	i, j := f.indexes()
	if j < 0 {
		return string(f)[i:]
	}
	return string(f)[i : j-1]
}

func (f Flag) Arg() (Arg, bool) {
	_, j := f.indexes()
	if j < 0 {
		return "", false
	}

	return Arg(string(f)[j:]), true
}

func (f Flag) String() string {
	_, j := f.indexes()
	v := string(f)
	if j < 0 {
		return v
	}
	return fmt.Sprintf("%s=%q", v[:j-1], v[j:])
}

// Spread converts stacked flags into individual flags like:
//
//	"-bar" -> ["-b", "a", "r"]
//	"-bar=foo" -> ["-b", "a", "r=foo"]
//	"-b" -> ["-b"]
//	"--bar" -> ["--bar"]
func (f Flag) Spread() []Flag {
	i, j := f.indexes()
	if i > 1 {
		// Long one.
		return []Flag{f}
	}

	// Index of name end.
	k := j - 1
	if k < 0 {
		k = len(f)
	}

	v := string(f)
	n := v[i:k]

	x := i + 1
	y := k - 1

	//  |-n-|
	// -abcde=foo
	//  ^^  ^^^
	//  ix  ykj

	vs := make([]Flag, len(n))
	vs[0] = Flag(v[:x])
	vs[len(n)-1] = Flag(v[y:])

	for ; x < y; x++ {
		vs[x-i] = Flag(v[x])
	}

	return vs
}

func (f Flag) WithArg(a Arg) Flag {
	_, j := f.indexes()
	if j > 0 {
		j -= 1
	} else {
		j = len(f)
	}

	return Flag(fmt.Sprintf("%s=%s", string(f)[:j], string(a)))
}

func (f Flag) WithoutArg() Flag {
	_, j := f.indexes()
	if j < 0 {
		return f
	}
	return f[:j-1]
}
