package x

import "testing"

func F(f func(x X)) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()
		x := New(t)
		f(x)
	}
}
