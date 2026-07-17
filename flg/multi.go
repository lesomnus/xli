package flg

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/lesomnus/xli/mode"
)

// Strings is a repeatable string flag: each occurrence on the command line
// appends one value, so `--tag a --tag b` yields []string{"a", "b"}.
type Strings = Multi[string, StringParser]

// Multi is a flag that accumulates a value of type T for every occurrence on
// the command line. Where [Base] holds a single scalar and the last occurrence
// wins, Multi appends each parsed value to a slice. It reuses the same scalar
// [Parser] as the corresponding Base flag (e.g. [StringParser] for [Strings]),
// parsing one token per occurrence.
//
// Like [arg.Rest], the accumulated Default and Value are plain slices whose nil
// value means "absent" — unlike the scalar Base, which uses *T.
type Multi[T any, P Parser[T]] struct {
	Name     string
	Alias    rune
	Category string

	Brief string
	Synop string
	Usage fmt.Stringer

	// Default is the value used when the user does not provide the flag. It is
	// set by the framework user and never modified by the framework. A nil
	// Default means there is no default.
	Default []T

	// Value holds the values parsed from the command line, one appended per
	// occurrence; it is nil until the user provides the flag. Read it via
	// Get/MustGet rather than directly.
	Value []T

	Handler Handler[[]T]

	Parser P

	// Required reports that the user must provide this flag at least once; Run
	// returns ErrFlagRequired when a required flag is absent.
	Required bool
}

func (f *Multi[T, P]) Info() *Info {
	info := &Info{
		Category: f.Category,
		Name:     f.Name,
		Alias:    f.Alias,

		Type:     fmt.Sprintf("%s...", f.Parser.String()),
		Brief:    f.Brief,
		Synop:    f.Synop,
		Usage:    f.Usage,
		Required: f.Required,
	}
	if f.Default != nil {
		info.Default = f.defaultString()
		info.HasDefault = true
	}
	return info
}

// defaultString renders the default slice using the element parser, e.g.
// ["a" "b"] for a Strings default of {"a", "b"}.
func (f *Multi[T, P]) defaultString() string {
	parts := make([]string, len(f.Default))
	for i, v := range f.Default {
		parts[i] = f.Parser.ToString(v)
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, " "))
}

// Get returns the values parsed from the command line and whether the user
// provided the flag. It does not consider Default; use MustGet for the
// effective value.
func (f *Multi[T, P]) Get() ([]T, bool) {
	return f.Value, len(f.Value) > 0
}

// lookupDefault returns the configured default values, if any.
func (f *Multi[T, P]) lookupDefault() ([]T, bool) {
	if f.Default == nil {
		return nil, false
	}
	return f.Default, true
}

func (f *Multi[T, P]) Handle(ctx context.Context, u string) error {
	if m := mode.From(ctx); m == mode.Tab {
		var z []T
		f.handle(ctx, z)
		return nil
	}

	v, err := f.Parser.Parse(u)
	if err != nil {
		return err
	}

	f.Value = append(f.Value, v)
	// Hand the handler an independent snapshot. The framework keeps appending
	// into f.Value's backing array on later occurrences, so passing the live
	// slice would let a handler that retains or extends it observe those later
	// writes (or have its own appended result silently overwritten).
	return f.handle(ctx, slices.Clone(f.Value))
}

// Count returns the number of times the flag was provided, which equals the
// number of accumulated values.
func (f *Multi[T, P]) Count() int {
	return len(f.Value)
}

func (f *Multi[T, P]) setCategory(name string) {
	f.Category = name
}

// NoValue reports whether the flag's parser is value-less (a switch). A Multi
// flag consumes a value per occurrence unless its parser opts out by
// implementing `NoValue() bool`.
func (f *Multi[T, P]) NoValue() bool {
	if p, ok := any(f.Parser).(interface{ NoValue() bool }); ok {
		return p.NoValue()
	}
	return false
}

func (f *Multi[T, P]) handle(ctx context.Context, v []T) error {
	if h := f.Handler; h != nil {
		return h.Handle(ctx, v)
	}
	return nil
}
