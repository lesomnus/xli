package cfg_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lesomnus/xli/cfg"
	"github.com/lesomnus/xli/internal/x"
)

type event struct {
	s   *cfg.Snapshot[Config]
	err error
}

type watcher interface {
	Watch(ctx context.Context, f func(*cfg.Snapshot[Config], error)) error
}

// watch starts w.Watch and returns its events.
func watch(t *testing.T, w watcher) <-chan event {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ch := make(chan event, 16)
	go w.Watch(ctx, func(s *cfg.Snapshot[Config], err error) {
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
	e = next(t, ch)
	x.NoError(e.err, "back to the content in force: the failure is over")
	x.Same(l.Current(), e.s)

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
	e := next(t, ch)
	x.NoError(e.err)
	x.Equal("", e.s.Config.Name, "an empty file that stays empty is the configuration")
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

func TestWatchKeepsToAFileFoundLater(t *testing.T) {
	x := x.New(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "app.yaml")
	l := cfg.New("app", &Config{Name: "default"}, cfg.WithPaths(p), cfg.WithInterval(10*time.Millisecond))
	_, err := l.Load("", nil)
	x.NoError(err)

	ch := watch(t, l)
	writeAt(t, p, "name: from-file\n")
	e := next(t, ch)
	x.NoError(e.err)
	x.Equal("from-file", e.s.Config.Name)

	x.NoError(os.Remove(p))
	e = next(t, ch)
	x.True(errors.Is(e.err, os.ErrNotExist), "not the defaults again")
	x.Equal("from-file", l.Current().Config.Name)
}

func TestWatchRetries(t *testing.T) {
	x := x.New(t)
	up := atomic.Bool{}
	p := write(t, "name: one\n")
	l := cfg.New("app", &Config{}, cfg.WithInterval(10*time.Millisecond), cfg.WithScheme("vault", func(ref string) ([]byte, error) {
		if !up.Load() {
			return nil, errors.New("vault is down")
		}
		return []byte(ref), nil
	}))
	_, err := l.Load(p, nil)
	x.NoError(err)

	ch := watch(t, l)
	rotate(t, p, "name: ${vault:two}\n")
	e := next(t, ch)
	x.ErrorContains(e.err, "vault is down")
	quiet(t, ch)

	up.Store(true)
	e = next(t, ch)
	x.NoError(e.err, "the same content is tried again")
	x.Equal("two", e.s.Config.Name)
}

func TestReload(t *testing.T) {
	x := x.New(t)
	p := write(t, "name: one\n")
	l := cfg.New("app", &Config{})
	_, _, err := l.Reload()
	x.ErrorContains(err, "before the first Load")

	first, err := l.Load(p, nil)
	x.NoError(err)

	s, changed, err := l.Reload()
	x.NoError(err)
	x.False(changed)
	x.Same(first, s)

	rotate(t, p, "name: two\n")
	_, changed, err = l.Reload()
	x.NoError(err)
	x.False(changed, "read once: it may be being written")
	s, changed, err = l.Reload()
	x.NoError(err)
	x.True(changed)
	x.Equal("two", s.Config.Name)
}

func TestWatchBeforeLoad(t *testing.T) {
	l := cfg.New("app", &Config{})
	err := l.Watch(context.Background(), func(*cfg.Snapshot[Config], error) {})
	x.New(t).ErrorContains(err, "before the first Load")
}

func TestWatchFile(t *testing.T) {
	x := x.New(t)
	p := write(t, "name: one\n")
	f := cfg.NewFile[Config](p, cfg.WithInterval(10*time.Millisecond))
	_, err := f.Load()
	x.NoError(err)

	ch := watch(t, f)
	rotate(t, p, "name: two\nnmae: typo\n")
	e := next(t, ch)
	x.ErrorContains(e.err, "nmae: nothing reads this key")

	rotate(t, p, "name: two\n")
	e = next(t, ch)
	x.NoError(e.err)
	x.Equal("two", e.s.Config.Name)
}
