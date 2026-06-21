package tab

import "context"

type Tab interface {
	// Value adds a completion candidate.
	Value(v string)
	// ValueD adds a completion candidate with a description.
	ValueD(v string, desc string)
	// Group returns a Tab whose candidates are shown under the given heading.
	// Implementations that do not support grouping may return the receiver.
	Group(name string) Tab
	// Files requests filename completion. An empty pattern matches any file;
	// otherwise it is a shell glob such as "*.go".
	Files(pattern string)
	// Dirs requests directory-only completion.
	Dirs()
}

type ctxKey struct{}

func From(ctx context.Context) Tab {
	v, ok := ctx.Value(ctxKey{}).(Tab)
	if !ok {
		return nil
	}

	return v
}

func Into(ctx context.Context, v Tab) context.Context {
	return context.WithValue(ctx, ctxKey{}, v)
}
