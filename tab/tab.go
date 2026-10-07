// Package tab is where completion candidates go: a handler running for shell
// completion finds a Tab in its context and offers values to it.
//
//	Handler: flg.OnTab[string](func(ctx context.Context, t tab.Tab) error {
//		t.ValueD("json", "JSON output")
//		t.ValueD("yaml", "YAML output")
//		return nil
//	}),
package tab

import "context"

// Tab is the sink of completion candidates. The one a run puts in the context
// writes them for the shell's completion script.
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

// From is the Tab ctx carries, or nil when it is not a completion run.
func From(ctx context.Context) Tab {
	v, ok := ctx.Value(ctxKey{}).(Tab)
	if !ok {
		return nil
	}

	return v
}

// Into is ctx carrying v, for [From].
func Into(ctx context.Context, v Tab) context.Context {
	return context.WithValue(ctx, ctxKey{}, v)
}
