package xli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/frm"
	"github.com/lesomnus/xli/mode"
	"github.com/stretchr/testify/require"
)

func TestFrameParseSwitches(t *testing.T) {
	new_cmd := func() *xli.Command {
		return &xli.Command{
			Flags: flg.Flags{
				&flg.Switch{Name: "foo"},
			},
		}
	}

	t.Run("switch on", func(t *testing.T) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{"--foo"})
		require.NoError(t, err)

		v := c.Flags.Get("foo").(*flg.Switch)
		require.NotNil(t, v)
		require.Equal(t, true, *v.Value)
	})
	t.Run("switch off", func(t *testing.T) {
		c := new_cmd()
		err := c.Run(t.Context(), nil)
		require.NoError(t, err)

		v := c.Flags.Get("foo").(*flg.Switch)
		require.NotNil(t, v)

		// Note that value is nil since the flag is not given
		require.Nil(t, v.Value)
	})
	t.Run("switch with true", func(t *testing.T) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{"--foo=true"})
		require.NoError(t, err)

		v := c.Flags.Get("foo").(*flg.Switch)
		require.NotNil(t, v)
		require.Equal(t, true, *v.Value)
	})
	t.Run("switch with false", func(t *testing.T) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{"--foo=false"})
		require.NoError(t, err)

		v := c.Flags.Get("foo").(*flg.Switch)
		require.NotNil(t, v)
		require.Equal(t, false, *v.Value)
	})
}

func TestFrameParseFlags(t *testing.T) {
	new_cmd := func() *xli.Command {
		return &xli.Command{
			Flags: flg.Flags{
				&flg.String{Name: "foo"},
				&flg.String{Name: "bar"},
			},
		}
	}

	t.Run("flag with value", func(t *testing.T) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{"--foo", "bar"})
		require.NoError(t, err)

		v := c.Flags.Get("foo").(*flg.String)
		require.NotNil(t, v)
		require.Equal(t, "bar", *v.Value)
	})
	t.Run("flag with value in single arg", func(t *testing.T) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{"--foo=bar"})
		require.NoError(t, err)

		v := c.Flags.Get("foo").(*flg.String)
		require.NotNil(t, v)
		require.Equal(t, "bar", *v.Value)
	})
	t.Run("flag with no value", func(t *testing.T) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{"--foo"})
		require.ErrorContains(t, err, "--foo: no value is given")
	})
	t.Run("flag with no value but flag", func(t *testing.T) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{"--foo", "--bar", "baz"})
		require.ErrorContains(t, err, "--foo: no value is given")
	})
	t.Run("invalid flag syntax", func(t *testing.T) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{
			"---foo=a",
			"--bar=b",
		})
		require.ErrorContains(t, err, "too many dashes: ---foo=a")
	})
}

