package arg

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
