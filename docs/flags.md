# Flags

Flags are the optional, named inputs of a command (`--name`, `-n`). They are
declared on `Command.Flags` and must appear **before** positional arguments.

```go
&xli.Command{
	Flags: flg.Flags{
		&flg.String{Name: "config", Alias: 'c', Brief: "path to the config file"},
		&flg.Switch{Name: "verbose", Alias: 'v'},
	},
}
```

## Built-in types

| Type | Go type | Notes |
| --- | --- | --- |
| `flg.String` | `string` | |
| `flg.Switch` | `bool` | value-less; `--flag` means `true`, `--flag=false` to unset |
| `flg.Int` `flg.Int32` `flg.Int64` | signed ints | |
| `flg.Uint` `flg.Uint32` `flg.Uint64` | unsigned ints | |
| `flg.Float32` `flg.Float64` | floats | |
| `flg.Duration` | `time.Duration` | accepts `1m30s`, `500ms`, … |

All of them are aliases of the generic `flg.Base[T, P]`.

Every flag carries metadata:

```go
&flg.Int{
	Name:     "retries",
	Alias:    'r',
	Category: "network",     // grouping in help/completion
	Brief:    "retry count", // one-line help
	Synop:    "...",         // longer help
	Required: true,          // see "Required flags"
	Default:  &three,        // see "Defaults"
}
```

## Reading values: the default/parsed contract

A flag distinguishes the **default** (what you configured) from the **value**
(what the user actually passed):

- `Default *T` — the fallback. The framework never writes to it.
- `Value *T` — the parsed value; `nil` until the user provides the flag.

Read flags with the package helpers (a `*xli.Command` is a valid holder):

```go
// Did the user provide it? Returns the parsed value and ok=provided.
v, ok := flg.Get[string](cmd, "config")

// Effective value: parsed value, else the default, else panic.
cfg := flg.MustGet[string](cmd, "config")

// Write into a destination only if provided.
verbose := false
flg.VisitP(cmd, "verbose", &verbose)
```

| Accessor | Behavior |
| --- | --- |
| `flg.Get[T]` | `(value, true)` if the user provided the flag; otherwise `(zero, false)` |
| `flg.MustGet[T]` | provided value → default → panic |
| `flg.Visit[T]` / `flg.VisitP[T]` | invoke the visitor / write the pointer only if provided |
| `flg.Find[T]` / `flg.MustFind[T]` | like Get/MustGet, but search the parent chain too |
| `flg.Lookup[T]` / `flg.LookupP[T]` | like Visit/VisitP, but search the parent chain |

This makes "did the user set it?" unambiguous — important for layering CLI flags
over config files or environment variables:

```go
port := loadFromConfig()                 // start from config
if v, ok := flg.Get[int](cmd, "port"); ok {
	port = v                             // override only when the user passed --port
}
```

### Defaults

```go
def := "config.yaml"
&flg.String{Name: "config", Default: &def}
```

- `flg.Get` still reports `ok=false` when the flag is omitted.
- `flg.MustGet` returns the default when the flag is omitted.
- The default is shown in `--help` as `(default: …)`.

> Setting `Value` at construction time is not how you configure a default — use
> `Default`. `Value` is owned by the framework and holds the parsed result.

### Required flags

```go
&flg.String{Name: "token", Required: true}
```

If a required flag is absent, `Run` returns `ErrFlagRequired`. `--help` and shell
completion are exempt, so they keep working.

## Switches and short flags

`flg.Switch` takes no value: `--verbose` sets it to `true`; `--verbose=false`
unsets it. A flag is treated as value-less when its parser reports `NoValue()`,
so custom no-value flags are possible (see below).

Short aliases use a single rune: `&flg.Switch{Name: "verbose", Alias: 'v'}`
enables `-v`.

## Categories

Group flags under a heading in help and completion:

```go
Flags: flg.Flags{
	&flg.String{Name: "log-level"},
}.WithCategory("network",
	&flg.String{Name: "host"},
	&flg.Int{Name: "port"},
),
```

## Handlers

Attach a handler to react when a flag is parsed. Handlers are mode-aware, mirroring
command handlers (`flg.OnRun`, `flg.OnHelp`, `flg.OnTab`, …) and compose with
`flg.Wrap`:

```go
&flg.String{
	Name: "level",
	Handler: flg.OnRun(func(ctx context.Context, v string) error {
		return setLogLevel(v)
	}),
}
```

### Completion for flag values

Provide completion candidates for a flag's value with `flg.OnTab`:

```go
&flg.String{
	Name: "format",
	Handler: flg.OnTab[string](func(ctx context.Context, t tab.Tab) error {
		t.Value("json")
		t.Value("yaml")
		return nil
	}),
}
```

Candidates may be grouped with `t.Group("name")`. Completion for both long
(`--format=`) and short (`-f=`) forms is supported.

## Custom flag types

Implement a `flg.Parser[T]` and use `flg.Base[T, P]`:

```go
type Parser[T any] interface {
	Parse(s string) (T, error)
	ToString(v T) string // used for the help default
	String() string      // the type name shown in help
}
```

To make a custom value-less flag (a switch), also implement `NoValue() bool`
returning `true` on the parser.