func TestFrameParseArgs(t *testing.T) {
	t.Run("arg", func(t *testing.T) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
			},
		}

		err := c.Run(t.Context(), []string{"foo"})
		require.NoError(t, err)

		v := c.Args.Get("FOO").(*arg.String)
		require.NotNil(t, v)
		require.Equal(t, "foo", *v.Value)
	})
	t.Run("args", func(t *testing.T) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
				&arg.String{Name: "BAZ"},
			},
		}

		err := c.Run(t.Context(), []string{"foo", "bar", "baz"})
		require.NoError(t, err)

		foo := c.Args.Get("FOO").(*arg.String)
		require.NotNil(t, foo)
		require.Equal(t, "foo", *foo.Value)

		bar := c.Args.Get("BAR").(*arg.String)
		require.NotNil(t, bar)
		require.Equal(t, "bar", *bar.Value)

		baz := c.Args.Get("BAZ").(*arg.String)
		require.NotNil(t, baz)
		require.Equal(t, "baz", *baz.Value)
	})
	t.Run("extra args", func(t *testing.T) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
			},
		}

		err := c.Run(t.Context(), []string{"foo", "bar", "baz", "qux"})
		require.ErrorContains(t, err, `baz: too many arguments`)
	})
	t.Run("extra args with --help", func(t *testing.T) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
			},
		}

		err := c.Run(t.Context(), []string{"--help", "foo", "bar", "baz", "qux"})
		require.NoError(t, err)
	})
	t.Run("less args", func(t *testing.T) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
			},
		}

		err := c.Run(t.Context(), []string{"foo"})
		require.ErrorContains(t, err, `"BAR": required argument not given`)
	})
	t.Run("less args with --help", func(t *testing.T) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
			},
		}

		err := c.Run(t.Context(), []string{"--help", "foo"})
		require.NoError(t, err)
	})
	t.Run("optional", func(t *testing.T) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "BAR", Optional: true},
			},
		}

		err := c.Run(t.Context(), nil)
		require.NoError(t, err)

		bar := c.Args.Get("BAR").(*arg.String).Value
		require.Nil(t, bar)
	})
	t.Run("multiple optional", func(t *testing.T) {
		// TODO: test BAZ is parsed when if BAR parses 0 arguments.
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "BAR", Optional: true},
				&arg.String{Name: "BAZ", Optional: true},
				&arg.String{Name: "QUX", Optional: true},
			},
		}

		err := c.Run(t.Context(), nil)
		require.NoError(t, err)

		bar := c.Args.Get("BAR").(*arg.String).Value
		require.Nil(t, bar)
		baz := c.Args.Get("BAZ").(*arg.String).Value
		require.Nil(t, baz)
		qux := c.Args.Get("QUX").(*arg.String).Value
		require.Nil(t, qux)
	})
	t.Run("optional with optional remains with no args", func(t *testing.T) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "BAR", Optional: true},
				&arg.Remains{Name: "BAZ", Optional: true},
			},
		}

		err := c.Run(t.Context(), nil)
		require.NoError(t, err)

		bar := c.Args.Get("BAR").(*arg.String).Value
		require.Nil(t, bar)
		baz := c.Args.Get("BAZ").(*arg.Remains).Value
		require.Nil(t, baz)
	})
	t.Run("optional with optional remains with arg and remain args", func(t *testing.T) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "BAR", Optional: true},
				&arg.Remains{Name: "BAZ", Optional: true},
			},
		}

		err := c.Run(t.Context(), []string{"bar", "--", "baz", "qux"})
		require.NoError(t, err)

		bar := c.Args.Get("BAR").(*arg.String).Value
		require.NotNil(t, bar)
		require.Equal(t, "bar", *bar)

		baz := c.Args.Get("BAZ").(*arg.Remains).Value
		require.NotNil(t, baz)
		require.NotNil(t, *baz)
		require.Equal(t, []string{"baz", "qux"}, *baz)
	})
	t.Run("optional with optional remains with remain args", func(t *testing.T) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "BAR", Optional: true},
				&arg.Remains{Name: "BAZ", Optional: true},
			},
		}

		err := c.Run(t.Context(), []string{"--", "baz", "qux"})
		require.NoError(t, err)

		bar := c.Args.Get("BAR").(*arg.String).Value
		require.Nil(t, bar)
		baz := c.Args.Get("BAZ").(*arg.Remains).Value
		require.NotNil(t, baz)
		require.NotNil(t, *baz)
		require.Equal(t, []string{"baz", "qux"}, *baz)
	})
	t.Run("optional after required", func(t *testing.T) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR", Optional: true},
			},
		}

		err := c.Run(t.Context(), []string{"foo"})
		require.NoError(t, err)
		require.Equal(t, "foo", *c.Args.Get("FOO").(*arg.String).Value)

		bar := c.Args.Get("BAR").(*arg.String).Value
		require.Nil(t, bar)
	})
	t.Run("consume many", func(t *testing.T) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.RestStrings{Name: "BAR"},
			},
		}

		err := c.Run(t.Context(), []string{"foo", "bar", "baz", "qux"})
		require.NoError(t, err)
		require.Equal(t, "foo", *c.Args.Get("FOO").(*arg.String).Value)

		bar := c.Args.Get("BAR").(*arg.RestStrings).Value
		require.Equal(t, []string{"bar", "baz", "qux"}, bar)
	})
}

