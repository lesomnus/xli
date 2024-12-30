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
type Flag struct {
	raw  string
	name string
	arg  *Arg
}

func (f *Flag) Raw() string {
	return f.raw
}

func (f *Flag) IsShort() bool {
	return !strings.HasPrefix(f.raw, "--")
}

func (f *Flag) IsStacked() bool {
	return f.IsShort() && len(f.raw) > 2
}

func (f *Flag) Name() string {
	return f.name
}

func (f *Flag) String() string {
	b := strings.Builder{}
	if f.IsShort() {
		b.WriteString("-")
	} else {
		b.WriteString("--")
	}
	b.WriteString(f.name)
	if f.arg != nil {
		b.WriteString("=")
		b.WriteString(f.arg.String())
	}

	return b.String()
}

func (f *Flag) Arg() *Arg {
	return f.arg
}

// Spread converts stacked flags into individual flags like:
//
//	"-bar" -> ["-b", "a", "r"]
//	"-bar=foo" -> ["-b", "a", "r=foo"]
//	"-b" -> ["-b"]
//	"--bar" -> ["--bar"]
func (f *Flag) Spread() []*Flag {
	if !f.IsShort() {
		return []*Flag{f}
	}

	fs := make([]*Flag, len(f.name))
	for i, r := range f.name {
		fs[i] = &Flag{
			raw:  string(r),
			name: string(r),
		}
	}

	// First flag contains leading dash.
	fs[0].raw = f.raw[:2]

	if f.arg != nil {
		l := len(fs) - 1
		fl := fs[l]
		fl.raw = f.raw[l+1:]
		fl.arg = f.arg
	}

	return fs
}

func (f *Flag) WithArg(a Arg) *Flag {
	f_ := *f
	f_.arg = &a
	return &f_
}
