package cfg

import "reflect"

// clone is a deep copy of v: what v points to, the elements of its lists and
// the values of its maps are copied too, so that reading into the copy never
// writes into v. Fields that are not exported are copied as they are; a value
// that keeps state behind one (a Secret) shares it, as copies of it do.
func clone(v reflect.Value) reflect.Value {
	return cloneOf(v, map[pointer]reflect.Value{})
}

// pointer identifies what a pointer points to: a struct and its first field
// share an address.
type pointer struct {
	at  uintptr
	typ reflect.Type
}

func cloneOf(v reflect.Value, seen map[pointer]reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		p := pointer{v.Pointer(), v.Type()}
		if c, ok := seen[p]; ok {
			// Pointed to twice, or by itself: so is the copy.
			return c
		}
		c := reflect.New(v.Type().Elem())
		seen[p] = c
		c.Elem().Set(cloneOf(v.Elem(), seen))
		return c

	case reflect.Struct:
		c := reflect.New(v.Type()).Elem()
		c.Set(v)
		for i := range v.NumField() {
			if f := c.Field(i); f.CanSet() {
				f.Set(cloneOf(v.Field(i), seen))
			}
		}
		return c

	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		c := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		if plain(v.Type().Elem()) {
			reflect.Copy(c, v)
			return c
		}
		for i := range v.Len() {
			c.Index(i).Set(cloneOf(v.Index(i), seen))
		}
		return c

	case reflect.Array:
		c := reflect.New(v.Type()).Elem()
		c.Set(v)
		if plain(v.Type().Elem()) {
			return c
		}
		for i := range v.Len() {
			c.Index(i).Set(cloneOf(v.Index(i), seen))
		}
		return c

	case reflect.Map:
		if v.IsNil() {
			return v
		}
		c := reflect.MakeMapWithSize(v.Type(), v.Len())
		for it := v.MapRange(); it.Next(); {
			c.SetMapIndex(it.Key(), cloneOf(it.Value(), seen))
		}
		return c

	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		c := reflect.New(v.Type()).Elem()
		c.Set(cloneOf(v.Elem(), seen))
		return c
	}
	return v
}

// plain reports whether a value of type t holds nothing that refers to
// anything else, so that copying it copies all of it.
func plain(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return true
	case reflect.Array:
		return plain(t.Elem())
	}
	return false
}
