package cfg

import (
	"context"
)

// File is a file of its own beside the configuration, such as cr's policy
// file: read from its path alone, with no environment variables or flags over
// it, and reloaded as the configuration is. Of the options, WithScheme,
// WithEnviron (which `${env:}` references are resolved against) and
// WithInterval apply.
type File[T any] struct {
	l *Loader[T]
}

// NewFile is the file at path, which must exist when it is loaded.
func NewFile[T any](path string, opts ...Option) *File[T] {
	if path == "" {
		panic("cfg: NewFile needs a path")
	}
	l := newLoader[T]("", opts)
	l.file = path
	l.root = new(T)
	return &File[T]{l: l}
}

// Path is the path of the file.
func (f *File[T]) Path() string { return f.l.file }

// Load reads the file and makes it Current.
func (f *File[T]) Load() (*Snapshot[T], error) {
	return f.l.Load("", f.l.opts.environ())
}

// Read reads the file, without making it Current.
func (f *File[T]) Read() (*Snapshot[T], error) {
	return f.l.Read("", f.l.opts.environ())
}

// Current is the snapshot in force: the latest one loaded, or nil before the
// first Load.
func (f *File[T]) Current() *Snapshot[T] { return f.l.Current() }

// Reload checks the file once; see Loader.Reload.
func (f *File[T]) Reload() (s *Snapshot[T], changed bool, err error) { return f.l.Reload() }

// Watch reloads the file every interval until ctx is done; see Loader.Watch.
func (f *File[T]) Watch(ctx context.Context, fn func(s *Snapshot[T], err error)) error {
	return f.l.Watch(ctx, fn)
}
