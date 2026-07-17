package flg_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/mode"
	"github.com/lesomnus/xli/tab"
)

type handlerTestFakeTab struct {
	values []string
}

func (t *handlerTestFakeTab) Value(v string)               { t.values = append(t.values, v) }
func (t *handlerTestFakeTab) ValueD(v string, desc string) { t.values = append(t.values, v) }
func (t *handlerTestFakeTab) Group(name string) tab.Tab    { return t }
func (t *handlerTestFakeTab) Files(pattern string)         {}
func (t *handlerTestFakeTab) Dirs()                        {}

func TestHandle(t *testing.T) {
	t.Run("runs the wrapped func", x.F(func(x x.X) {
		got := ""
		h := flg.Handle[string](func(ctx context.Context, v string) error {
			got = v
			return nil
		})
		err := h.Handle(context.Background(), "foo")
		x.NoError(err)
		x.Equal("foo", got)
	}))
}

func TestWrap(t *testing.T) {
	t.Run("runs handlers in order", x.F(func(x x.X) {
		order := []int{}
		h := flg.Wrap(
			flg.Handle[string](func(ctx context.Context, v string) error {
				order = append(order, 1)
				return nil
			}),
			flg.Handle[string](func(ctx context.Context, v string) error {
				order = append(order, 2)
				return nil
			}),
		)
		err := h.Handle(context.Background(), "v")
		x.NoError(err)
		x.Equal([]int{1, 2}, order)
	}))

	t.Run("stops at the first failing handler", x.F(func(x x.X) {
		want := errors.New("boom")
		ran := []int{}
		h := flg.Wrap(
			flg.Handle[string](func(ctx context.Context, v string) error {
				ran = append(ran, 1)
				return nil
			}),
			flg.Handle[string](func(ctx context.Context, v string) error {
				ran = append(ran, 2)
				return want
			}),
			flg.Handle[string](func(ctx context.Context, v string) error {
				ran = append(ran, 3)
				return nil
			}),
		)
		err := h.Handle(context.Background(), "v")
		x.ErrorContains(err, "boom")
		x.Equal([]int{1, 2}, ran)
	}))
}

func TestOnF(t *testing.T) {
	t.Run("runs when predicate is true", x.F(func(x x.X) {
		ran := false
		h := flg.OnF[string](func(m mode.Mode) bool { return true }, func(ctx context.Context, v string) error {
			ran = true
			return nil
		})
		err := h.Handle(context.Background(), "v")
		x.NoError(err)
		x.True(ran)
	}))

	t.Run("skips when predicate is false", x.F(func(x x.X) {
		ran := false
		h := flg.OnF[string](func(m mode.Mode) bool { return false }, func(ctx context.Context, v string) error {
			ran = true
			return nil
		})
		err := h.Handle(context.Background(), "v")
		x.NoError(err)
		x.False(ran)
	}))
}

func TestOn(t *testing.T) {
	t.Run("runs when mode contains the bit", x.F(func(x x.X) {
		ran := false
		h := flg.On[string](mode.Run, func(ctx context.Context, v string) error {
			ran = true
			return nil
		})

		ctx := mode.Into(context.Background(), mode.Run)
		err := h.Handle(ctx, "v")
		x.NoError(err)
		x.True(ran)

		ran = false
		ctx = mode.Into(context.Background(), mode.Run|mode.Pass)
		err = h.Handle(ctx, "v")
		x.NoError(err)
		x.True(ran)
	}))

	t.Run("skips in an unrelated mode", x.F(func(x x.X) {
		ran := false
		h := flg.On[string](mode.Run, func(ctx context.Context, v string) error {
			ran = true
			return nil
		})
		ctx := mode.Into(context.Background(), mode.Help)
		err := h.Handle(ctx, "v")
		x.NoError(err)
		x.False(ran)
	}))
}

func TestOnExact(t *testing.T) {
	t.Run("fires only on exact equality", x.F(func(x x.X) {
		ran := false
		h := flg.OnExact[string](mode.Run, func(ctx context.Context, v string) error {
			ran = true
			return nil
		})
		ctx := mode.Into(context.Background(), mode.Run)
		err := h.Handle(ctx, "v")
		x.NoError(err)
		x.True(ran)
	}))

	t.Run("does not fire for a superset mode", x.F(func(x x.X) {
		ran := false
		h := flg.OnExact[string](mode.Run, func(ctx context.Context, v string) error {
			ran = true
			return nil
		})
		ctx := mode.Into(context.Background(), mode.Run|mode.Pass)
		err := h.Handle(ctx, "v")
		x.NoError(err)
		x.False(ran)
	}))
}

