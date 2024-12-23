package xli_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/mode"
	"github.com/stretchr/testify/require"
)

func TestCommandLifeCycle(t *testing.T) {
	appender := func(vs *[]string, v string, err error) xli.Action {
		return func(ctx context.Context, cmd *xli.Command) (context.Context, error) {
			(*vs) = append((*vs), v)
			return ctx, err
		}
	}

	t.Run("no actions", func(t *testing.T) {
		c := &xli.Command{}

		_, err := c.Run(context.TODO(), nil)
		require.NoError(t, err)
	})
	t.Run("actions", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			PreAction:  appender(&vs, "pre", nil),
			Action:     appender(&vs, "body", nil),
			PostAction: appender(&vs, "post", nil),
		}

		_, err := c.Run(context.TODO(), nil)
		require.NoError(t, err)
		require.Equal(t, []string{"pre", "body", "post"}, vs)
	})
	t.Run("subcommand", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			PreAction:  appender(&vs, "pre", nil),
			Action:     appender(&vs, "body", nil),
			PostAction: appender(&vs, "post", nil),
			Commands: []*xli.Command{
				{
					Name:       "foo",
					PreAction:  appender(&vs, "sub_1-pre", nil),
					Action:     appender(&vs, "sub_1-body", nil),
					PostAction: appender(&vs, "sub_1-post", nil),
				},
			},
		}

		_, err := c.Run(context.TODO(), []string{"foo"})
		require.NoError(t, err)
		require.Equal(t, []string{
			"pre", "body",
			/* foo */ "sub_1-pre", "sub_1-body",
			/* foo */ "sub_1-post",
			"post",
		}, vs)
	})
	t.Run("nested subcommand", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			PreAction:  appender(&vs, "pre", nil),
			Action:     appender(&vs, "body", nil),
			PostAction: appender(&vs, "post", nil),
			Commands: []*xli.Command{
				{
					Name:       "foo",
					PreAction:  appender(&vs, "sub_1-pre", nil),
					Action:     appender(&vs, "sub_1-body", nil),
					PostAction: appender(&vs, "sub_1-post", nil),
					Commands: []*xli.Command{
						{
							Name:       "bar",
							PreAction:  appender(&vs, "sub_a-pre", nil),
							Action:     appender(&vs, "sub_a-body", nil),
							PostAction: appender(&vs, "sub_a-post", nil),
						},
					},
				},
			},
		}

		_, err := c.Run(context.TODO(), []string{"foo", "bar"})
		require.NoError(t, err)
		require.Equal(t, []string{
			"pre", "body",
			/* foo */ "sub_1-pre", "sub_1-body",
			/* | bar */ "sub_a-pre", "sub_a-body",
			/* | bar */ "sub_a-post",
			/* foo */ "sub_1-post",
			"post",
		}, vs)
	})
	t.Run("error in PreAction", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			PreAction:  appender(&vs, "pre", errors.New("pre")),
			Action:     appender(&vs, "body", nil),
			PostAction: appender(&vs, "post", nil),
		}

		_, err := c.Run(context.TODO(), nil)
		require.ErrorContains(t, err, "pre")
		require.Equal(t, []string{"pre", "post"}, vs)
	})
	t.Run("error in Action", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			PreAction:  appender(&vs, "pre", nil),
			Action:     appender(&vs, "body", errors.New("body")),
			PostAction: appender(&vs, "post", nil),
		}

		_, err := c.Run(context.TODO(), nil)
		require.ErrorContains(t, err, "body")
		require.Equal(t, []string{"pre", "body", "post"}, vs)
	})
	t.Run("error in PostAction", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			PreAction:  appender(&vs, "pre", nil),
			Action:     appender(&vs, "body", nil),
			PostAction: appender(&vs, "post", errors.New("post")),
		}

		_, err := c.Run(context.TODO(), nil)
		require.ErrorContains(t, err, "post")
		require.Equal(t, []string{"pre", "body", "post"}, vs)
	})
	t.Run("error in PreAction with subcommand", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			PreAction:  appender(&vs, "pre", errors.New("pre")),
			Action:     appender(&vs, "body", nil),
			PostAction: appender(&vs, "post", nil),
			Commands: []*xli.Command{
				{
					Name:       "foo",
					PreAction:  appender(&vs, "sub_1-pre", nil),
					Action:     appender(&vs, "sub_1-body", nil),
					PostAction: appender(&vs, "sub_1-post", nil),
				},
			},
		}

		_, err := c.Run(context.TODO(), []string{"foo"})
		require.ErrorContains(t, err, "pre")
		require.Equal(t, []string{"pre", "post"}, vs)
	})
	t.Run("error in Action with subcommand", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			PreAction:  appender(&vs, "pre", nil),
			Action:     appender(&vs, "body", errors.New("body")),
			PostAction: appender(&vs, "post", nil),
			Commands: []*xli.Command{
				{
					Name:       "foo",
					PreAction:  appender(&vs, "sub_1-pre", nil),
					Action:     appender(&vs, "sub_1-body", nil),
					PostAction: appender(&vs, "sub_1-post", nil),
				},
			},
		}

		_, err := c.Run(context.TODO(), []string{"foo"})
		require.ErrorContains(t, err, "body")
		require.Equal(t, []string{"pre", "body", "post"}, vs)
	})
	t.Run("error in PostAction with subcommand", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			PreAction:  appender(&vs, "pre", nil),
			Action:     appender(&vs, "body", nil),
			PostAction: appender(&vs, "post", errors.New("post")),
			Commands: []*xli.Command{
				{
					Name:       "foo",
					PreAction:  appender(&vs, "sub_1-pre", nil),
					Action:     appender(&vs, "sub_1-body", nil),
					PostAction: appender(&vs, "sub_1-post", nil),
				},
			},
		}

		_, err := c.Run(context.TODO(), []string{"foo"})
		require.ErrorContains(t, err, "post")
		require.Equal(t, []string{
			"pre", "body",
			/* foo */ "sub_1-pre", "sub_1-body",
			/* foo */ "sub_1-post",
			"post",
		}, vs)
	})
	t.Run("error in PostAction of subcommand", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			PreAction:  appender(&vs, "pre", nil),
			Action:     appender(&vs, "body", nil),
			PostAction: appender(&vs, "post", nil),
			Commands: []*xli.Command{
				{
					Name:       "foo",
					PreAction:  appender(&vs, "sub_1-pre", nil),
					Action:     appender(&vs, "sub_1-body", nil),
					PostAction: appender(&vs, "sub_1-post", errors.New("sub_1-post")),
				},
			},
		}

		_, err := c.Run(context.TODO(), []string{"foo"})
		require.ErrorContains(t, err, "sub_1-post")
		require.Equal(t, []string{
			"pre", "body",
			/* foo */ "sub_1-pre", "sub_1-body",
			/* foo */ "sub_1-post",
			"post",
		}, vs)
	})
	t.Run("errors in PostActions", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			PreAction:  appender(&vs, "pre", nil),
			Action:     appender(&vs, "body", nil),
			PostAction: appender(&vs, "post", errors.New("post")),
			Commands: []*xli.Command{
				{
					Name:       "foo",
					PreAction:  appender(&vs, "sub_1-pre", nil),
					Action:     appender(&vs, "sub_1-body", nil),
					PostAction: appender(&vs, "sub_1-post", errors.New("sub_1-post")),
				},
			},
		}

		_, err := c.Run(context.TODO(), []string{"foo"})
		require.ErrorContains(t, err, "sub_1-post")
		require.ErrorContains(t, err, "post")
		require.Equal(t, []string{
			"pre", "body",
			/* foo */ "sub_1-pre", "sub_1-body",
			/* foo */ "sub_1-post",
			"post",
		}, vs)
	})
	t.Run("error while parsing flags", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			PreAction:  appender(&vs, "pre", nil),
			Action:     appender(&vs, "body", nil),
			PostAction: appender(&vs, "post", nil),
			Commands: []*xli.Command{
				{
					Name: "foo",
					Flags: xli.Flags{
						&flg.String{
							Name: "flag",
							Action: func(ctx context.Context, cmd *xli.Command, v string) (context.Context, error) {
								vs = append(vs, "flag")
								return nil, fmt.Errorf("%v for flag", v)
							},
						},
					},
					PreAction:  appender(&vs, "sub_1-pre", nil),
					Action:     appender(&vs, "sub_1-body", nil),
					PostAction: appender(&vs, "sub_1-post", nil),
					Commands: []*xli.Command{
						{
							Name:       "bar",
							PreAction:  appender(&vs, "sub_a-pre", nil),
							Action:     appender(&vs, "sub_a-body", nil),
							PostAction: appender(&vs, "sub_a-post", nil),
						},
					},
				},
			},
		}

		_, err := c.Run(context.TODO(), []string{"foo", "--flag=value", "bar"})
		require.ErrorContains(t, err, `invalid flag: --flag="value": value for flag`)
		require.Equal(t, []string{
			"pre", "body",
			/* foo */ "sub_1-pre", "flag",
			/* foo */ "sub_1-post",
			"post",
		}, vs)
	})
	t.Run("error while parsing args", func(t *testing.T) {
		vs := []string{}
		c := &xli.Command{
			PreAction:  appender(&vs, "pre", nil),
			Action:     appender(&vs, "body", nil),
			PostAction: appender(&vs, "post", nil),
			Commands: []*xli.Command{
				{
					Name: "foo",
					Args: xli.Args{
						&arg.String{
							Name: "ARG",
							Action: func(ctx context.Context, cmd *xli.Command, v string) (context.Context, error) {
								vs = append(vs, "arg")
								return nil, fmt.Errorf("%v for ARG", v)
							},
						},
					},
					PreAction:  appender(&vs, "sub_1-pre", nil),
					Action:     appender(&vs, "sub_1-body", nil),
					PostAction: appender(&vs, "sub_1-post", nil),
					Commands: []*xli.Command{
						{
							Name:       "bar",
							PreAction:  appender(&vs, "sub_a-pre", nil),
							Action:     appender(&vs, "sub_a-body", nil),
							PostAction: appender(&vs, "sub_a-post", nil),
						},
					},
				},
			},
		}

		_, err := c.Run(context.TODO(), []string{"foo", "arg", "bar"})
		require.ErrorContains(t, err, `invalid argument: "arg": arg for ARG`)
		require.Equal(t, []string{
			"pre", "body",
			/* foo */ "sub_1-pre", "arg",
			/* foo */ "sub_1-post",
			"post",
		}, vs)
	})
}

