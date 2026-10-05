# Commands & subcommands

A CLI is a tree of `*xli.Command`. Each command is a middleware: it receives the
execution and decides whether (and when) to descend into its subcommand. Context
is nested as you go down the tree.

## The `Command` struct

```go
type Command struct {
	Category string   // grouping for the parent's help/completion
	Name     string
	Aliases  []string
	Brief    string   // one-line summary (shown next to the name)
	Synop    string   // longer description (shown as "Description:")

	Flags    flg.Flags
	Args     arg.Args
	Commands Commands

	Exclusive [][]string // groups of flags of which at most one may be given

	Handler Handler

	Hidden bool // omit from the parent's help and completion

	io.ReadCloser // input;  defaults to os.Stdin
	io.Writer     // output; defaults to os.Stdout
	ErrWriter io.Writer
}
```

Run a command with `Run(ctx, args)`, where `args` is usually `os.Args[1:]`:

```go
root := &xli.Command{Name: "app", /* ... */}
if err := root.Run(context.Background(), os.Args[1:]); err != nil {
	fmt.Fprintln(os.Stderr, err)

	var ue *xli.UsageError
	if errors.As(err, &ue) {
		fmt.Fprintf(os.Stderr, "Run '%s --help' for usage.\n", ue.Path())
		os.Exit(2)
	}
	os.Exit(1)
}
```

### Errors

If the command line doesn't match what the commands declare (an unknown flag or
subcommand, a bad value, a missing required flag or argument), `Run` returns an
`*xli.UsageError`. It names the command the mistake was made on, so the message
looks like `app deploy: --prot: unknown flag (did you mean "--port"?)`. Unknown
subcommands and long flags get a "did you mean" suggestion when a close name
exists. `ue.Cmd` is the command in question, so
`ue.Cmd.PrintHelp(w)` prints its help. The sentinel errors (`ErrUnknownFlag`,
`ErrUnknownCmd`, `ErrNeedArgs`, `ErrNeedCmd`, `ErrFlagRequired`, ...) still work
with `errors.Is`. Errors returned by your handlers are passed through unchanged.

### Keep constructors cheap

The whole tree is built eagerly, even though a single run only visits one path
through it. Building the tree itself costs almost nothing (about 0.1 ms and
250 KB for ~200 commands with several flags each, which is small next to Go
runtime startup). What does cost something is real work inside constructors.
Keep `newXxx()` functions to declaring structs, and do I/O, config loading, and
client setup inside the `Handler`, where it runs only for the command that is
actually executed.

## Handlers are middleware

A `Handler` receives the command and a `next` function:

```go
type HandlerFunc func(ctx context.Context, cmd *xli.Command, next xli.Next) error
```

To run the matched subcommand you must call `next(ctx)`. This is what makes a
parent command behave like middleware — it can set up state, run the child, and
act on the result:

```go
&xli.Command{
	Name: "app",
	Handler: xli.OnRunPass(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
		ctx = withConfig(ctx)        // before the subcommand
		if err := next(ctx); err != nil { // run the subcommand
			return err
		}
		return cleanup()             // after the subcommand
	}),
	Commands: xli.Commands{ subCmd() },
}
```

`Run` does **not** call the subcommand for you — a handler that never calls
`next` stops the chain there. If a command has no handler, a no-op that simply
calls `next` is used.

Compose multiple handlers with `xli.Chain(h1, h2, ...)`; each is invoked in order
and is itself responsible for calling `next`.

> If a command both wires IO defaults and relies on `next`, remember that IO
> (`Writer`/`ReadCloser`) is inherited by the subcommand only when the parent's
> handler calls `next`.

## Execution modes

The same tree is walked for different purposes; handlers are gated on the mode so
they only run when relevant. The mode lives in the context (`mode` package).

| Helper | Runs when |
| --- | --- |
| `xli.OnRun` | executing this command as the target |
| `xli.OnRunPass` | passing through, on the way to a subcommand |
| `xli.OnHelp` | rendering `--help` |
| `xli.OnHelpPass` | passing through while rendering help |
| `xli.OnTab` | producing shell completions |
| `xli.OnTabPass` | passing through during completion |

Lower-level building blocks:

- `xli.On(m, fn)` — runs when all bits of mode `m` are set.
- `xli.OnExact(m, fn)` — runs only when the mode equals `m` exactly.
- `xli.OnF(pred, fn)` — runs when `pred(mode)` is true.

The mode is a bit set: `Run`/`Help`/`Tab` (the kind) optionally combined with
`Pass` (more commands will run after this one). So `OnRun` is "this is the leaf
being run", while `OnRunPass` is "running, but a subcommand follows".

## Strict flag/argument positioning

`xli` is deliberately strict to keep parsing unambiguous in deep trees:

- A command owns **only its own** flags and arguments; they may not appear on a
  parent or a child.
- **Flags must come before arguments** for a given command.
- A token that is not a flag and is not consumed as an argument is treated as a
  subcommand name.

```
app --verbose deploy --force web
    \_______/        \_____/ \_/
     app's flag    deploy's   deploy's
                    flag       arg
```

## Subcommand lookup & categories

`Commands` is a slice with helpers:

- `Get(name)` — find by name or alias.
- `WithCategory(name, cmds...)` — tag commands with a category.
- `ByCategory()` — group for help/completion.

```go
Commands: xli.Commands{
	newServe(),
}.WithCategory("debug",
	newDump(),
	newTrace(),
),
```

Categories become headings in `--help` and groups in shell completion.

A command with `Hidden: true` can still be run by name, but it is left out of its
parent's help, shell completion, and "did you mean" suggestions. This is useful
for internal or deprecated commands. `Commands.Visible()` / `Flags.Visible()`
return only the non-hidden entries.

Require a subcommand to be chosen with `xli.RequireSubcommand()`:

```go
Handler: xli.RequireSubcommand(),
```

## Tree navigation

Inside a handler the command is wired to its parent:

- `cmd.Parent()` / `cmd.HasParent()`
- `cmd.Root()` — the top-most command
- `cmd.Tree()` — root→leaf slice

The current frame chain is also available via `frm.From(ctx)` (and
`frm.HasSeq(f, "a", "b")` to test the command path).

> The parent links are established while the command runs, so `Parent()`/`Root()`
> are meaningful inside handlers. A command tree is meant to be run once
> (typically a process singleton); it is not safe to run the same tree
> concurrently or to reuse it across runs.

## IO

A command exposes IO helpers that default to the process streams:

```go
cmd.Print(...)   cmd.Printf(...)   cmd.Println(...)
cmd.Scan(...)    cmd.Scanf(...)    cmd.Scanln(...)
```

## Help

`--help` / `-h` print a generated message: name, usage line, argument and flag
details (with defaults and `(required)` markers), and subcommands grouped by
category. `Synop` is shown as a `Description:` section. The template is built in
and not currently overridable.

## Version

There is no built-in `--version`. Add your own — typically a `version`
subcommand:

```go
func newVersion() *xli.Command {
	return &xli.Command{
		Name:  "version",
		Brief: "print the version",
		Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			cmd.Println(buildVersion)
			return next(ctx)
		}),
	}
}
```

## Shell completion

Mount the completion command and source its script:

```go
Commands: xli.Commands{ xli.NewCmdCompletion() },
```

```sh
source <(app completion zsh)
```

See [completion.md](completion.md) for the completion model and the `tab.Tab`
sink, and [flags.md](flags.md) / [arguments.md](arguments.md) for providing
candidates for flag/argument values.
