package flg

import (
	"context"

	"github.com/lesomnus/xli/mode"
	"github.com/lesomnus/xli/tab"
)

// Parser reads the value of a flag of type T from the text it was given as.
// ToString is a value written back, for a Default in help, and String names
// the type, for help; empty for a switch. A parser with a NoValue() bool
// method that reports true takes no value: a switch.
type Parser[T any] interface {
	Parse(s string) (T, error)
	ToString(v T) string
	String() string
}

// Base is a flag that takes one value of type T, read by P; the last time it
// is given wins. The types of this package are Bases with their parsers:
// [String], [Int], [Switch] and the rest.
type Base[T any, P Parser[T]] struct {
	Name     string
	Alias    rune
	Category string

	Brief string
	Synop string

	// Default is the value used when the user does not provide the flag.
	// It is set by the framework user and never modified by the framework.
	// A nil Default means there is no default.
	Default *T

	// Value holds the value parsed from the command line; it is nil until
	// the user provides the flag. Read it via Get/MustGet rather than
	// directly.
	Value *T

	Handler Handler[T]

	Parser P

	// Required reports that the user must provide this flag; Run returns
	// ErrFlagRequired when a required flag is absent.
	Required bool

	// Hidden omits the flag from help and completion; it is still accepted on
	// the command line.
	Hidden bool

	count int
}

// Info is what the flag says of itself.
func (f *Base[T, P]) Info() *Info {
	info := &Info{
		Category: f.Category,
		Name:     f.Name,
		Alias:    f.Alias,

		Type:     f.Parser.String(),
		Brief:    f.Brief,
		Synop:    f.Synop,
		Required: f.Required,
		Hidden:   f.Hidden,
	}
	if f.Default != nil {
		info.Default = f.Parser.ToString(*f.Default)
		info.HasDefault = true
	}
	return info
}

// Get returns the value parsed from the command line and whether the user
// provided the flag. It does not consider Default; use MustGet for the
// effective value.
func (f *Base[T, P]) Get() (T, bool) {
	if f.count == 0 {
		var z T
		return z, false
	}
	return *f.Value, true
}

// GetDefault returns the configured default value and whether there is one.
// Unlike Get it does not consider what the user provided.
func (f *Base[T, P]) GetDefault() (T, bool) {
	if f.Default == nil {
		var z T
		return z, false
	}
	return *f.Default, true
}

// Handle parses u into Value and calls the Handler with it. In completion it
// offers the parser's candidates, if it has any, instead.
func (f *Base[T, P]) Handle(ctx context.Context, u string) error {
	if m := mode.From(ctx); m == mode.Tab {
		complete(tab.From(ctx), f.Parser)
		var z T
		f.handle(ctx, z)
		return nil
	}

	v, err := f.Parser.Parse(u)
	if err != nil {
		return err
	}

	f.count++
	f.Value = &v
	return f.handle(ctx, v)
}

// Count is how many times the flag was given.
func (f *Base[T, P]) Count() int {
	return f.count
}

func (f *Base[T, P]) setCategory(name string) {
	f.Category = name
}

// NoValue reports whether the flag's parser is value-less (a switch).
// A parser opts in by implementing `NoValue() bool`; otherwise the flag
// is assumed to require a value.
func (f *Base[T, P]) NoValue() bool {
	if p, ok := any(f.Parser).(interface{ NoValue() bool }); ok {
		return p.NoValue()
	}
	return false
}

func (a *Base[T, P]) handle(ctx context.Context, v T) error {
	if h := a.Handler; h != nil {
		return h.Handle(ctx, v)
	}
	return nil
}
