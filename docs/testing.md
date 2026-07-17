# Testing commands

A `xli.Command` is a plain struct with a `Run` method, so you test it by running
it and asserting on what it did — what it wrote, what it returned, and which
completion candidates it offered. The `xli/xlitest` package wires the in-memory
IO for you and decodes completion output so you don't have to.

```go
import "github.com/lesomnus/xli/xlitest"
```

Everything here uses the standard `testing` package; `xlitest` takes a
`testing.TB`, so it works from tests, benchmarks, and fuzz targets.

## Running a command

`xlitest.Run` sets up in-memory stdin/stdout/stderr, runs the command, and
returns a `Result`:

```go
func TestGreet(t *testing.T) {
	got := xlitest.Run(t, newGreetCmd(), "--greeting=Hi", "World")

	if got.Err != nil {
		t.Fatalf("unexpected error: %v", got.Err)
	}
	if got.Stdout != "Hi, World!\n" {
		t.Errorf("stdout = %q", got.Stdout)
	}
}
```

Here `newGreetCmd()` builds the `greet` command from the
[README quick start](../README.md#quick-start), whose handler prints the
greeting followed by a newline — the framework adds none, so `Stdout` is exactly
what the handler wrote.

`Result` has three fields:

| Field | What it holds |
| --- | --- |
| `Stdout` | everything written to the command's `Writer` |
| `Stderr` | everything written to its `ErrWriter` |
| `Err` | the error returned by `Run` |

The command never touches the real `os.Stdin`/`os.Stdout`/`os.Stderr`, so tests
neither block on the terminal nor leak output.

> **Build a fresh command per run.** A command keeps the values parsed into its
> flags and arguments, so reusing one `*xli.Command` across runs lets earlier
> state leak into later ones. Wrap construction in a factory and call it each
> time:
>
> ```go
> newCmd := func() *xli.Command { return &xli.Command{ /* ... */ } }
> a := xlitest.Run(t, newCmd(), "one")
> b := xlitest.Run(t, newCmd(), "two")
> ```

## Asserting errors

`Run` returns the error rather than exiting, so assert on it directly. The
sentinel errors in the `xli` package pair well with `errors.Is`:

```go
got := xlitest.Run(t, newCmd()) // required --token omitted
if !errors.Is(got.Err, xli.ErrFlagRequired) {
	t.Fatalf("want ErrFlagRequired, got %v", got.Err)
}
```

Useful sentinels: `ErrFlagRequired`, `ErrUnknownFlag`, `ErrNoFlagValue`,
`ErrFlagAfterArg`, `ErrUnknownCmd`, `ErrTooManyArgs`, `ErrNeedArgs`.

## Asserting parsed values

To check what the framework parsed (rather than what the handler printed), read
the flag and argument values back after the run with `flg.Get`/`flg.MustGet` and
`arg.Get`/`arg.MustGet`:

```go
c := newCmd()
xlitest.Run(t, c, "--retries=3", "src", "dst")

if v, ok := flg.Get[int](c, "retries"); !ok || v != 3 {
	t.Errorf("retries = %d, provided = %v", v, ok)
}
if arg.MustGet[string](c, "SRC") != "src" {
	t.Error("SRC not parsed")
}
```

`Get` reports whether the user *provided* the value; `MustGet` returns the
provided value, else the default, else panics. (See
[flags.md](flags.md) and [arguments.md](arguments.md).)

## Feeding stdin

Set `Stdin` on a `Harness` to give the command input:

```go
got := xlitest.Harness{Cmd: newCmd(), Stdin: "yes\n"}.Run(t)
```

Inside the handler, read it with the command's scan helpers
(`cmd.Scanln`, `cmd.Scan`, `cmd.Scanf`). Left empty, stdin reads as immediate
EOF.

## Choosing the context

`Run` uses `context.Background()` by default. To pass your own context — for a
deadline, or values a parent handler would normally inject — use a `Harness`:

```go
ctx := context.WithValue(context.Background(), keyConfig, cfg)
got := xlitest.Harness{Cmd: newCmd(), Ctx: ctx}.Run(t)
```

## Testing handlers and execution order

Handlers are middleware and run in a mode-dependent order (see
[commands.md](commands.md)). A common test records the order handlers fire by
appending to a slice:

```go
var order []string
record := func(name string) xli.HandlerFunc {
	return func(ctx context.Context, cmd *xli.Command, next xli.Next) error {
		order = append(order, name)
		return next(ctx)
	}
}

c := &xli.Command{
	Name:    "app",
	Handler: xli.OnRunPass(record("root")),
	Commands: xli.Commands{
		&xli.Command{Name: "sub", Handler: xli.OnRun(record("sub"))},
	},
}

xlitest.Run(t, c, "sub")
// order == ["root", "sub"]
```

This is also how you assert that a parent's setup (auth, config) runs before the
subcommand, and that it is skipped for `--help`.

## Testing help output

`--help` renders to the command's `Writer`, so it lands in `Result.Stdout`:

```go
got := xlitest.Run(t, newCmd(), "--help")
if !strings.Contains(got.Stdout, "Usage:") {
	t.Error("help not rendered")
}
```

## Testing completion

Completion is the awkward part to test by hand: the shell re-invokes your
program with an internal marker and a cursor protocol, and the reply is a
line-based wire format. `xlitest.Complete` hides both. You give it the command
line as the user would type it (the tokens after the program name, cursor at the
end) and it returns decoded `Completions`:

```go
got := xlitest.Complete(t, newCmd(), "remote ")
// got.Values() == []string{"add", "remove"}

if !got.Has("add") {
	t.Error("expected 'add' to be offered")
}
```

A **trailing space** matters — it means the cursor is at a fresh word:

| Line | Completes |
| --- | --- |
| `Complete(t, c, "remote ")` | `remote`'s subcommands (`add`, `remove`) |
| `Complete(t, c, "remote a")` | still the subcommand set (the shell filters by `a`) |
| `Complete(t, c, "--")` | flag names |
| `Complete(t, c, "--tag=")` | values for `--tag` |
| `Complete(t, c, "")` | root subcommands |

xli emits the full candidate set and lets the shell narrow it by the typed
prefix, so a partial word offers the same set as the bare position.

### The `Completions` result

```go
type Completions struct {
	Candidates   []Candidate // value suggestions, in order
	FilePatterns []string    // globs from Files(), e.g. "*.go"
	WantFiles    bool        // file completion requested
	WantDirs     bool        // directory completion requested
	Err          error       // completion error (rare)
	Raw          string      // undecoded wire output
}

type Candidate struct {
	Group string // heading; empty when ungrouped
	Value string // the inserted text
	Desc  string // description; empty when none
}
```

Convenience methods:

- `Values()` — just the `Value` of each candidate.
- `Has(value)` — whether a candidate with that value was offered.
- `Get(value)` — the first matching candidate (for checking `Desc`/`Group`).

```go
got := xlitest.Complete(t, newCmd(), "--format=")
cand, ok := got.Get("json")
// ok == true, cand.Desc == "JSON output"
```

Group headings and file/dir requests are decoded too:

```go
got := xlitest.Complete(t, newCmd(), "")
apple, _ := got.Get("apple")
// apple.Group == "fruits"

paths := xlitest.Complete(t, newCmd(), "")
// paths.WantFiles == true, paths.FilePatterns == []string{"*.go"}
```

`Complete` does not fail the test on a completion error; it records it in
`Err`. That is rare and usually means the line was malformed (for example an
unknown flag earlier in it), since completion is designed to emit nothing rather
than error.

> `Complete` splits the line on whitespace and does not interpret shell quoting
> or escapes. For a value containing spaces, drive the run through
> `Harness.Complete` with the words already separated, or test the flag's
> `OnTab` handler in isolation.

## Without xlitest

`xlitest` only wraps what you can do directly: set the command's `Writer`,
`ErrWriter`, and `ReadCloser` to your own buffers before calling `Run`. Reach
for the raw form when you need an IO type `xlitest` doesn't model (a failing
writer, a slow reader); otherwise `xlitest` is less to get wrong.