func TestFrameParseEndOfCommands(t *testing.T) {
	t.Run("remains", func(t *testing.T) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.Remains{Name: "FOO"},
			},
		}

		err := c.Run(t.Context(), []string{"--", "foo", "bar", "baz"})
		require.NoError(t, err)
		require.Equal(t, []string{"foo", "bar", "baz"}, *c.Args.Get("FOO").(*arg.Remains).Value)
	})
	t.Run("remains after flags and args", func(t *testing.T) {
		c := &xli.Command{
			Flags: flg.Flags{
				&flg.String{Name: "foo"},
				&flg.String{Name: "bar"},
			},
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
				&arg.Remains{Name: "BAZ"},
			},
		}

		err := c.Run(t.Context(), []string{
			"--foo", "foo",
			"--bar", "bar",
			"foo", "bar",
			"--", "baz1", "baz2", "baz3",
		})
		require.NoError(t, err)
		require.Equal(t, []string{"baz1", "baz2", "baz3"}, *c.Args.Get("BAZ").(*arg.Remains).Value)
	})
}

func TestFrameParseComposite(t *testing.T) {
	t.Run("switches and flags", func(t *testing.T) {
		c := &xli.Command{
			Flags: flg.Flags{
				&flg.Switch{Name: "foo"},
				&flg.Switch{Name: "bar"},
				&flg.String{Name: "baz"},
				&flg.String{Name: "qux"},
			},
		}

		err := c.Run(t.Context(), []string{
			"--foo",
			"--baz=a",
			"--bar=false",
			"--qux=b",
		})
		require.NoError(t, err)

		foo := c.Flags.Get("foo").(*flg.Switch)
		require.NotNil(t, foo)
		require.Equal(t, true, *foo.Value)

		bar := c.Flags.Get("bar").(*flg.Switch)
		require.NotNil(t, bar)
		require.Equal(t, false, *bar.Value)

		baz := c.Flags.Get("baz").(*flg.String)
		require.NotNil(t, baz)
		require.Equal(t, "a", *baz.Value)

		qux := c.Flags.Get("qux").(*flg.String)
		require.NotNil(t, qux)
		require.Equal(t, "b", *qux.Value)
	})
	t.Run("switches, flags, and args", func(t *testing.T) {
		c := &xli.Command{
			Flags: flg.Flags{
				&flg.Switch{Name: "foo"},
				&flg.String{Name: "bar"},
			},
			Args: arg.Args{
				&arg.String{Name: "BAZ"},
				&arg.String{Name: "QUX"},
			},
		}

		err := c.Run(t.Context(), []string{
			"--foo",
			"--bar=a",
			"baz",
			"qux",
		})
		require.NoError(t, err)

		foo := c.Flags.Get("foo").(*flg.Switch)
		require.NotNil(t, foo)
		require.Equal(t, true, *foo.Value)

		bar := c.Flags.Get("bar").(*flg.String)
		require.NotNil(t, bar)
		require.Equal(t, "a", *bar.Value)

		baz := c.Args.Get("BAZ").(*arg.String)
		require.NotNil(t, baz)
		require.Equal(t, "baz", *baz.Value)

		qux := c.Args.Get("QUX").(*arg.String)
		require.NotNil(t, qux)
		require.Equal(t, "qux", *qux.Value)
	})
	t.Run("flag in the middle of args", func(t *testing.T) {
		c := &xli.Command{
			Flags: flg.Flags{
				&flg.String{Name: "foo"},
				&flg.String{Name: "bar"},
			},
			Args: arg.Args{
				&arg.String{Name: "BAZ"},
				&arg.String{Name: "QUX"},
			},
		}

		err := c.Run(t.Context(), []string{
			"--foo", "a",
			"baz",
			"--bar", "b",
			"qux",
		})
		require.ErrorContains(t, err, "flags are must be set at the behind")
		require.ErrorContains(t, err, "--bar")
	})
	t.Run("flag with value in single arg in the middle of args", func(t *testing.T) {
		c := &xli.Command{
			Flags: flg.Flags{
				&flg.String{Name: "foo"},
				&flg.String{Name: "bar"},
			},
			Args: arg.Args{
				&arg.String{Name: "BAZ"},
				&arg.String{Name: "QUX"},
			},
		}

		err := c.Run(t.Context(), []string{
			"--foo=a",
			"baz",
			"--bar=b",
			"qux",
		})
		require.ErrorContains(t, err, "flags are must be set at the behind")
		require.ErrorContains(t, err, `--bar="b"`)
	})
	t.Run("subcommand with switches, flags, and args", func(t *testing.T) {
		c := &xli.Command{
			Flags: flg.Flags{
				&flg.Switch{Name: "switch"},
				&flg.String{Name: "flag"},
			},
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
			},
			Commands: []*xli.Command{
				{
					Name: "foo",
					Flags: flg.Flags{
						&flg.Switch{Name: "switch_1"},
						&flg.String{Name: "flag_1"},
					},
					Args: arg.Args{
						&arg.String{Name: "FOO_1"},
						&arg.String{Name: "BAR_1"},
					},
					Commands: []*xli.Command{
						{
							Name: "bar",
							Flags: flg.Flags{
								&flg.Switch{Name: "switch_a"},
								&flg.String{Name: "flag_a"},
							},
							Args: arg.Args{
								&arg.String{Name: "FOO_a"},
								&arg.String{Name: "BAR_a"},
							},
						},
					},
				},
			},
		}

		err := c.Run(t.Context(), []string{
			"--switch", "--flag=flag", "foo", "bar",
			"foo", "--switch_1", "--flag_1=flag_1", "foo_1", "bar_1",
			"bar", "--switch_a", "--flag_a=flag_a", "foo_a", "bar_a",
		})
		require.NoError(t, err)
		require.Equal(t, true, *c.Flags.Get("switch").(*flg.Switch).Value)
		require.Equal(t, "flag", *c.Flags.Get("flag").(*flg.String).Value)
		require.Equal(t, "foo", *c.Args.Get("FOO").(*arg.String).Value)
		require.Equal(t, "bar", *c.Args.Get("BAR").(*arg.String).Value)

		foo := c.Commands.Get("foo")
		require.Equal(t, true, *foo.Flags.Get("switch_1").(*flg.Switch).Value)
		require.Equal(t, "flag_1", *foo.Flags.Get("flag_1").(*flg.String).Value)
		require.Equal(t, "foo_1", *foo.Args.Get("FOO_1").(*arg.String).Value)
		require.Equal(t, "bar_1", *foo.Args.Get("BAR_1").(*arg.String).Value)

		bar := foo.Commands.Get("bar")
		require.Equal(t, true, *bar.Flags.Get("switch_a").(*flg.Switch).Value)
		require.Equal(t, "flag_a", *bar.Flags.Get("flag_a").(*flg.String).Value)
		require.Equal(t, "foo_a", *bar.Args.Get("FOO_a").(*arg.String).Value)
		require.Equal(t, "bar_a", *bar.Args.Get("BAR_a").(*arg.String).Value)
	})
	t.Run("remains after flags, args, and subcommand", func(t *testing.T) {
		c := &xli.Command{
			Flags: flg.Flags{
				&flg.String{Name: "foo"},
				&flg.String{Name: "bar"},
			},
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
			},
			Commands: xli.Commands{
				&xli.Command{
					Name: "foo",
					Flags: flg.Flags{
						&flg.String{Name: "foo"},
						&flg.String{Name: "bar"},
					},
					Args: arg.Args{
						&arg.String{Name: "FOO"},
						&arg.String{Name: "BAR"},
						&arg.Remains{Name: "BAZ"},
					},
				},
			},
		}
		err := c.Run(t.Context(), []string{
			"--foo=foo", "--bar", "bar", "foo", "bar",
			"foo", "--foo=foo", "--bar", "bar", "foo", "bar",
			"--", "baz1", "baz2", "baz3",
		})
		require.NoError(t, err)
		require.Equal(t, []string{"baz1", "baz2", "baz3"}, *c.Commands.Get("foo").Args.Get("BAZ").(*arg.Remains).Value)
	})
}

