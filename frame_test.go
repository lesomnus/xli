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
	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/mode"
)

func TestFrameParseSwitches(t *testing.T) {
	new_cmd := func() *xli.Command {
		return &xli.Command{
			Flags: flg.Flags{
				&flg.Switch{Name: "foo"},
			},
		}
	}

	t.Run("switch on", x.F(func(x x.X) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{"--foo"})
		x.NoError(err)

		v := c.Flags.Get("foo").(*flg.Switch)
		x.NotNil(v)
		x.Equal(true, *v.Value)
	}))
	t.Run("switch off", x.F(func(x x.X) {
		c := new_cmd()
		err := c.Run(t.Context(), nil)
		x.NoError(err)

		v := c.Flags.Get("foo").(*flg.Switch)
		x.NotNil(v)

		// Note that value is nil since the flag is not given
		x.Nil(v.Value)
	}))
	t.Run("switch with true", x.F(func(x x.X) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{"--foo=true"})
		x.NoError(err)

		v := c.Flags.Get("foo").(*flg.Switch)
		x.NotNil(v)
		x.Equal(true, *v.Value)
	}))
	t.Run("switch with false", x.F(func(x x.X) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{"--foo=false"})
		x.NoError(err)

		v := c.Flags.Get("foo").(*flg.Switch)
		x.NotNil(v)
		x.Equal(false, *v.Value)
	}))
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

	t.Run("flag with value", x.F(func(x x.X) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{"--foo", "bar"})
		x.NoError(err)

		v := c.Flags.Get("foo").(*flg.String)
		x.NotNil(v)
		x.Equal("bar", *v.Value)
	}))
	t.Run("flag with value in single arg", x.F(func(x x.X) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{"--foo=bar"})
		x.NoError(err)

		v := c.Flags.Get("foo").(*flg.String)
		x.NotNil(v)
		x.Equal("bar", *v.Value)
	}))
	t.Run("flag with no value", x.F(func(x x.X) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{"--foo"})
		x.ErrorContains(err, "--foo: no value is given")
	}))
	t.Run("flag with no value but flag", x.F(func(x x.X) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{"--foo", "--bar", "baz"})
		x.ErrorContains(err, "--foo: no value is given")
	}))
	t.Run("invalid flag syntax", x.F(func(x x.X) {
		c := new_cmd()
		err := c.Run(t.Context(), []string{
			"---foo=a",
			"--bar=b",
		})
		x.ErrorContains(err, "too many dashes: ---foo=a")
	}))
}

