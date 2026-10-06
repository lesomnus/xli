package cfg

import (
	"context"
	"errors"
	"time"
)

// Watch keeps the configuration current with its file until ctx is done.
//
// Every interval (WithInterval, 5 seconds by default) it reads the file. New
// content is loaded once it has read the same on two checks in a row, and an
// empty file is skipped, so that a file caught while it is being written is not
// taken for the configuration. Replacing the file by renaming a new one into
// place, as Kubernetes does with a mounted ConfigMap, avoids the question.
// The new snapshot keeps the environment and the flags of the first load:
// those cannot change under a running process. A snapshot that loads is made
// Current and passed to f; one that fails to load or validate is reported to f
// as an error, once per content, and the snapshot in force stays.
//
// A reload never writes into the root: it is the first load, for the life of
// the process. Read Current, or keep what f is given, for the live values.
// Secrets read from files need no Watch; they re-read their file on use.
//
// It must be called after the first Load (for a loader made by NewFile, Load
// with no arguments).
func (l *Loader[T]) Watch(ctx context.Context, f func(s *Snapshot[T], err error)) error {
	l.mu.Lock()
	in, first := l.in, l.first
	l.mu.Unlock()
	if in == nil {
		return errors.New("cfg: Watch before the first Load")
	}
	if l.file == "" && in.path == "" && first.Path != "" {
		// Keep to the file the first load found: one that disappears is an
		// error, not a reason to fall back to the defaults or to another of
		// the default paths.
		c := *in
		c.path = first.Path
		in = &c
	}

	var (
		pending string // a new content seen once
		failed  string // the content that last failed
	)
	t := time.NewTicker(l.opts.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}

		path, content, err := l.readFile(in)
		if err != nil {
			id := "error:" + err.Error()
			if id != failed {
				failed = id
				f(nil, err)
			}
			continue
		}

		if path != "" && len(content) == 0 {
			// Truncated and not yet written.
			continue
		}

		id := path + "\x00" + revision(path, content)
		if cur := l.cur.Load(); cur != nil && cur.Path+"\x00"+cur.Revision == id {
			pending, failed = "", ""
			continue
		}
		if id != pending {
			pending = id
			continue
		}
		if id == failed {
			continue
		}

		s, err := l.build(in, path, content)
		if err != nil {
			failed = id
			f(nil, err)
			continue
		}
		pending, failed = "", ""
		l.cur.Store(s)
		f(s, nil)
	}
}