func TestCommandParse(t *testing.T) {
	t.Run("switch", func(t *testing.T) {
		c := &xli.Command{
			Flags: xli.Flags{
				&flg.Switch{Name: "foo"},
			},
		}

		_, err := c.Run(context.TODO(), []string{"--foo"})
		require.NoError(t, err)

		v := c.Flags.Get("foo").(*flg.Switch)
		require.NotNil(t, v)
		require.Equal(t, true, *v.Value)
	})
	t.Run("switch off", func(t *testing.T) {
		c := &xli.Command{
			Flags: xli.Flags{
				&flg.Switch{Name: "foo"},
			},
		}

		_, err := c.Run(context.TODO(), nil)
		require.NoError(t, err)

		v := c.Flags.Get("foo").(*flg.Switch)
		require.NotNil(t, v)

		// Note that value is nil since the flag is not given
		require.Nil(t, v.Value)
	})
	t.Run("switch with true", func(t *testing.T) {
		c := &xli.Command{
			Flags: xli.Flags{
				&flg.Switch{Name: "foo"},
			},
		}

		_, err := c.Run(context.TODO(), []string{"--foo=true"})
		require.NoError(t, err)

		v := c.Flags.Get("foo").(*flg.Switch)
		require.NotNil(t, v)
		require.Equal(t, true, *v.Value)
	})
	t.Run("switch with false", func(t *testing.T) {
		c := &xli.Command{
			Flags: xli.Flags{
				&flg.Switch{Name: "foo"},
			},
		}

		_, err := c.Run(context.TODO(), []string{"--foo=false"})
		require.NoError(t, err)

		v := c.Flags.Get("foo").(*flg.Switch)
		require.NotNil(t, v)
		require.Equal(t, false, *v.Value)
	})
	t.Run("flag with value", func(t *testing.T) {
		c := &xli.Command{
			Flags: xli.Flags{
				&flg.String{Name: "foo"},
			},
		}

		_, err := c.Run(context.TODO(), []string{"--foo", "bar"})
		require.NoError(t, err)

		v := c.Flags.Get("foo").(*flg.String)
		require.NotNil(t, v)
		require.Equal(t, "bar", *v.Value)
	})
	t.Run("flag with value in single arg", func(t *testing.T) {
		c := &xli.Command{
			Flags: xli.Flags{
				&flg.String{Name: "foo"},
			},
		}

		_, err := c.Run(context.TODO(), []string{"--foo=bar"})
		require.NoError(t, err)

		v := c.Flags.Get("foo").(*flg.String)
		require.NotNil(t, v)
		require.Equal(t, "bar", *v.Value)
	})
	t.Run("flag with no value", func(t *testing.T) {
		c := &xli.Command{
			Flags: xli.Flags{
				&flg.String{Name: "foo"},
			},
		}

		_, err := c.Run(context.TODO(), []string{"--foo"})
		require.ErrorContains(t, err, "--foo: no value is given")
	})
	t.Run("flag with no value but flag", func(t *testing.T) {
		c := &xli.Command{
			Flags: xli.Flags{
				&flg.String{Name: "foo"},
			},
		}

		_, err := c.Run(context.TODO(), []string{"--foo", "--bar", "baz"})
		require.ErrorContains(t, err, "--foo: no value is given but was flag: --bar")
	})
	t.Run("invalid flag syntax", func(t *testing.T) {
		c := &xli.Command{
			Flags: xli.Flags{
				&flg.String{Name: "foo"},
				&flg.String{Name: "bar"},
			},
		}

		_, err := c.Run(context.TODO(), []string{
			"---foo=a",
			"--bar=b",
		})
		require.ErrorContains(t, err, "three dashes: ---foo=a")
	})
	t.Run("switches and flags", func(t *testing.T) {
		c := &xli.Command{
			Flags: xli.Flags{
				&flg.Switch{Name: "foo"},
				&flg.Switch{Name: "bar"},
				&flg.String{Name: "baz"},
				&flg.String{Name: "qux"},
			},
		}

		_, err := c.Run(context.TODO(), []string{
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
	t.Run("arg", func(t *testing.T) {
		c := &xli.Command{
			Args: xli.Args{
				&arg.String{Name: "FOO"},
			},
		}

		_, err := c.Run(context.TODO(), []string{"foo"})
		require.NoError(t, err)

		v := c.Args.Get("FOO").(*arg.String)
		require.NotNil(t, v)
		require.Equal(t, "foo", *v.Value)
	})
	t.Run("args", func(t *testing.T) {
		c := &xli.Command{
			Args: xli.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
				&arg.String{Name: "BAZ"},
			},
		}

		_, err := c.Run(context.TODO(), []string{"foo", "bar", "baz"})
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
			Args: xli.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
			},
		}

		_, err := c.Run(context.TODO(), []string{"foo", "bar", "baz", "qux"})
		require.ErrorContains(t, err, `too many arguments: "baz"`)
	})
	t.Run("switches, flags, and args", func(t *testing.T) {
		c := &xli.Command{
			Flags: xli.Flags{
				&flg.Switch{Name: "foo"},
				&flg.String{Name: "bar"},
			},
			Args: xli.Args{
				&arg.String{Name: "BAZ"},
				&arg.String{Name: "QUX"},
			},
		}

		_, err := c.Run(context.TODO(), []string{
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
			Flags: xli.Flags{
				&flg.String{Name: "foo"},
				&flg.String{Name: "bar"},
			},
			Args: xli.Args{
				&arg.String{Name: "BAZ"},
				&arg.String{Name: "QUX"},
			},
		}

		_, err := c.Run(context.TODO(), []string{
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
			Flags: xli.Flags{
				&flg.String{Name: "foo"},
				&flg.String{Name: "bar"},
			},
			Args: xli.Args{
				&arg.String{Name: "BAZ"},
				&arg.String{Name: "QUX"},
			},
		}

		_, err := c.Run(context.TODO(), []string{
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
			Flags: xli.Flags{
				&flg.Switch{Name: "switch"},
				&flg.String{Name: "flag"},
			},
			Args: xli.Args{
				&arg.String{Name: "FOO"},
				&arg.String{Name: "BAR"},
			},
			Commands: []*xli.Command{
				{
					Name: "foo",
					Flags: xli.Flags{
						&flg.Switch{Name: "switch_1"},
						&flg.String{Name: "flag_1"},
					},
					Args: xli.Args{
						&arg.String{Name: "FOO_1"},
						&arg.String{Name: "BAR_1"},
					},
					Commands: []*xli.Command{
						{
							Name: "bar",
							Flags: xli.Flags{
								&flg.Switch{Name: "switch_a"},
								&flg.String{Name: "flag_a"},
							},
							Args: xli.Args{
								&arg.String{Name: "FOO_a"},
								&arg.String{Name: "BAR_a"},
							},
						},
					},
				},
			},
		}

		_, err := c.Run(context.TODO(), []string{
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
}

func TestCommandMode(t *testing.T) {
	appender := func(vs *[]string, ms *[]mode.Mode, l string) xli.Action {
		return func(ctx context.Context, cmd *xli.Command) (context.Context, error) {
			*vs = append(*vs, l)
			*ms = append(*ms, mode.From(ctx))
			return ctx, nil
		}
	}

	t.Run("run", func(t *testing.T) {
		vs := []string{}
		ms := []mode.Mode{}
		c := &xli.Command{
			Name: "root",
			Commands: xli.Commands{
				&xli.Command{
					Name: "foo",
					Args: xli.Args{
						&arg.String{
							Name: "ARG",
							Action: func(ctx context.Context, cmd *xli.Command, v string) (context.Context, error) {
								vs = append(vs, "foo_arg")
								ms = append(ms, mode.From(ctx))
								return ctx, nil
							},
						},
					},
					Commands: xli.Commands{
						&xli.Command{
							Name: "bar",
							Args: xli.Args{
								&arg.String{
									Name: "ARG",
									Action: func(ctx context.Context, cmd *xli.Command, v string) (context.Context, error) {
										vs = append(vs, "bar_arg")
										ms = append(ms, mode.From(ctx))
										return ctx, nil
									},
								},
							},
							PreAction:  appender(&vs, &ms, "bar-pre"),
							Action:     appender(&vs, &ms, "bar"),
							PostAction: appender(&vs, &ms, "bar-post"),
						},
					},
					PreAction:  appender(&vs, &ms, "foo-pre"),
					Action:     appender(&vs, &ms, "foo"),
					PostAction: appender(&vs, &ms, "foo-post"),
				},
			},
			PreAction:  appender(&vs, &ms, "root-pre"),
			Action:     appender(&vs, &ms, "root"),
			PostAction: appender(&vs, &ms, "root-post"),
		}

		_, err := c.Run(context.TODO(), []string{"foo", "something", "bar", "else"})
		require.NoError(t, err)
		require.Equal(t, []string{
			"root-pre", "root",
			/* foo */ "foo-pre", "foo_arg", "foo",
			/* | bar */ "bar-pre", "bar_arg", "bar",
			/* | bar */ "bar-post",
			/* foo */ "foo-post",
			"root-post",
		}, vs)
		require.Equal(t, []mode.Mode{
			mode.Run | mode.Pass, // root-pre
			mode.Run | mode.Pass, // root
			mode.Run | mode.Pass, // foo-pre
			mode.Run | mode.Pass, // foo_arg
			mode.Run | mode.Pass, // foo
			mode.Run,             // bar-pre
			mode.Run,             // bar_arg
			mode.Run,             // bar
			mode.Run,             // bar-post
			mode.Run | mode.Pass, // foo-post
			mode.Run | mode.Pass, // root-post
		}, ms)
	})
}
