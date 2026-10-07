package flg

import (
	"context"
	"fmt"
)

// Info is what a flag says of itself, for parsing, help and documentation.
type Info struct {
	Category string
	// Name is the long name, given as --name, and Alias the short one, given
	// as -a; 0 is none.
	Name  string
	Alias rune

	// Type names the value the flag takes, for help; empty for a switch.
	Type     string
	Brief    string
	Synop    string
	Required bool

	// Hidden reports that the flag is omitted from help and completion; it is
	// still accepted on the command line.
	Hidden bool

	// Env names an environment variable the flag's value can also be given
	// by, for help and generated documentation. The flag machinery does not
	// read it; it is set by whatever does (e.g. the cfg package).
	Env string

	// Default is the string form of the flag's default value, for help
	// rendering. HasDefault is false when the flag has no default.
	Default    string
	HasDefault bool
}

// String is the flag as help lists it: its names and the type it takes.
func (i *Info) String() string {
	if i.Alias == 0 {
		return fmt.Sprintf("   --%s %s", i.Name, i.Type)
	} else {
		return fmt.Sprintf("-%c,--%s %s", i.Alias, i.Name, i.Type)
	}
}

// Flag is a flag of a command. The types of this package are Flags; a type of
// one's own is too, given these methods.
type Flag interface {
	Info() *Info
	// Handle is given the value of each occurrence of the flag on the command
	// line, as it was written.
	Handle(ctx context.Context, v string) error

	// Count is how many times the flag was given; 0 is not given.
	Count() int

	// NoValue reports whether the flag is a switch that does not consume a
	// value (e.g. a boolean switch). Such flags default to "true" when given
	// without an explicit value.
	NoValue() bool
}

// Flags are the flags of one command.
type Flags []Flag

// Visible returns the flags that are not hidden.
func (fs Flags) Visible() Flags {
	vs := Flags{}
	for _, f := range fs {
		if !f.Info().Hidden {
			vs = append(vs, f)
		}
	}
	return vs
}

// Get is the flag of the long name name, or nil.
func (fs Flags) Get(name string) Flag {
	for _, f := range fs {
		if f.Info().Name == name {
			return f
		}
	}

	return nil
}

// GetByAlias is the flag of the short name c, or nil.
func (fs Flags) GetByAlias(c rune) Flag {
	for _, f := range fs {
		if f.Info().Alias == c {
			return f
		}
	}

	return nil
}

// ByCategory is fs grouped by Category, each group in the order its first flag
// appears.
func (fs Flags) ByCategory() []Flags {
	i := map[string]int{}
	vs := []Flags{}
	for _, f := range fs {
		j, ok := i[f.Info().Category]
		if !ok {
			j = len(vs)
			i[f.Info().Category] = j
			vs = append(vs, Flags{})
		}

		vs[j] = append(vs[j], f)
	}
	return vs
}

// WithCategory is fs with vs appended, each put in the category name.
func (fs Flags) WithCategory(name string, vs ...Flag) Flags {
	for _, v := range vs {
		for f := v; f != nil; f = Unwrap(f) {
			if s, ok := f.(interface{ setCategory(string) }); ok {
				s.setCategory(name)
				break
			}
		}
	}
	return append(fs, vs...)
}

// Unwrap returns the flag f wraps, or nil if f does not wrap one. A flag that
// decorates another (e.g. to bind it to a configuration field) implements
// `Unwrap() Flag`, which lets the flg package reach the flag underneath.
func Unwrap(f Flag) Flag {
	if u, ok := f.(interface{ Unwrap() Flag }); ok {
		return u.Unwrap()
	}
	return nil
}
