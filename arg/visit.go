package arg

import "fmt"

type Holder interface {
	GetArgs() Args
}

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

func VisitP[T any](h Holder, name string, dst *T) bool {
	if dst == nil {
		return false
	}
	return Visit(h, name, func(v T) {
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
	if a := h.GetArgs().Get(name); a != nil {
		if d, ok := a.(interface{ lookupDefault() (T, bool) }); ok {
			if v, ok := d.lookupDefault(); ok {
				return v
			}
		}
	}

	panic(fmt.Sprintf("%q: arg not set", name))
}