func TestFrameParseArgs(t *testing.T) {
	t.Run("arg", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
			},
		}

		err := c.Run(t.Context(), []string{"foo"})
		x.NoError(err)

		v := c.Args.Get("FOO").(*arg.String)
		x.NotNil(v)
		x.Equal("foo", *v.Value)
	}))
	t.Run("args", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
				&arg.String{Name: "BAZ"},
			},
		}

		err := c.Run(t.Context(), []string{"foo", "bar", "baz"})
		x.NoError(err)

		foo := c.Args.Get("FOO").(*arg.String)
		x.NotNil(foo)
		x.Equal("foo", *foo.Value)

		bar := c.Args.Get("BAR").(*arg.String)
		x.NotNil(bar)
		x.Equal("bar", *bar.Value)

		baz := c.Args.Get("BAZ").(*arg.String)
		x.NotNil(baz)
		x.Equal("baz", *baz.Value)
	}))
	t.Run("extra args", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
			},
		}

		err := c.Run(t.Context(), []string{"foo", "bar", "baz", "qux"})
		x.ErrorContains(err, `baz: too many arguments`)
	}))
	t.Run("extra args with --help", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
			},
		}

		err := c.Run(t.Context(), []string{"--help", "foo", "bar", "baz", "qux"})
		x.NoError(err)
	}))
	t.Run("less args", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
			},
		}

		err := c.Run(t.Context(), []string{"foo"})
		x.ErrorContains(err, `"BAR": required argument not given`)
	}))
	t.Run("less args with --help", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
			},
		}

		err := c.Run(t.Context(), []string{"--help", "foo"})
		x.NoError(err)
	}))
	t.Run("optional", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "BAR", Optional: true},
			},
		}

		err := c.Run(t.Context(), nil)
		x.NoError(err)

		bar := c.Args.Get("BAR").(*arg.String).Value
		x.Nil(bar)
	}))
	t.Run("multiple optional", x.F(func(x x.X) {
		// TODO: test BAZ is parsed when if BAR parses 0 arguments.
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "BAR", Optional: true},
				&arg.String{Name: "BAZ", Optional: true},
				&arg.String{Name: "QUX", Optional: true},
			},
		}

		err := c.Run(t.Context(), nil)
		x.NoError(err)

		bar := c.Args.Get("BAR").(*arg.String).Value
		x.Nil(bar)
		baz := c.Args.Get("BAZ").(*arg.String).Value
		x.Nil(baz)
		qux := c.Args.Get("QUX").(*arg.String).Value
		x.Nil(qux)
	}))
	t.Run("optional with optional remains with no args", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "BAR", Optional: true},
				&arg.Remains{Name: "BAZ", Optional: true},
			},
		}

		err := c.Run(t.Context(), nil)
		x.NoError(err)

		bar := c.Args.Get("BAR").(*arg.String).Value
		x.Nil(bar)
		baz := c.Args.Get("BAZ").(*arg.Remains).Value
		x.Nil(baz)
	}))
	t.Run("optional with optional remains with arg and remain args", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "BAR", Optional: true},
				&arg.Remains{Name: "BAZ", Optional: true},
			},
		}

		err := c.Run(t.Context(), []string{"bar", "--", "baz", "qux"})
		x.NoError(err)

		bar := c.Args.Get("BAR").(*arg.String).Value
		x.NotNil(bar)
		x.Equal("bar", *bar)

		baz := c.Args.Get("BAZ").(*arg.Remains).Value
		x.NotNil(baz)
		x.NotNil(*baz)
		x.Equal([]string{"baz", "qux"}, *baz)
	}))
	t.Run("optional with optional remains with remain args", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "BAR", Optional: true},
				&arg.Remains{Name: "BAZ", Optional: true},
			},
		}

		err := c.Run(t.Context(), []string{"--", "baz", "qux"})
		x.NoError(err)

		bar := c.Args.Get("BAR").(*arg.String).Value
		x.Nil(bar)
		baz := c.Args.Get("BAZ").(*arg.Remains).Value
		x.NotNil(baz)
		x.NotNil(*baz)
		x.Equal([]string{"baz", "qux"}, *baz)
	}))
	t.Run("optional after required", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR", Optional: true},
			},
		}

		err := c.Run(t.Context(), []string{"foo"})
		x.NoError(err)
		x.Equal("foo", *c.Args.Get("FOO").(*arg.String).Value)

		bar := c.Args.Get("BAR").(*arg.String).Value
		x.Nil(bar)
	}))
	t.Run("consume many", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.String{Name: "FOO"},
				&arg.RestStrings{Name: "BAR"},
			},
		}

		err := c.Run(t.Context(), []string{"foo", "bar", "baz", "qux"})
		x.NoError(err)
		x.Equal("foo", *c.Args.Get("FOO").(*arg.String).Value)

		bar := c.Args.Get("BAR").(*arg.RestStrings).Value
		x.Equal([]string{"bar", "baz", "qux"}, bar)
	}))
}

func TestFrameParseEndOfCommands(t *testing.T) {
	t.Run("remains", x.F(func(x x.X) {
		c := &xli.Command{
			Args: arg.Args{
				&arg.Remains{Name: "FOO"},
			},
		}

		err := c.Run(t.Context(), []string{"--", "foo", "bar", "baz"})
		x.NoError(err)
		x.Equal([]string{"foo", "bar", "baz"}, *c.Args.Get("FOO").(*arg.Remains).Value)
	}))
	t.Run("remains after flags and args", x.F(func(x x.X) {
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
		x.NoError(err)
		x.Equal([]string{"baz1", "baz2", "baz3"}, *c.Args.Get("BAZ").(*arg.Remains).Value)
	}))
}