func TestFrameMode(t *testing.T) {
	ms := []mode.Mode{}
	append_mode := xli.Handle(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
		m := mode.From(ctx)
		ms = append(ms, m)
		return next(ctx)
	})

	c := &xli.Command{
		Commands: xli.Commands{
			&xli.Command{
				Name: "foo",
				Commands: xli.Commands{
					&xli.Command{
						Name:    "bar",
						Handler: append_mode,
					},
				},
				Handler: append_mode,
			},
		},
		Handler: append_mode,
	}

	err := c.Run(t.Context(), []string{"foo", "bar"})
	require.NoError(t, err)
	require.Equal(t, []mode.Mode{
		mode.Run | mode.Pass,
		mode.Run | mode.Pass,
		mode.Run,
	}, ms)

	ms = []mode.Mode{}
	err = c.Run(t.Context(), []string{"foo", "bar", "--help"})
	require.NoError(t, err)
	require.Equal(t, []mode.Mode{
		mode.Help | mode.Pass,
		mode.Help | mode.Pass,
		mode.Help,
	}, ms)
}

func TestFrameAccess(t *testing.T) {
	trace := []string{}
	handler := xli.Handle(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
		f := frm.From(ctx)
		require.NotNil(t, f.Cmd())

		name := f.Cmd().GetName()
		trace = append(trace, name)

		return next(ctx)
	})

	c := &xli.Command{
		Name: "foo",
		Handler: xli.Handle(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			f := frm.From(ctx)
			require.True(t, frm.HasSeq(f, "foo", "bar", "baz"))

			return handler.Handle(ctx, cmd, next)
		}),
		Commands: xli.Commands{
			&xli.Command{
				Name:    "bar",
				Handler: handler,
				Commands: xli.Commands{
					&xli.Command{
						Name:    "baz",
						Handler: handler,
					},
				},
			},
		},
	}

	err := c.Run(t.Context(), []string{"bar", "baz"})
	require.NoError(t, err)
	require.Equal(t, trace, []string{"foo", "bar", "baz"})
}

