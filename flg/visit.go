package flg

import "fmt"

// Holder is what has flags: an *xli.Command.
type Holder interface {
	GetFlags() Flags
}

// Visit calls visitor with the value of h's flag name if the user gave it, and
// reports whether it did. A flag that was not given -- whatever its Default --
// is not visited, and neither is one whose value is not a T.
func Visit[T any](h Holder, name string, visitor func(v T)) bool {
	f := h.GetFlags().Get(name)
	if f == nil {
		return false
	}

	g, ok := f.(interface{ Get() (T, bool) })
	if !ok {
		return false
	}

	v, ok := g.Get()
	if !ok {
		return false
	}

	visitor(v)
	return true
}

// VisitP is Visit storing the value in dst, which is left alone if the flag was
// not given: a value set before is what the flag overrides.
func VisitP[T any](h Holder, name string, dst *T) bool {
	if dst == nil {
		return false
	}
	return Visit(h, name, func(v T) {
		*dst = v
	})
}

// NestedHolder is a Holder with a parent: an *xli.Command, for the functions
// that look for a flag on the command and then on the commands above it.
type NestedHolder[T Holder] interface {
	Holder
	HasParent() bool
	Parent() T
}

// Lookup is Visit on h, and then on each command above it in turn, until one
// has the flag given: a flag a parent declares for all of its subcommands.
func Lookup[T any, U NestedHolder[U]](h NestedHolder[U], name string, visitor func(v T)) bool {
	for {
		if Visit(h, name, visitor) {
			return true
		}

		if !h.HasParent() {
			break
		}
		h = h.Parent()
	}

	return false
}

// LookupP is Lookup storing the value in dst, as VisitP does.
func LookupP[T any, U NestedHolder[U]](h NestedHolder[U], name string, dst *T) bool {
	if dst == nil {
		return false
	}
	return Lookup(h, name, func(v T) {
		*dst = v
	})
}

// Get is the value of h's flag name and true if the user gave it, or the zero
// value and false if not -- its Default is not looked at.
func Get[T any](h Holder, name string) (v T, ok bool) {
	ok = VisitP(h, name, &v)
	return
}

// MustGet is the value of h's flag name if the user gave it, or else its
// Default. It panics if there is neither, or if the flag's value is not a T:
// that is a mistake in the program, not in the command line.
func MustGet[T any](h Holder, name string) T {
	if v, ok := Get[T](h, name); ok {
		return v
	}
	if f := h.GetFlags().Get(name); f != nil {
		if d, ok := f.(interface{ GetDefault() (T, bool) }); ok {
			if v, ok := d.GetDefault(); ok {
				return v
			}
		}
	}

	panic(fmt.Sprintf("%q: flg not set", name))
}

// Find is Get on h, and then on each command above it in turn, until one has
// the flag given.
func Find[T any, U NestedHolder[U]](h NestedHolder[U], name string) (v T, ok bool) {
	ok = LookupP(h, name, &v)
	return
}

// MustFind is Find, or else the Default of the nearest command that declares
// the flag with one. It panics if there is neither.
func MustFind[T any, U NestedHolder[U]](h NestedHolder[U], name string) T {
	if v, ok := Find[T, U](h, name); ok {
		return v
	}
	for {
		if f := h.GetFlags().Get(name); f != nil {
			if d, ok := f.(interface{ GetDefault() (T, bool) }); ok {
				if v, ok := d.GetDefault(); ok {
					return v
				}
			}
		}
		if !h.HasParent() {
			break
		}
		h = h.Parent()
	}

	panic(fmt.Sprintf("%q: flg not set", name))
}
