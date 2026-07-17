package arg_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/mode"
	"github.com/lesomnus/xli/tab"
)

type handlerTestTab struct {
	values []string
}

func (t *handlerTestTab) Value(v string)               { t.values = append(t.values, v) }
func (t *handlerTestTab) ValueD(v string, desc string) { t.values = append(t.values, v) }
func (t *handlerTestTab) Group(name string) tab.Tab    { return t }
func (t *handlerTestTab) Files(pattern string)         {}
func (t *handlerTestTab) Dirs()                        {}

func TestWrap(t *testing.T) {
	t.Run("runs in order", x.F(func(x x.X) {
		order := []string{}
		h := arg.Wrap(
			arg.Handle(func(ctx context.Context, v string) error {
				order = append(order, "a")
				return nil
			}),
			arg.Handle(func(ctx context.Context, v string) error {
				order = append(order, "b")
				return nil
			}),
		)
		err := h.Handle(context.Background(), "v")
		x.NoError(err)
		x.Equal([]string{"a", "b"}, order)
	}))

	t.Run("short-circuits on error", x.F(func(x x.X) {
		order := []string{}
		want := errors.New("boom")
		h := arg.Wrap(
			arg.Handle(func(ctx context.Context, v string) error {
				order = append(order, "a")
				return want
			}),
			arg.Handle(func(ctx context.Context, v string) error {
				order = append(order, "b")
				return nil
			}),
		)
		err := h.Handle(context.Background(), "v")
		x.ErrorContains(err, "boom")
		x.Equal([]string{"a"}, order)
	}))

	t.Run("empty is no-op", x.F(func(x x.X) {
		h := arg.Wrap[string]()
		err := h.Handle(context.Background(), "v")
		x.NoError(err)
	}))
}

func TestOn(t *testing.T) {
	t.Run("runs when mode subset matches", x.F(func(x x.X) {
		called := false
		h := arg.On(mode.Run, func(ctx context.Context, v string) error {
			called = true
			return nil
		})
		ctx := mode.Into(context.Background(), mode.Run|mode.Pass)
		err := h.Handle(ctx, "v")
		x.NoError(err)
		x.True(called)
	}))

	t.Run("skips when mode not subset", x.F(func(x x.X) {
		called := false
		h := arg.On(mode.Run, func(ctx context.Context, v string) error {
			called = true
			return nil
		})
		ctx := mode.Into(context.Background(), mode.Help)
		err := h.Handle(ctx, "v")
		x.NoError(err)
		x.False(called)
	}))
}

func TestOnExact(t *testing.T) {
	t.Run("runs on exact match", x.F(func(x x.X) {
		got := ""
		h := arg.OnExact(mode.Run, func(ctx context.Context, v string) error {
			got = v
			return nil
		})
		ctx := mode.Into(context.Background(), mode.Run)
		err := h.Handle(ctx, "hello")
		x.NoError(err)
		x.Equal("hello", got)
	}))

	t.Run("skips when not exact", x.F(func(x x.X) {
		called := false
		h := arg.OnExact(mode.Run, func(ctx context.Context, v string) error {
			called = true
			return nil
		})
		ctx := mode.Into(context.Background(), mode.Run|mode.Pass)
		err := h.Handle(ctx, "v")
		x.NoError(err)
		x.False(called)
	}))
}

func TestOnRunOnHelp(t *testing.T) {
	t.Run("OnRun gates on Run", x.F(func(x x.X) {
		called := false
		h := arg.OnRun(func(ctx context.Context, v string) error {
			called = true
			return nil
		})
		x.NoError(h.Handle(mode.Into(context.Background(), mode.Run), "v"))
		x.True(called)

		called = false
		x.NoError(h.Handle(mode.Into(context.Background(), mode.Help), "v"))
		x.False(called)
	}))

	t.Run("OnHelp gates on Help", x.F(func(x x.X) {
		called := false
		h := arg.OnHelp(func(ctx context.Context, v string) error {
			called = true
			return nil
		})
		x.NoError(h.Handle(mode.Into(context.Background(), mode.Help), "v"))
		x.True(called)

		called = false
		x.NoError(h.Handle(mode.Into(context.Background(), mode.Run), "v"))
		x.False(called)
	}))
}

func TestOnTab(t *testing.T) {
	t.Run("runs tab func when tab present", x.F(func(x x.X) {
		ft := &handlerTestTab{}
		called := false
		h := arg.OnTab[string](func(ctx context.Context, t tab.Tab) {
			called = true
			t.Value("suggest")
		})
		ctx := tab.Into(mode.Into(context.Background(), mode.Tab), ft)
		err := h.Handle(ctx, "")
		x.NoError(err)
		x.True(called)
		x.Equal([]string{"suggest"}, ft.values)
	}))

	t.Run("no-op when tab absent", x.F(func(x x.X) {
		called := false
		h := arg.OnTab[string](func(ctx context.Context, t tab.Tab) {
			called = true
		})
		ctx := mode.Into(context.Background(), mode.Tab)
		err := h.Handle(ctx, "")
		x.NoError(err)
		x.False(called)
	}))

	t.Run("no-op when not tab mode", x.F(func(x x.X) {
		ft := &handlerTestTab{}
		called := false
		h := arg.OnTab[string](func(ctx context.Context, t tab.Tab) {
			called = true
		})
		ctx := tab.Into(mode.Into(context.Background(), mode.Run), ft)
		err := h.Handle(ctx, "")
		x.NoError(err)
		x.False(called)
	}))
}
