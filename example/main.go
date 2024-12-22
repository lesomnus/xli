package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flag"
)

func main() {
	c := &xli.Command{
		Name:  "root",
		Brief: "root-brief",
		Synop: "Long description will be printed here",
		Flags: xli.Flags{
			&flag.String{Name: "foo", Brief: "foo-brief"},
			&flag.String{Name: "bar", Brief: "bar-brief", Alias: 'b'},
			&flag.Uint32{Name: "size", Brief: "size-brief", Alias: 's'},
		},
		Args: xli.Args{
			&arg.String{Name: "FOO", Brief: "FOO-brief"},
			&arg.String{Name: "BAR", Brief: "BAR-brief"},
		},
		Commands: xli.Commands{
			&xli.Command{
				Name:  "foo",
				Brief: "cmd-foo-brief",
				Flags: xli.Flags{
					&flag.String{Name: "baz"},
				},
				Args: xli.Args{
					&arg.String{Name: "BAZ"},
				},
			},
		}.WithCategory("fruits",
			&xli.Command{
				Name:    "apple",
				Aliases: []string{"app", "ap"},
				Brief:   "looks red",
			},
			&xli.Command{
				Name:    "banana",
				Aliases: []string{"bnn"},
				Brief:   "looks yellow",
			},
			&xli.Command{
				Name:  "kiwi",
				Brief: "looks green",
			},
		),
	}

	_, err := c.Run(context.TODO(), os.Args[1:])
	fmt.Printf("err: %v\n", err)
}
