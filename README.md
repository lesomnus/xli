# xli

> Build your CLI like a middleware.

`xli` is a small, opinionated CLI framework for Go. Commands form a tree, each
command is a middleware that decides whether to descend into its subcommand,
and context is nested as you go down the tree.

> **Status:** pre-1.0, API not yet frozen. See [ROADMAP.md](ROADMAP.md) for the
> path to a stable release.

## Features

- **Middleware handlers** — every command receives a `next` function and chooses
  if/when to invoke its subcommand, so cross-cutting setup (auth, config, tracing)
  lives in parent commands.
- **Nested context** — each subcommand nests `context.Context`; values flow down
  the tree.
- **Strict flag/arg positioning** — a command owns only its own flags and args;
  flags must come before args. This keeps parsing unambiguous across deep trees.
- **Typed flags and arguments** — `string`, `bool` (switch), `int`/`uint`
  families, `float32`/`float64`, and `time.Duration`, plus variadic arguments.
- **Defaults & required flags** — a clear contract between the configured default
  and the value the user actually provided.
- **Generated help** and **shell completion** (zsh).

## Install

```sh
go get github.com/lesomnus/xli
```

Requires Go 1.24+.

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
)

func main() {
	greeting := "Hello"
	cmd := &xli.Command{
		Name:  "greet",
		Brief: "greet someone",
		Flags: flg.Flags{
			&flg.String{Name: "greeting", Alias: 'g', Brief: "the greeting to use", Default: &greeting},
		},
		Args: arg.Args{
			&arg.String{Name: "NAME", Brief: "who to greet"},
		},
		Handler: xli.OnRun(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
			g := flg.MustGet[string](cmd, "greeting")
			name := arg.MustGet[string](cmd, "NAME")
			cmd.Printf("%s, %s!\n", g, name)
			return next(ctx)
		}),
	}

	if err := cmd.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

```console
$ greet World
Hello, World!
$ greet --greeting=Hi World
Hi, World!
$ greet --help
```

## Documentation

- [docs/commands.md](docs/commands.md) — the command tree, middleware handlers,
  execution modes, strict positioning, and completion.
- [docs/flags.md](docs/flags.md) — flag types, the default/parsed contract,
  required flags, categories, and value completion.
- [docs/arguments.md](docs/arguments.md) — positional/variadic arguments, the
  default/parsed contract, and value completion.

## Concepts

### Handlers are middleware

A `Handler` receives the command and a `next` function. To run a subcommand you
must call `next(ctx)`; this is what makes parent commands behave like middleware.

```go
root := &xli.Command{
	Name: "app",
	// Runs while passing through `app` on the way to a subcommand.
	Handler: xli.OnRunPass(func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
		ctx = withConfig(ctx) // set up shared state...
		return next(ctx)      // ...then descend into the subcommand.
	}),
	Commands: xli.Commands{
		subCmd(),
	},
}
```

Compose handlers with `xli.Chain(...)`.

### Execution modes

The same command tree is walked for different purposes. Handlers are gated on the
mode so you only do work when it is relevant:

| Helper | Runs when |
| --- | --- |
| `OnRun` | executing this command as the target |
| `OnRunPass` | passing through on the way to a subcommand |
| `OnHelp` | rendering `--help` |
| `OnTab` | producing shell completions |

There are matching helpers in the `flg` and `arg` packages for flag/argument
handlers (e.g. `flg.OnTab` to provide completion candidates for a flag value,
optionally grouped with `tab.Group`).

### Flags and arguments

Declare typed flags and positional arguments; read them back with `Get`/`MustGet`
(or `VisitP`, `Find` for the parent chain).

```go
Flags: flg.Flags{
	&flg.Switch{Name: "verbose", Alias: 'v'},
	&flg.Int{Name: "retries", Default: &three},
	&flg.String{Name: "token", Required: true},
},
Args: arg.Args{
	&arg.String{Name: "SRC"},
	&arg.RestStrings{Name: "DST"}, // variadic
},
```

**Default vs. provided value.** `Default` is the configured fallback and is never
modified by the framework; `Value` holds what the user actually passed.

- `Get` / `VisitP` / `Find` report whether the user *provided* the flag.
- `MustGet` / `MustFind` return the provided value, else the default, else panic.

```go
if v, ok := flg.Get[string](cmd, "token"); ok {
	// the user explicitly passed --token
}
token := flg.MustGet[string](cmd, "token") // provided value or default
```

A `Required` flag that is absent makes `Run` return `ErrFlagRequired` (help and
completion are exempt).

### Help and completion

`--help` / `-h` render a generated help message (usage line, arguments, flags
with defaults, and subcommands grouped by category).

Add shell completion by mounting the completion command and sourcing its script:

```go
Commands: xli.Commands{
	xli.NewCmdCompletion(),
},
```

```sh
source <(app completion zsh)
```

## License

See the repository for license details.
