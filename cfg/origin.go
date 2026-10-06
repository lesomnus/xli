package cfg

import (
	"fmt"
	"strings"
)

// Source is the layer a value came from.
type Source int

const (
	// Unset is a field nothing said anything about; it holds its zero value.
	Unset Source = iota
	// Default is a value from WithDefaults or a bound flag's Default.
	Default
	// File is a value from the configuration file.
	File
	// Env is a value from an environment variable.
	Env
	// Flag is a value from a command-line flag.
	Flag
)

func (s Source) String() string {
	switch s {
	case Unset:
		return "unset"
	case Default:
		return "default"
	case File:
		return "file"
	case Env:
		return "env"
	case Flag:
		return "flag"
	default:
		return fmt.Sprintf("Source(%d)", int(s))
	}
}

// Origin is where a field's value came from.
type Origin struct {
	Source Source
	// Key is the field's dotted path, e.g. "ldap.addr".
	Key string
	// Name is the file path, the environment variable or the flag ("--listen")
	// the value came from; empty for Unset and Default.
	Name string
	// Line and Column locate the value in the file; zero unless Source is File.
	Line, Column int
	// Refs are the references the value was read through, as written, e.g.
	// "${env:DB_PASSWORD}".
	Refs []string
	// Cleared reports that the value was given as empty and cleared the field.
	Cleared bool
}

// IsSet reports whether the value was given by the file, the environment or a
// flag.
func (o Origin) IsSet() bool {
	return o.Source >= File
}

// String names where the value came from, for messages: "--listen",
// "ROSTER_LDAP_ADDR", "/etc/roster.yaml:12 (ldap.addr)", "default" or "unset".
func (o Origin) String() string {
	switch o.Source {
	case File:
		return fmt.Sprintf("%s:%d (%s)", o.Name, o.Line, o.Key)
	case Env, Flag:
		return o.Name
	default:
		return o.Source.String()
	}
}

// describe is the origin as the start of an error message about the field.
func (o Origin) describe() string {
	switch o.Source {
	case File:
		if o.Line > 0 {
			return fmt.Sprintf("%s:%d:%d: %s", o.Name, o.Line, o.Column, o.Key)
		}
		return fmt.Sprintf("%s: %s", o.Name, o.Key)
	case Env, Flag:
		return o.Name
	default:
		return o.Key
	}
}

// FieldError is an error about one field, naming where its value came from.
type FieldError struct {
	Origin Origin
	Err    error
}

func (e *FieldError) Error() string {
	if e.Origin.Key == "" && e.Origin.Name == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %s", e.Origin.describe(), e.Err)
}

func (e *FieldError) Unwrap() error {
	return e.Err
}

// errs collects errors of one load.
type errs []error

func (es *errs) add(o Origin, err error) {
	*es = append(*es, &FieldError{Origin: o, Err: err})
}

func (es *errs) addf(o Origin, format string, vs ...any) {
	es.add(o, fmt.Errorf(format, vs...))
}

// join is the collected errors as one, nil if there are none.
func (es errs) join() error {
	if len(es) == 0 {
		return nil
	}
	return &LoadError{Errs: es}
}

// LoadError is every error of one load.
type LoadError struct {
	Errs []error
}

func (e *LoadError) Error() string {
	if len(e.Errs) == 1 {
		return e.Errs[0].Error()
	}
	b := strings.Builder{}
	fmt.Fprintf(&b, "%d errors in the configuration:", len(e.Errs))
	for _, err := range e.Errs {
		b.WriteString("\n  ")
		b.WriteString(strings.ReplaceAll(err.Error(), "\n", "\n  "))
	}
	return b.String()
}

func (e *LoadError) Unwrap() []error {
	return e.Errs
}
