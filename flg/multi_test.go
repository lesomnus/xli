package flg_test

import (
	"context"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/mode"
	"github.com/lesomnus/xli/tab"
)

func TestStrings(t *testing.T) {
	newCmd := func() *xli.Command {
		return &xli.Command{
			Flags: flg.Flags{
				&flg.Strings{Name: "tag", Alias: 't'},
			},
		}
	}

	t.Run("accumulates repeated long flags", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), []string{"--tag", "a", "--tag", "b"}))

		v, ok := flg.Get[[]string](c, "tag")
		x.True(ok)
		x.Equal([]string{"a", "b"}, v)
	}))
	t.Run("accumulates repeated =value forms", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), []string{"--tag=a", "--tag=b", "--tag=c"}))

		v, _ := flg.Get[[]string](c, "tag")
		x.Equal([]string{"a", "b", "c"}, v)
	}))
	t.Run("accumulates repeated aliases", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), []string{"-t", "a", "-t", "b"}))

		v, _ := flg.Get[[]string](c, "tag")
		x.Equal([]string{"a", "b"}, v)
	}))
	t.Run("mixes forms in order", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), []string{"--tag=a", "-t", "b", "--tag", "c"}))

		v, _ := flg.Get[[]string](c, "tag")
		x.Equal([]string{"a", "b", "c"}, v)
	}))
	t.Run("Count is the number of occurrences", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), []string{"--tag=a", "--tag=b"}))
		x.Equal(2, c.Flags.Get("tag").Count())
	}))
	t.Run("single occurrence yields one value", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), []string{"--tag=only"}))

		v, ok := flg.Get[[]string](c, "tag")
		x.True(ok)
		x.Equal([]string{"only"}, v)
	}))
	t.Run("Get is false when not provided", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), nil))

		v, ok := flg.Get[[]string](c, "tag")
		x.False(ok)
		x.Empty(v)
	}))
}

func TestStringsDefault(t *testing.T) {
	newCmd := func() *xli.Command {
		return &xli.Command{
			Flags: flg.Flags{
				&flg.Strings{Name: "tag", Default: []string{"x", "y"}},
			},
		}
	}

	t.Run("Get ignores the default when not provided", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), nil))

		_, ok := flg.Get[[]string](c, "tag")
		x.False(ok)
	}))
	t.Run("MustGet returns the default when not provided", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), nil))
		x.Equal([]string{"x", "y"}, flg.MustGet[[]string](c, "tag"))
	}))
	t.Run("MustGet returns the provided values over the default", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), []string{"--tag=a"}))
		x.Equal([]string{"a"}, flg.MustGet[[]string](c, "tag"))
	}))
	t.Run("MustGet panics when neither provided nor default", x.F(func(x x.X) {
		c := &xli.Command{
			Flags: flg.Flags{&flg.Strings{Name: "tag"}},
		}
		x.NoError(c.Run(t.Context(), nil))

		defer func() { x.NotNil(recover()) }()
		_ = flg.MustGet[[]string](c, "tag")
	}))
}

func TestStringsInfo(t *testing.T) {
	t.Run("type label signals multiplicity", x.F(func(x x.X) {
		info := (&flg.Strings{Name: "tag"}).Info()
		x.Equal("string...", info.Type)
		x.False(info.HasDefault)
	}))
	t.Run("default renders the slice via the element parser", x.F(func(x x.X) {
		info := (&flg.Strings{Name: "tag", Default: []string{"a", "b"}}).Info()
		x.True(info.HasDefault)
		x.Equal(`["a" "b"]`, info.Default)
	}))
	t.Run("required is reported", x.F(func(x x.X) {
		info := (&flg.Strings{Name: "tag", Required: true}).Info()
		x.True(info.Required)
	}))
}

