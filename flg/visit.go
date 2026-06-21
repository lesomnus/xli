package flg

import "fmt"

type Holder interface {
	GetFlags() Flags
}

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

func VisitP[T any](h Holder, name string, dst *T) bool {
	if dst == nil {
		return false
	}
	return Visit(h, name, func(v T) {
		*dst = v
	})
}

type NestedHolder[T Holder] interface {
	Holder
	HasParent() bool
	Parent() T
}

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

func LookupP[T any, U NestedHolder[U]](h NestedHolder[U], name string, dst *T) bool {
	if dst == nil {
		return false
	}
	return Lookup(h, name, func(v T) {
		*dst = v
	})
}

func Get[T any](h Holder, name string) (v T, ok bool) {
	ok = VisitP(h, name, &v)
	return
}

func MustGet[T any](h Holder, name string) T {
	if v, ok := Get[T](h, name); ok {
		return v
	}
	if f := h.GetFlags().Get(name); f != nil {
		if d, ok := f.(interface{ lookupDefault() (T, bool) }); ok {
			if v, ok := d.lookupDefault(); ok {
				return v
			}
		}
	}

	panic(fmt.Sprintf("%q: flg not set", name))
}

func Find[T any, U NestedHolder[U]](h NestedHolder[U], name string) (v T, ok bool) {
	ok = LookupP(h, name, &v)
	return
}

func MustFind[T any, U NestedHolder[U]](h NestedHolder[U], name string) T {
	if v, ok := Find[T, U](h, name); ok {
		return v
	}
	for {
		if f := h.GetFlags().Get(name); f != nil {
			if d, ok := f.(interface{ lookupDefault() (T, bool) }); ok {
				if v, ok := d.lookupDefault(); ok {
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