func TestFrameParseComposite(t *testing.T) {
	t.Run("switches and flags", x.F(func(x x.X) {
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
		x.NoError(err)

		foo := c.Flags.Get("foo").(*flg.Switch)
		x.NotNil(foo)
		x.Equal(true, *foo.Value)

		bar := c.Flags.Get("bar").(*flg.Switch)
		x.NotNil(bar)
		x.Equal(false, *bar.Value)

		baz := c.Flags.Get("baz").(*flg.String)
		x.NotNil(baz)
		x.Equal("a", *baz.Value)

		qux := c.Flags.Get("qux").(*flg.String)
		x.NotNil(qux)
		x.Equal("b", *qux.Value)
	}))
	t.Run("switches, flags, and args", x.F(func(x x.X) {
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
		x.NoError(err)

		foo := c.Flags.Get("foo").(*flg.Switch)
		x.NotNil(foo)
		x.Equal(true, *foo.Value)

		bar := c.Flags.Get("bar").(*flg.String)
		x.NotNil(bar)
		x.Equal("a", *bar.Value)

		baz := c.Args.Get("BAZ").(*arg.String)
		x.NotNil(baz)
		x.Equal("baz", *baz.Value)

		qux := c.Args.Get("QUX").(*arg.String)
		x.NotNil(qux)
		x.Equal("qux", *qux.Value)
	}))
	t.Run("flag in the middle of args", x.F(func(x x.X) {
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
		x.ErrorContains(err, "flags are must be set at the behind")
		x.ErrorContains(err, "--bar")
	}))
	t.Run("flag with value in single arg in the middle of args", x.F(func(x x.X) {
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
		x.ErrorContains(err, "flags are must be set at the behind")
		x.ErrorContains(err, `--bar="b"`)
	}))
	t.Run("subcommand with switches, flags, and args", x.F(func(x x.X) {
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
		x.NoError(err)
		x.Equal(true, *c.Flags.Get("switch").(*flg.Switch).Value)
		x.Equal("flag", *c.Flags.Get("flag").(*flg.String).Value)
		x.Equal("foo", *c.Args.Get("FOO").(*arg.String).Value)
		x.Equal("bar", *c.Args.Get("BAR").(*arg.String).Value)

		foo := c.Commands.Get("foo")
		x.Equal(true, *foo.Flags.Get("switch_1").(*flg.Switch).Value)
		x.Equal("flag_1", *foo.Flags.Get("flag_1").(*flg.String).Value)
		x.Equal("foo_1", *foo.Args.Get("FOO_1").(*arg.String).Value)
		x.Equal("bar_1", *foo.Args.Get("BAR_1").(*arg.String).Value)

		bar := foo.Commands.Get("bar")
		x.Equal(true, *bar.Flags.Get("switch_a").(*flg.Switch).Value)
		x.Equal("flag_a", *bar.Flags.Get("flag_a").(*flg.String).Value)
		x.Equal("foo_a", *bar.Args.Get("FOO_a").(*arg.String).Value)
		x.Equal("bar_a", *bar.Args.Get("BAR_a").(*arg.String).Value)
	}))
	t.Run("remains after flags, args, and subcommand", x.F(func(x x.X) {
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
		x.NoError(err)
		x.Equal([]string{"baz1", "baz2", "baz3"}, *c.Commands.Get("foo").Args.Get("BAZ").(*arg.Remains).Value)
	}))
}

func TestFrameMode(t *testing.T) {
	x := x.New(t)

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
	x.NoError(err)
	x.Equal([]mode.Mode{
		mode.Run | mode.Pass,
		mode.Run | mode.Pass,
		mode.Run,
	}, ms)

	ms = []mode.Mode{}
	err = c.Run(t.Context(), []string{"foo", "bar", "--help"})
	x.NoError(err)
	x.Equal([]mode.Mode{
		mode.Help | mode.Pass,
		mode.Help | mode.Pass,
		mode.Help,
	}, ms)
}

func TestFrameAccess(t *testing.T) {
	x := x.New(t)

	trace := []string{}
	handler := xli.Handle(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
		f := frm.From(ctx)
		x.NotNil(f.Cmd())

		name := f.Cmd().GetName()
		trace = append(trace, name)

		return next(ctx)
	})

	c := &xli.Command{
		Name: "foo",
		Handler: xli.Handle(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			f := frm.From(ctx)
			x.True(frm.HasSeq(f, "foo", "bar", "baz"))

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
	x.NoError(err)
	x.Equal(trace, []string{"foo", "bar", "baz"})
}

func TestFrameIos(t *testing.T) {
	t.Run("use standard one if nil", x.F(func(x x.X) {
		ok := false
		c := &xli.Command{
			Handler: xli.Handle(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
				ok = true
				x.Same(cmd.ReadCloser, os.Stdin)
				x.Same(cmd.Writer, os.Stdout)
				x.Same(cmd.ErrWriter, os.Stderr)
				return next(ctx)
			}),
		}

		err := c.Run(t.Context(), nil)
		x.NoError(err)
		x.True(ok)
	}))
	t.Run("inherit", x.F(func(x x.X) {
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
		x.NoError(err)
		x.Equal("royale\ncheese\n", o.String())
		x.Equal("with\n", e.String())
	}))
}
