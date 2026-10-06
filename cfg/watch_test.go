package cfg_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/lesomnus/xli/cfg"
	"github.com/lesomnus/xli/internal/x"
)

type event struct {
	s   *cfg.Snapshot[Config]
	err error
}

// watch starts l.Watch and returns its events.
func watch(t *testing.T, l *cfg.Loader[Config]) <-chan event {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ch := make(chan event, 16)
	go l.Watch(ctx, func(s *cfg.Snapshot[Config], err error) {
		ch <- event{s, err}
	})
	return ch
}

func next(t *testing.T, ch <-chan event) event {
	t.Helper()
	select {
	case e := <-ch:
		return e
	case <-time.After(5 * time.Second):
		t.Fatal("no reload")
		return event{}
	}
}

func quiet(t *testing.T, ch <-chan event) {
	t.Helper()
	select {
	case e := <-ch:
		t.Fatalf("unexpected reload: %+v", e)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestWatch(t *testing.T) {
	x := x.New(t)
	p := write(t, "name: one\n")
	c := Config{}
	l := cfg.New("app", &c, cfg.WithInterval(10*time.Millisecond))
	first, err := l.Load(p, env("APP_DB_DSN=env"))
	x.NoError(err)

	ch := watch(t, l)
	quiet(t, ch)

	rotate(t, p, "name: two\n")
	e := next(t, ch)
	x.NoError(e.err)
	x.Equal("two", e.s.Config.Name)
	x.Equal("env", e.s.Config.Db.Dsn, "the environment of the first load")
	x.NotEqual(first.Revision, e.s.Revision)
	x.Same(e.s, l.Current())
	x.Equal("one", c.Name, "the root holds the first load")

	rotate(t, p, "name: [not, a, string]\n")
	e = next(t, ch)
	x.ErrorContains(e.err, "name: want a single value")
	x.Equal("two", l.Current().Config.Name, "a bad file keeps the snapshot in force")

	quiet(t, ch) // the same failure is reported once

	rotate(t, p, "name: two\n")
	quiet(t, ch) // back to the content in force: nothing changed

	rotate(t, p, "name: three\n")
	e = next(t, ch)
	x.NoError(e.err)
	x.Equal("three", e.s.Config.Name)
	quiet(t, ch)
}

func TestWatchEmptyFile(t *testing.T) {
	x := x.New(t)
	p := write(t, "name: one\n")
	l := cfg.New("app", &Config{}, cfg.WithInterval(10*time.Millisecond))
	_, err := l.Load(p, nil)
	x.NoError(err)

	ch := watch(t, l)
	rotate(t, p, "")
	quiet(t, ch) // taken for a file being written
	x.Equal("one", l.Current().Config.Name)
}

func TestWatchMissingFile(t *testing.T) {
	x := x.New(t)
	p := write(t, "name: one\n")
	l := cfg.New("app", &Config{}, cfg.WithInterval(10*time.Millisecond))
	_, err := l.Load(p, nil)
	x.NoError(err)

	ch := watch(t, l)
	x.NoError(os.Remove(p))
	e := next(t, ch)
	x.True(errors.Is(e.err, os.ErrNotExist))
	quiet(t, ch)
	x.Equal("one", l.Current().Config.Name)
}

func TestWatchBeforeLoad(t *testing.T) {
	l := cfg.New("app", &Config{})
	err := l.Watch(context.Background(), func(*cfg.Snapshot[Config], error) {})
	x.New(t).ErrorContains(err, "before the first Load")
}

func TestWatchFile(t *testing.T) {
	x := x.New(t)
	p := write(t, "name: one\n")
	l := cfg.NewFile[Config](p, cfg.WithInterval(10*time.Millisecond))
	_, err := l.Load("", nil)
	x.NoError(err)

	ch := watch(t, l)
	rotate(t, p, "name: two\nnmae: typo\n")
	e := next(t, ch)
	x.ErrorContains(e.err, "nmae: nothing reads this key")

	rotate(t, p, "name: two\n")
	e = next(t, ch)
	x.NoError(e.err)
	x.Equal("two", e.s.Config.Name)
}
