# Arguments

Arguments are the positional inputs of a command. They are declared on
`Command.Args` and must appear **after** any flags for that command.

```go
&xli.Command{
	Args: arg.Args{
		&arg.String{Name: "SRC"},
		&arg.String{Name: "DST"},
	},
}
```

By convention argument names are upper-case; the usage line renders them as
`<SRC>` (required), `[DST]` (optional), and `[NAME...]` (variadic).

## Built-in types

| Type | Go type |
| --- | --- |
| `arg.String` | `string` |
| `arg.Int` `arg.Int32` `arg.Int64` | signed ints |
| `arg.Uint` `arg.Uint32` `arg.Uint64` | unsigned ints |
| `arg.Float32` `arg.Float64` | floats |
| `arg.Duration` | `time.Duration` |

Scalar types are aliases of `arg.Base[T, P]`.

Variadic (collect the rest) types use `arg.Rest[T, P]`:

| Type | Collects |
| --- | --- |
| `arg.RestStrings` | `[]string` |
| `arg.RestInts` / `arg.RestInt32s` / `arg.RestInt64s` | `[]int…` |
| `arg.RestUints` / `arg.RestUint32s` / `arg.RestUint64s` | `[]uint…` |

`arg.Remains` (= `arg.Base[[]string, …]`) collects everything after a literal
`--` separator.

## Required, optional, variadic

```go
Args: arg.Args{
	&arg.String{Name: "SRC"},                  // required
	&arg.String{Name: "DST", Optional: true},  // optional
	&arg.RestStrings{Name: "MORE"},            // variadic (implies optional)
}
```

- A required argument that is missing makes `Run` return `ErrNeedArgs`.
- Optional arguments may be omitted.
- A `Rest`/`Remains` argument is always optional and must be last.

> A command cannot have both an optional argument and subcommands — the parser
> could not tell an argument from a subcommand name. This is enforced.

## Reading values: the default/parsed contract

Arguments follow the same contract as flags:

- `Default *T` — the fallback (meaningful for an optional argument); never written
  by the framework.
- `Value *T` — the parsed value; `nil` until provided.

```go
// (value, provided?)
v, ok := arg.Get[string](cmd, "DST")

// effective value: parsed → default → panic
dst := arg.MustGet[string](cmd, "DST")

// write only if provided
var out string
arg.VisitP(cmd, "DST", &out)
```

| Accessor | Behavior |
| --- | --- |
| `arg.Get[T]` | `(value, true)` if provided; otherwise `(zero, false)` |
| `arg.MustGet[T]` | provided value → default → panic |
| `arg.Visit[T]` / `arg.VisitP[T]` | invoke / write only if provided |

A variadic argument's `Get` returns `ok=true` once at least one value is parsed:

```go
vs, ok := arg.Get[[]string](cmd, "MORE")
```

### Defaults

```go
def := "out.txt"
&arg.String{Name: "DST", Optional: true, Default: &def}
```

`MustGet` returns the default when the argument is omitted, and the default is
shown in `--help`.

## Handlers

Attach a handler that runs when the argument is parsed (mode-aware, like flags):

```go
&arg.String{
	Name: "PATH",
	Handler: arg.OnRun(func(ctx context.Context, v string) error {
		return validatePath(v)
	}),
}
```

### Completion for argument values

Provide candidates with `arg.OnTab`:

```go
&arg.RestStrings{
	Name: "FILE",
	Handler: arg.OnTab[[]string](func(ctx context.Context, t tab.Tab) {
		t.Value("a.txt")
		t.Value("b.txt")
	}),
}
```

Use `t.Group("name")` to group candidates.

## Custom argument types

Implement an `arg.Parser[T]` (it may consume more than one token) and use
`arg.Base[T, P]`:

```go
type Parser[T any] interface {
	Parse(rest []string) (value T, consumed int, err error)
	String() string
}
```

To reuse a `flg.Parser` (single-token) as an argument parser, wrap it with
`arg.Mono[T, P]`.
