package arg

import "fmt"

// Holder is what has arguments: an *xli.Command.
type Holder interface {
	GetArgs() Args
}

// Visit calls visitor with the value of h's argument name if the user gave it,
// and reports whether it did. An argument that was not given -- whatever its
// Default -- is not visited, and neither is one whose value is not a T.
func Visit[T any](h Holder, name string, visitor func(v T)) bool {
	f := h.GetArgs().Get(name)
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

// VisitP is Visit storing the value in dst, which is left alone if the
// argument was not given.
func VisitP[T any](h Holder, name string, dst *T) bool {
	if dst == nil {
		return false
	}
	return Visit(h, name, func(v T) {
		*dst = v
	})
}

// Get is the value of h's argument name and true if the user gave it, or the
// zero value and false if not -- its Default is not looked at.
func Get[T any](h Holder, name string) (v T, ok bool) {
	ok = VisitP(h, name, &v)
	return
}

// MustGet is the value of h's argument name if the user gave it, or else its
// Default. It panics if there is neither, or if the argument's value is not a
// T: that is a mistake in the program, not in the command line.
func MustGet[T any](h Holder, name string) T {
	if v, ok := Get[T](h, name); ok {
		return v
	}
	if a := h.GetArgs().Get(name); a != nil {
		if d, ok := a.(interface{ lookupDefault() (T, bool) }); ok {
			if v, ok := d.lookupDefault(); ok {
				return v
			}
		}
	}

	panic(fmt.Sprintf("%q: arg not set", name))
}
