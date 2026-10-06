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
	// Default is a value the root held when the loader was made, or the
	// Default of a bound flag on the command path.
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

// describe is the origin as the start of an error message about the field:
// "/etc/app.yaml:3:5: db.dsn", "/etc/app.yaml" for the file as a whole,
// "APP_DB_DSN" or "--dsn".
func (o Origin) describe() string {
	switch o.Source {
	case File:
		at := o.Name
		if o.Line > 0 {
			at += fmt.Sprintf(":%d:%d", o.Line, o.Column)
		}
		switch {
		case o.Key == "":
			return at
		case at == "":
			return o.Key
		default:
			return at + ": " + o.Key
		}
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
	d := e.Origin.describe()
	if d == "" {
		return e.Err.Error()
	}
	return d + ": " + e.Err.Error()
}

func (e *FieldError) Unwrap() error {
	return e.Err
}

// errs collects errors of one load.
type errs []error

// add adds err about the value o tells of; the errors of several items of a
// list or a map are added one by one.
func (es *errs) add(o Origin, err error) {
	if j, ok := err.(joined); ok {
		for _, err := range j {
			es.add(o, err)
		}
		return
	}
	*es = append(*es, &FieldError{Origin: o, Err: err})
}

// joined are the errors of the items of one value, such as a list.
type joined []error

func (e joined) Error() string {
	vs := make([]string, len(e))
	for i, err := range e {
		vs[i] = err.Error()
	}
	return strings.Join(vs, "; ")
}

func (e joined) Unwrap() []error {
	return e
}

// joinItems is es as one error: nil, the one, or joined.
func joinItems(es []error) error {
	switch len(es) {
	case 0:
		return nil
	case 1:
		return es[0]
	default:
		return joined(es)
	}
}

// within puts the errors of an item at where it is, e.g. "[1]: ...".
func within(at string, err error) error {
	if j, ok := err.(joined); ok {
		vs := make(joined, len(j))
		for i, err := range j {
			vs[i] = within(at, err)
		}
		return vs
	}
	return fmt.Errorf("%s: %w", at, err)
}

// isPending reports whether err is a secret file not there yet, and nothing
// else: every one of joined errors has to be.
func isPending(err error) bool {
	switch e := err.(type) {
	case *pendingError:
		return true
	case interface{ Unwrap() []error }:
		vs := e.Unwrap()
		for _, err := range vs {
			if !isPending(err) {
				return false
			}
		}
		return len(vs) > 0
	case interface{ Unwrap() error }:
		return isPending(e.Unwrap())
	}
	return false
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
