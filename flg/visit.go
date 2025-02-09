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
	for h.HasParent() {
		if Visit(h, name, visitor) {
			return true
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
	v, ok := Get[T](h, name)
	if !ok {
		panic(fmt.Sprintf("%q: flg not parsed", name))
	}

	return v
}

func Find[T any, U NestedHolder[U]](h NestedHolder[U], name string) (v T, ok bool) {
	ok = LookupP(h, name, &v)
	return
}

func MustFind[T any, U NestedHolder[U]](h NestedHolder[U], name string) T {
	v, ok := Find[T, U](h, name)
	if !ok {
		panic(fmt.Sprintf("%q: flg not parsed", name))
	}

	return v
}
