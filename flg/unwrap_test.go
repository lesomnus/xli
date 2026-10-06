package flg_test

import (
	"testing"

	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
)

// wrapped decorates a flag the way an integration (e.g. cfg.Bind) does.
type wrapped[T any] struct {
	flg.Flag
	inner interface {
		Get() (T, bool)
		GetDefault() (T, bool)
	}
}

func (w *wrapped[T]) Unwrap() flg.Flag      { return w.Flag }
func (w *wrapped[T]) Get() (T, bool)        { return w.inner.Get() }
func (w *wrapped[T]) GetDefault() (T, bool) { return w.inner.GetDefault() }

func wrap(f *flg.String) *wrapped[string] { return &wrapped[string]{Flag: f, inner: f} }

func TestUnwrap(t *testing.T) {
	t.Run("returns the wrapped flag", x.F(func(x x.X) {
		f := &flg.String{Name: "a"}
		x.Equal(flg.Flag(f), flg.Unwrap(wrap(f)))
		x.Nil(flg.Unwrap(f))
	}))
	t.Run("WithCategory reaches the wrapped flag", x.F(func(x x.X) {
		f := &flg.String{Name: "a"}
		flg.Flags{}.WithCategory("net", wrap(f))
		x.Equal("net", f.Category)
	}))
	t.Run("MustGet falls back to the default through a wrapper", x.F(func(x x.X) {
		def := "d"
		fs := flg.Flags{wrap(&flg.String{Name: "a", Default: &def})}
		x.Equal("d", flg.MustGet[string](holder(fs), "a"))
	}))
}

func TestGetDefault(t *testing.T) {
	x := x.New(t)
	def := "d"
	v, ok := (&flg.String{Default: &def}).GetDefault()
	x.True(ok)
	x.Equal("d", v)

	_, ok = (&flg.String{}).GetDefault()
	x.False(ok)

	vs, ok := (&flg.Strings{Default: []string{"a"}}).GetDefault()
	x.True(ok)
	x.Equal([]string{"a"}, vs)
}

type holder flg.Flags

func (h holder) GetFlags() flg.Flags { return flg.Flags(h) }
