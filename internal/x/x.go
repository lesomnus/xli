package x

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type X struct {
	T *testing.T
}

func New(t *testing.T) X {
	return X{T: t}
}

// Equal fails if expected != actual
func (x X) Equal(expected, actual any, args ...any) {
	if !reflect.DeepEqual(expected, actual) {
		x.failf(fmt.Sprintf("expected %v but got %v", expected, actual), args...)
	}
}

// NotEqual fails if expected == actual
func (x X) NotEqual(expected, actual any, args ...any) {
	if reflect.DeepEqual(expected, actual) {
		x.failf(fmt.Sprintf("should not be equal: %v", expected), args...)
	}
}

// Nil fails if v is not nil
func (x X) Nil(v any, args ...any) {
	if !isNil(v) {
		x.failf(fmt.Sprintf("expected nil but got %v", v), args...)
	}
}

// NotNil fails if v is nil
func (x X) NotNil(v any, args ...any) {
	if isNil(v) {
		x.failf("expected not nil", args...)
	}
}

// True fails if v is false
func (x X) True(v bool, args ...any) {
	if !v {
		x.failf("expected true but got false", args...)
	}
}

// False fails if v is true
func (x X) False(v bool, args ...any) {
	if v {
		x.failf("expected false but got true", args...)
	}
}

// Len fails if len(v) != length
func (x X) Len(v any, length int, args ...any) {
	if v == nil {
		x.T.Errorf("Len does not support type %T", v)
		return
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String, reflect.Chan:
		if rv.Len() != length {
			x.failf(fmt.Sprintf("expected length %d but got %d", length, rv.Len()), args...)
		}
	default:
		x.T.Errorf("Len does not support type %T", v)
	}
}

// IsType fails if the type of obj does not match the type of expectedType
func (x X) IsType(expected any, obj any, args ...any) {
	if fmt.Sprintf("%T", expected) != fmt.Sprintf("%T", obj) {
		x.failf(fmt.Sprintf("expected type %T but got %T", expected, obj), args...)
	}
}

// NoError fails if err is not nil
func (x X) NoError(err error, args ...any) {
	if err != nil {
		x.failf(fmt.Sprintf("expected no error but got %v", err), args...)
	}
}

// ErrorContains fails if err is nil or if err.Error() does not contain contains
func (x X) ErrorContains(err error, contains string, args ...any) {
	if err == nil {
		x.failf("expected error but got nil", args...)
		return
	}
	if !strings.Contains(err.Error(), contains) {
		x.failf(fmt.Sprintf("expected error to contain %q but got %q", contains, err.Error()), args...)
	}
}

// Contains fails if haystack does not contain needle
func (x X) Contains(haystack string, needle string, args ...any) {
	if !strings.Contains(haystack, needle) {
		x.failf(fmt.Sprintf("expected %q to contain %q", haystack, needle), args...)
	}
}

// NotContains fails if haystack contains needle
func (x X) NotContains(haystack string, needle string, args ...any) {
	if strings.Contains(haystack, needle) {
		x.failf(fmt.Sprintf("expected %q to not contain %q", haystack, needle), args...)
	}
}

// Empty fails if v is not empty
func (x X) Empty(v any, args ...any) {
	if isEmpty(v) == false {
		x.failf(fmt.Sprintf("expected empty but got %v", v), args...)
	}
}

// NotEmpty fails if v is empty
func (x X) NotEmpty(v any, args ...any) {
	if isEmpty(v) == true {
		x.failf("expected not empty but got empty", args...)
	}
}

// Zero fails if v is not zero-valued
func (x X) Zero(v any, args ...any) {
	zero := false
	switch val := v.(type) {
	case int:
		zero = val == 0
	case int32:
		zero = val == 0
	case int64:
		zero = val == 0
	case uint:
		zero = val == 0
	case uint32:
		zero = val == 0
	case uint64:
		zero = val == 0
	case float32:
		zero = val == 0
	case float64:
		zero = val == 0
	case string:
		zero = val == ""
	case nil:
		zero = true
	default:
		zero = v == nil
	}
	if !zero {
		x.failf(fmt.Sprintf("expected zero value but got %v", v), args...)
	}
}

// Same fails if a and b are not the same (pointer equality)
func (x X) Same(a, b any, args ...any) {
	aPtrStr := fmt.Sprintf("%p", a)
	bPtrStr := fmt.Sprintf("%p", b)
	if aPtrStr != bPtrStr {
		x.failf(fmt.Sprintf("expected same pointer %p but got %p", a, b), args...)
	}
}

func (x X) failf(msg string, args ...any) {
	if len(args) > 0 {
		msg = msg + ": " + fmt.Sprint(args...)
	}
	x.T.Error(msg)
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String, reflect.Chan:
		return rv.Len() == 0
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return rv.Complex() == 0
	case reflect.Interface, reflect.Pointer, reflect.Func:
		return rv.IsNil()
	default:
		return rv.IsZero()
	}
}

func isNil(v any) bool {
	if v == nil {
		return true
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