func TestFrameIos(t *testing.T) {
	t.Run("use standard one if nil", func(t *testing.T) {
		ok := false
		c := &xli.Command{
			Handler: xli.Handle(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
				ok = true
				require.Same(t, cmd.ReadCloser, os.Stdin)
				require.Same(t, cmd.Writer, os.Stdout)
				require.Same(t, cmd.ErrWriter, os.Stderr)
				return next(ctx)
			}),
		}

		err := c.Run(t.Context(), nil)
		require.NoError(t, err)
		require.True(t, ok)
	})
	t.Run("inherit", func(t *testing.T) {
		i := bytes.NewReader([]byte("royale\nwith\ncheese"))
		o := &bytes.Buffer{}
		e := &bytes.Buffer{}

		c := &xli.Command{
			Commands: xli.Commands{
				&xli.Command{
					Name: "foo",
					Handler: xli.Handle(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
						for i := 0; ; i++ {
							var v string
							if _, err := cmd.Scanln(&v); err != nil {
								if errors.Is(err, io.EOF) {
									break
								}
								return err
							}
							if i%2 == 0 {
								cmd.Println(v)
							} else {
								fmt.Fprintln(cmd.ErrWriter, v)
							}
						}
						return next(ctx)
					}),
				},
			},

			ReadCloser: io.NopCloser(i),
			Writer:     o,
			ErrWriter:  e,
		}

		err := c.Run(t.Context(), []string{"foo"})
		require.NoError(t, err)
		require.Equal(t, "royale\ncheese\n", o.String())
		require.Equal(t, "with\n", e.String())
	})
}
