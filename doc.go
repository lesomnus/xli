// Package xli builds command-line interfaces as a tree of commands whose
// handlers are middleware.
//
// A [Command] declares its name, its [flg.Flags], its [arg.Args] and its
// subcommands. [Command.Run] parses a command line against the tree, stores
// what it parsed in the flags and arguments, and calls the handlers of the
// commands on the path, from the root down:
//
//	root := &xli.Command{
//		Name:  "app",
//		Flags: flg.Flags{&flg.Switch{Name: "verbose", Alias: 'v'}},
//		Commands: xli.Commands{
//			{
//				Name: "greet",
//				Args: arg.Args{&arg.String{Name: "NAME"}},
//				Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
//					cmd.Printf("hello, %s\n", arg.MustGet[string](cmd, "NAME"))
//					return next(ctx)
//				}),
//			},
//		},
//		Handler: xli.RequireSubcommand(),
//	}
//	err := root.Run(ctx, os.Args[1:])
//
// # What is intended, and easy to trip on
//
// The following are by design, and each is a surprise if it is not known.
//
// A handler is middleware. It is given next, and the subcommand runs only if
// the handler calls next(ctx): Run does not call it on the handler's behalf.
// A parent sets up what its subcommands share -- a configuration, a client, a
// span -- before next, and acts on the result after it; one that returns
// without calling next ends the run there. A command with no handler calls
// next, and [Chain] puts several handlers on one command, each of which calls
// the next in turn.
//
// The handler of every command on the path runs, in a mode that says why:
// [mode.Run] for the command named last, [mode.Run] with [mode.Pass] for the
// commands on the way to it, and [mode.Help] and [mode.Tab] for --help and
// shell completion, which walk the same tree. [OnRun], [OnRunPass], [OnHelp]
// and [OnTab] call a function in their mode and next in any other.
//
// Positions are strict. A command's flags and arguments are its own: they are
// not given to its parent or to its subcommand, and its flags come before its
// arguments. In `app --verbose deploy --force web`, --verbose is app's and
// --force and web are deploy's; `app deploy web --force` is [ErrFlagAfterArg].
// It is what keeps a deep tree unambiguous.
//
// A tree is run once. Run writes into the tree it is given: the values it
// parses into the flags and arguments, each command's parent, and the IO a
// subcommand inherits. Build the tree once per process, as a command line is
// run once per process; a test that runs two command lines builds two trees.
//
// A flag's Default and Value are not the same thing. Default is the value you
// configured, and the framework never writes it; Value is what the user gave,
// and only the framework writes it. [flg.Get] and [arg.Get] report whether the
// user gave a value; [flg.MustGet] and [arg.MustGet] return it, or else the
// Default, and panic if there is neither.
//
// # More
//
// The guides under docs/ in the repository cover commands, flags, arguments,
// completion and testing. Package [github.com/lesomnus/xli/xlitest] runs a
// command line in a test, and the optional module
// [github.com/lesomnus/xli/cfg] reads a configuration struct from a file, the
// environment and these flags.
package xli
