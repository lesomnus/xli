package xli_test

import (
	"strings"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/internal/x"
)

func TestPrintHelp(t *testing.T) {
	t.Run("usage separates arguments with spaces", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "cp",
			Args: arg.Args{
				&arg.String{Name: "SRC"},
				&arg.String{Name: "DST"},
			},
		}

		b := &strings.Builder{}
		err := c.PrintHelp(b)
		x.NoError(err)
		x.Contains(b.String(), "cp <SRC> <DST>")
	}))
	t.Run("optional argument is rendered with brackets", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "rm",
			Args: arg.Args{
				&arg.String{Name: "TARGET", Optional: true},
			},
		}

		b := &strings.Builder{}
		err := c.PrintHelp(b)
		x.NoError(err)
		x.Contains(b.String(), "rm [TARGET]")
	}))
	t.Run("variadic argument is rendered with ellipsis", x.F(func(x x.X) {
		c := &xli.Command{
			Name: "echo",
			Args: arg.Args{
				&arg.RestStrings{Name: "STRING"},
			},
		}

		b := &strings.Builder{}
		err := c.PrintHelp(b)
		x.NoError(err)
		x.Contains(b.String(), "echo [STRING...]")
	}))
}
