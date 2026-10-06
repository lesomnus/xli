package cfg

import (
	"context"
	"errors"
	"time"
)

// check is what came of one check of the file.
type check int

const (
	// unchanged: the file reads as the snapshot in force.
	unchanged check = iota
	// waiting: new content, read once; it is loaded when it reads the same at
	// the next check.
	waiting
	// reloaded: new content was loaded, and is the snapshot in force.
	reloaded
)

// Reload checks the file once, as Watch does every interval. New content is
// loaded once it reads the same at two checks in a row, so that a file caught
// while it is being written is not taken for the configuration; a file
// replaced by renaming a new one into place, as Kubernetes does with a mounted
// ConfigMap, is loaded at the second check too. The new snapshot keeps the
// environment and the flags of the last Load: those cannot change under a
// running process.
//
// s is the snapshot in force after the check, and changed reports that it is a
// new one. err is why the file could not be read, or its content did not load
// or validate; the snapshot in force stays, and the content is tried again at
// the next check.
//
// It keeps to the file a load found: one that disappears is an error, not a
// reason to fall back to the defaults or to another of the default paths.
//
// A reload never writes into the root, which holds the first load for the life
// of the process: read Current, or keep s, for the live values. Secrets read
// from files need no reload; they re-read their file on use.
func (l *Loader[T]) Reload() (s *Snapshot[T], changed bool, err error) {
	s, c, err := l.reload()
	return s, c == reloaded, err
}

func (l *Loader[T]) reload() (*Snapshot[T], check, error) {
	l.mu.Lock()
	in, at := l.in, l.at
	l.mu.Unlock()
	if in == nil {
		return nil, unchanged, errors.New("cfg: reload before the first Load")
	}

	l.reloading.Lock()
	defer l.reloading.Unlock()

	cur := l.cur.Load()
	if at != "" {
		c := *in
		c.path = at
		in = &c
	}
	path, content, err := l.readFile(in)
	if err != nil {
		l.pending = ""
		return cur, unchanged, err
	}

	id := path + "\x00" + revision(path, content)
	if cur.Path+"\x00"+cur.Revision == id {
		l.pending = ""
		return cur, unchanged, nil
	}
	if id != l.pending {
		l.pending = id
		return cur, waiting, nil
	}

	s, err := l.build(in, path, content)
	if err != nil {
		// Still pending: tried again at the next check, as what failed may
		// be a scheme's server, or a file a Validate looks at.
		return cur, unchanged, err
	}
	l.pending = ""
	if path != "" {
		l.mu.Lock()
		l.at = path
		l.mu.Unlock()
	}
	l.cur.Store(s)
	return s, reloaded, nil
}

// Watch reloads the configuration every interval (WithInterval, 5 seconds by
// default) until ctx is done; see Reload. f is called
//
//   - with a new snapshot, made Current, when the file's content changed;
//   - with an error when the file cannot be read, or its content does not
//     load or validate: once for each error, though the content is tried
//     again at every check;
//   - with the snapshot in force when the file reads as it again after an
//     error, so that what reported the error can tell it is over.
//
// It must be called after the first Load.
func (l *Loader[T]) Watch(ctx context.Context, f func(s *Snapshot[T], err error)) error {
	l.mu.Lock()
	in := l.in
	l.mu.Unlock()
	if in == nil {
		return errors.New("cfg: Watch before the first Load")
	}

	t := time.NewTicker(l.opts.interval)
	defer t.Stop()

	// failed is the error last reported, until the file reads well again.
	failed := ""
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}

		s, c, err := l.reload()
		switch {
		case err != nil:
			if msg := err.Error(); msg != failed {
				failed = msg
				f(nil, err)
			}
		case c == reloaded:
			failed = ""
			f(s, nil)
		case c == unchanged && failed != "":
			failed = ""
			f(s, nil)
		}
	}
}