func TestOnHelpAndOnRun(t *testing.T) {
	t.Run("OnHelp fires only in help mode", x.F(func(x x.X) {
		ran := false
		h := flg.OnHelp[string](func(ctx context.Context, v string) error {
			ran = true
			return nil
		})

		err := h.Handle(mode.Into(context.Background(), mode.Help), "v")
		x.NoError(err)
		x.True(ran)

		ran = false
		err = h.Handle(mode.Into(context.Background(), mode.Run), "v")
		x.NoError(err)
		x.False(ran)
	}))

	t.Run("OnRun fires only in run mode", x.F(func(x x.X) {
		ran := false
		h := flg.OnRun[string](func(ctx context.Context, v string) error {
			ran = true
			return nil
		})

		err := h.Handle(mode.Into(context.Background(), mode.Run), "v")
		x.NoError(err)
		x.True(ran)

		ran = false
		err = h.Handle(mode.Into(context.Background(), mode.Help), "v")
		x.NoError(err)
		x.False(ran)
	}))
}

func TestOnTab(t *testing.T) {
	t.Run("runs the tab func when a tab is present", x.F(func(x x.X) {
		ft := &handlerTestFakeTab{}
		ran := false
		h := flg.OnTab[string](func(ctx context.Context, tb tab.Tab) error {
			ran = true
			tb.Value("candidate")
			return nil
		})

		ctx := tab.Into(mode.Into(context.Background(), mode.Tab), ft)
		err := h.Handle(ctx, "")
		x.NoError(err)
		x.True(ran)
		x.Equal([]string{"candidate"}, ft.values)
	}))

	t.Run("no-op when no tab is in the context", x.F(func(x x.X) {
		ran := false
		h := flg.OnTab[string](func(ctx context.Context, tb tab.Tab) error {
			ran = true
			return nil
		})

		ctx := mode.Into(context.Background(), mode.Tab)
		err := h.Handle(ctx, "")
		x.NoError(err)
		x.False(ran)
	}))

	t.Run("OnTap is an alias of OnTab", x.F(func(x x.X) {
		ft := &handlerTestFakeTab{}
		ran := false
		h := flg.OnTap[string](func(ctx context.Context, tb tab.Tab) error {
			ran = true
			return nil
		})

		ctx := tab.Into(mode.Into(context.Background(), mode.Tab), ft)
		err := h.Handle(ctx, "")
		x.NoError(err)
		x.True(ran)

		ran = false
		err = h.Handle(mode.Into(context.Background(), mode.Tab), "")
		x.NoError(err)
		x.False(ran)
	}))
}

func TestOnTabPass(t *testing.T) {
	t.Run("fires in tab|pass mode", x.F(func(x x.X) {
		ft := &handlerTestFakeTab{}
		ran := false
		h := flg.OnTabPass[string](func(ctx context.Context, v string) error {
			ran = true
			return nil
		})

		ctx := tab.Into(mode.Into(context.Background(), mode.Tab|mode.Pass), ft)
		err := h.Handle(ctx, "v")
		x.NoError(err)
		x.True(ran)
	}))

	t.Run("does not fire in tab-only mode", x.F(func(x x.X) {
		ran := false
		h := flg.OnTabPass[string](func(ctx context.Context, v string) error {
			ran = true
			return nil
		})

		ctx := mode.Into(context.Background(), mode.Tab)
		err := h.Handle(ctx, "v")
		x.NoError(err)
		x.False(ran)
	}))

	t.Run("OnTapPass is an alias of OnTabPass", x.F(func(x x.X) {
		ran := false
		h := flg.OnTapPass[string](func(ctx context.Context, v string) error {
			ran = true
			return nil
		})

		ctx := mode.Into(context.Background(), mode.Tab|mode.Pass)
		err := h.Handle(ctx, "v")
		x.NoError(err)
		x.True(ran)

		ran = false
		err = h.Handle(mode.Into(context.Background(), mode.Tab), "v")
		x.NoError(err)
		x.False(ran)
	}))
}