func TestStringsRequired(t *testing.T) {
	newCmd := func() *xli.Command {
		return &xli.Command{
			Flags: flg.Flags{&flg.Strings{Name: "tag", Required: true}},
		}
	}

	t.Run("absent required flag is an error", x.F(func(x x.X) {
		c := newCmd()
		err := c.Run(t.Context(), nil)
		x.ErrorContains(err, "tag")
	}))
	t.Run("provided required flag is accepted", x.F(func(x x.X) {
		c := newCmd()
		x.NoError(c.Run(t.Context(), []string{"--tag=a"}))
	}))
}

func TestStringsNoValue(t *testing.T) {
	t.Run("consumes a value per occurrence", x.F(func(x x.X) {
		var f flg.Flag = &flg.Strings{Name: "tag"}
		x.False(f.NoValue())
	}))
}

func TestStringsCategory(t *testing.T) {
	t.Run("WithCategory tags a Multi flag", x.F(func(x x.X) {
		fs := flg.Flags{}.WithCategory("grouping", &flg.Strings{Name: "tag"})
		x.Equal("grouping", fs.Get("tag").Info().Category)
	}))
}

// recordTab is a minimal tab.Tab that records offered candidate values.
type recordTab struct {
	values []string
}

func (t *recordTab) Value(v string)            { t.values = append(t.values, v) }
func (t *recordTab) ValueD(v string, _ string) { t.values = append(t.values, v) }
func (t *recordTab) Group(string) tab.Tab      { return t }
func (t *recordTab) Files(string)              {}
func (t *recordTab) Dirs()                     {}

func TestStringsCompletion(t *testing.T) {
	t.Run("Tab mode fires OnTab without mutating the value", x.F(func(x x.X) {
		f := &flg.Strings{
			Name: "tag",
			Handler: flg.OnTab[[]string](func(ctx context.Context, t tab.Tab) error {
				t.Value("alpha")
				t.Value("beta")
				return nil
			}),
		}

		rec := &recordTab{}
		ctx := tab.Into(mode.Into(context.Background(), mode.Tab), rec)

		x.NoError(f.Handle(ctx, ""))
		x.Equal([]string{"alpha", "beta"}, rec.values)

		// Completion must not record a value or bump the count.
		x.Equal(0, f.Count())
		x.Empty(f.Value)
	}))

	// During completion the earlier occurrences of the flag being completed are
	// handled in mode.Tab|mode.Pass (not exact mode.Tab), where Multi must fall
	// through to the parse-and-append branch so an OnTab handler on the final
	// occurrence can see the values already entered. This locks that behavior so
	// a broadened exact-Tab guard cannot silently regress it.
	t.Run("Tab|Pass mode still accumulates", x.F(func(x x.X) {
		f := &flg.Strings{Name: "tag"}
		ctx := mode.Into(context.Background(), mode.Tab|mode.Pass)

		x.NoError(f.Handle(ctx, "a"))
		x.NoError(f.Handle(ctx, "b"))

		x.Equal([]string{"a", "b"}, f.Value)
		x.Equal(2, f.Count())
	}))
}

func TestStringsHandlerSnapshot(t *testing.T) {
	// The handler must receive an independent snapshot per occurrence: appending
	// to and retaining the received slice must not be clobbered by the framework
	// appending the next occurrence into its backing array.
	t.Run("retained handler slice is not mutated by later occurrences", x.F(func(x x.X) {
		var snapshots [][]string
		c := &xli.Command{
			Flags: flg.Flags{
				&flg.Strings{
					Name: "tag",
					Handler: flg.Handle[[]string](func(ctx context.Context, v []string) error {
						snapshots = append(snapshots, append(v, "END"))
						return nil
					}),
				},
			},
		}

		x.NoError(c.Run(t.Context(), []string{"--tag", "a", "--tag", "b", "--tag", "c", "--tag", "d"}))

		// The snapshot taken at the 3rd occurrence must still read [a b c END].
		x.Equal([]string{"a", "b", "c", "END"}, snapshots[2])
	}))
}
