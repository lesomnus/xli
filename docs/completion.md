# Shell completion

`xli` generates shell completion and lets commands contribute their own
candidates. **zsh**, **bash**, and **fish** are supported.

## How it works

When the shell asks for completions it re-invokes your program with a special
marker as the last arguments. `Run` detects this, switches into `Tab` mode, walks
the command tree, and lets handlers emit candidates into a `tab.Tab` sink. The
sink writes lines that the generated shell script turns into completions.

You normally don't see any of this — you just:

1. mount the completion command, and
2. provide candidates for the values you care about.

## Setup

Mount the completion command:

```go
Commands: xli.Commands{
	xli.NewCmdCompletion(),
},
```

Then load the generated script in your shell's startup file:

```sh
source <(app completion zsh)    # zsh  (~/.zshrc)
source <(app completion bash)   # bash (~/.bashrc)
app completion fish | source    # fish (~/.config/fish/config.fish)
```

The scripts are keyed on the root command's name, so they apply to the program
as invoked by that name.

## What is completed automatically

- **Subcommands** — at a command position, grouped by their `Category`.
- **Flag names** — after `--`, grouped by their `Category`.
- **Flag values / argument values** — only if you provide them (see below).

## Providing candidates

Use the mode-aware `OnTab` handler on a flag or argument. It receives a
`tab.Tab`:

```go
&flg.String{
	Name: "format",
	Handler: flg.OnTab[string](func(ctx context.Context, t tab.Tab) error {
		t.ValueD("json", "JSON output")
		t.ValueD("yaml", "YAML output")
		return nil
	}),
}

&arg.String{
	Name: "PATH",
	Handler: arg.OnTab[string](func(ctx context.Context, t tab.Tab) error {
		t.Files("") // complete any file
		return nil
	}),
}
```

> Both run only in `Tab` mode, and both return `error`.

## The `tab.Tab` sink

```go
type Tab interface {
	Value(v string)            // a candidate
	ValueD(v string, desc string) // a candidate with a description
	Group(name string) Tab     // candidates under a heading
	Files(pattern string)      // file completion ("" = any, or a glob like "*.go")
	Dirs()                     // directory-only completion
}
```

- **Descriptions** (`ValueD`) are shown next to the candidate.
- **Groups** put candidates under a heading; `Group` returns a sink scoped to
  that heading:

  ```go
  net := t.Group("network")
  net.Value("eth0")
  net.Value("wlan0")
  ```

- **Files / Dirs** delegate to the shell's own path completion, so globbing, `~`
  expansion, and the like work as users expect:

  ```go
  t.Files("*.go") // only .go files
  t.Dirs()        // directories only
  ```

Candidate emission does not return errors (a broken completion pipe should not
fail your command); an `OnTab` handler returns `error` only so it can surface
failures from work it does to compute candidates.

## Shell differences

All three scripts decode the same candidate stream, but not every shell can show
everything:

| | zsh | fish | bash |
| --- | --- | --- | --- |
| Descriptions (`ValueD`) | ✅ | ✅ | — |
| Groups (`Group`) | ✅ headings | — (flattened) | — (flattened) |
| Files / Dirs / globs | ✅ | ✅ | ✅ |

The bash script splits the command line itself rather than relying on
`COMP_WORDS`, so `--flag=value` completes correctly whether or not `=` is in
`COMP_WORDBREAKS`. It does not need the bash-completion package.

## Other shells

`tab.Tab` is an interface and the shell is selected by the completion request, so
support for additional shells can be added by implementing a new `Tab` and
emitting the corresponding script.
