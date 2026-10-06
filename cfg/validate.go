package cfg

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"

	"github.com/lesomnus/xli/internal/suggest"
)

// Validator is implemented by a block of the configuration that checks itself.
// It is called after the configuration is loaded, on every value of the tree
// that implements it: the root, nested structs, elements of lists and values
// of maps.
type Validator interface {
	Validate() error
}

var validatorType = reflect.TypeFor[Validator]()

// validate calls Validate on every value of v's tree that implements it.
func validate[T any](v reflect.Value, s *Snapshot[T]) errs {
	var es errs
	walkValidate(v, "", s.originFor, &es, false)
	return es
}

func (s *Snapshot[T]) originFor(key string) Origin {
	if f, ok := s.schema.byKey[key]; ok {
		return s.originOf(f)
	}
	return Origin{Key: key}
}

// walkValidate validates v and what it holds. promoted reports that v is an
// embedded field whose Validate was already called as its holder's.
func walkValidate(v reflect.Value, key string, origin func(string) Origin, es *errs, promoted bool) {
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		}
		walkValidate(v.Elem(), key, origin, es, promoted)
		return
	}

	if !v.CanAddr() {
		// A map value: copy it to call a method with a pointer receiver.
		c := reflect.New(v.Type()).Elem()
		c.Set(v)
		v = c
	}
	called := false
	if p := v.Addr(); p.Type().Implements(validatorType) {
		called = true
		if !promoted {
			if err := p.Interface().(Validator).Validate(); err != nil {
				es.add(origin(key), err)
			}
		}
	}

	t := v.Type()
	if t.Kind() == reflect.Struct && readsItself(t) {
		return
	}

	switch t.Kind() {
	case reflect.Struct:
		for i := range t.NumField() {
			sf := t.Field(i)
			if !sf.IsExported() {
				continue
			}
			tag, ok := parseTag(sf)
			if !ok {
				continue
			}
			sub := key
			if !tag.inline {
				sub = join(key, tag.name)
			}
			walkValidate(v.Field(i), sub, origin, es, sf.Anonymous && called)
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			walkValidate(v.Index(i), key+"["+itoa(i)+"]", origin, es, false)
		}
	case reflect.Map:
		it := v.MapRange()
		for it.Next() {
			walkValidate(it.Value(), join(key, textOf(it.Key())), origin, es, false)
		}
	}
}

func join(key, name string) string {
	if key == "" {
		return name
	}
	return key + "." + name
}

// hint is a did-you-mean suggestion for an unknown name among names.
func hint(name string, names []string) string {
	return suggest.Hint(suggest.Of(name, names))
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

// textOf is a map key as text.
func textOf(v reflect.Value) string {
	if v.CanInterface() {
		if t, ok := v.Interface().(encoding.TextMarshaler); ok {
			if b, err := t.MarshalText(); err == nil {
				return string(b)
			}
		}
	}
	return fmt.Sprint(v.Interface())
}
