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

type Arg string

func (f Arg) Raw() string {
	return string(f)
}

func (f Arg) String() string {
	return fmt.Sprintf("%q", string(f))
}

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

// Spread convert stacked flags into individual flags like:
// "-bar" -> ["-b", "a", "r"]
// "-b" -> ["-b"]
// "--bar" -> ["--bar"]
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

	fs[0].raw = f.raw[:2]
	return fs
}
