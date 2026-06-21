package flg

import (
	"context"
	"fmt"

	"github.com/lesomnus/xli/mode"
)

type Parser[T any] interface {
	Parse(s string) (T, error)
	ToString(v T) string
	String() string
}

type Base[T any, P Parser[T]] struct {
	Name     string
	Alias    rune
	Category string

	Brief string
	Synop string
	Usage fmt.Stringer

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

	count int
}

func (f *Base[T, P]) Info() *Info {
	info := &Info{
		Category: f.Category,
		Name:     f.Name,
		Alias:    f.Alias,

		Type:     f.Parser.String(),
		Brief:    f.Brief,
		Synop:    f.Synop,
		Usage:    f.Usage,
		Required: f.Required,
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

// lookupDefault returns the configured default value, if any.
func (f *Base[T, P]) lookupDefault() (T, bool) {
	if f.Default == nil {
		var z T
		return z, false
	}
	return *f.Default, true
}

func (f *Base[T, P]) Handle(ctx context.Context, u string) error {
	if m := mode.From(ctx); m == mode.Tab {
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
