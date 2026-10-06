# cfg design

`cfg` is the optional configuration layer for xli applications. It loads an
application's own Go struct from a file, the environment and command-line flags,
records where every value came from, and keeps file-backed values current while
the process runs.

It is a separate module (`github.com/lesomnus/xli/cfg`) so that xli itself stays
free of a YAML dependency.

This document records the decisions and the reasons for them. The decisions were
made by surveying viper, koanf, kong, urfave/cli v3, ff v4, kelseyhightower
envconfig, caarlos0/env, cleanenv and sethvargo/go-envconfig, and by reading how
the downstream applications (payday and, through it, roster, shale and cr, plus
gantry and bosun) configure themselves today.

## The struct is the application's

The configuration is a plain struct the application declares. `cfg` walks it; it
never owns it. Everything below (names, environment variables, help text, the
`config env` listing) is derived from the struct, so a list somebody maintains
by hand cannot fall out of date.

## Layers and precedence

    default < file < environment < flag

All four are applied by one ordered load into a fresh value, so there is no
separate "fill defaults" pass that could run at the wrong time. Libraries that
fill defaults in a separate pass either overwrite values that came from a file
(envconfig, caarlos0/env), or confuse an explicit zero with "unset" (cleanenv,
go-envconfig).

- **default**: `WithDefaults(func(*T))` and the `Default` of a bound flag.
- **file**: the file named by `--config`, otherwise the first of
  `<name>.yaml`, `<name>.yml` that exists. A file that was named must exist.
- **environment**: `<PREFIX>_<PATH>` for every field (see Names).
- **flag**: flags bound with `Bind` that the user actually gave.

## Absent, present, empty

| | environment | flag | file | effect |
|---|---|---|---|---|
| absent | unset | not given | key missing | lower layer stays |
| present | `X=v` | `--x=v` | `x: v` | applied |
| empty | `X=` | `--x=` | `x:` / `x: null` | cleared to the zero value |

The unset/empty distinction uses `os.LookupEnv`. An accidental empty variable
(e.g. a compose template with nothing to substitute) clears a value; the origin
record shows `cleared`, so `config` makes it visible.

## Names

A field's name is taken from the first non-empty of: the `cfg` tag, the `yaml`
tag, the `json` tag, the field name in snake_case. `-` excludes the field.
Anonymous struct fields without a name, and fields tagged `,inline`, are inlined.

The environment variable is the prefix (the application name upper-cased, with
anything other than a letter or digit turned into `_`) plus the path joined with
`_`, upper-cased: `db.dsn` of `roster` is `ROSTER_DB_DSN`. An `env:"NAME"` tag
replaces the whole name; `env:"-"` takes the field out of the environment. Two
fields that end up with the same variable name are an error when the loader is
created, rather than one silently winning (ff does the same).

## Environment values

A string field takes the value as it is. Other scalars are parsed with
`strconv` (`time.Duration` with `time.ParseDuration`), or by
`encoding.TextUnmarshaler` when the type implements it.

Lists and maps have two forms, chosen by the first character:

- plain: `a,b,c` and `k=v,k2=v2`. `\,`, `\=` and `\\` escape. A plain value
  that starts with `[` or `{` is written `\[` / `\{`.
- flow: a value starting with `[` or `{` is YAML flow syntax, e.g.
  `[a, "b,c"]`, `{k: v}`, `[{name: x}]`. This is what payday reads today.

Comma lists are what most libraries use; `=` rather than `:` for maps keeps
`host:port` and URLs unambiguous, and matches flags such as `--base a=dc=x`.

## The file

`cfg` decodes the YAML document itself, walking the struct, rather than handing
it to a YAML decoder. This is what gives it:

- the same names for the file and the environment;
- strict keys: a key no field answers to is an error with its position, and a suggestion
  when one is close (`db.dns: nothing reads this key (did you mean "dsn"?)`). Keys
  starting with `x-` are ignored at every level, as in compose, so anchors have
  a place to live;
- raw scalars: `version: 1.10` into a string field is `"1.10"`, not `"1.1"`;
- positions for the origin record;
- references resolved per value (below).

Anchors, aliases and merge keys (`<<`) are supported; an alias names the
nearest anchor of that name before it. `any` fields get plain Go values with
references resolved. Types implementing goccy/go-yaml's unmarshaler interfaces
are handed their node as written, without references resolved.

Integers are read as YAML 1.2 reads them: decimal, or `0x`/`0o`/`0b` prefixed
(`010` is ten), and floats accept `.inf` and `.nan`.

Unknown environment variables under the prefix are only warnings: the
environment is shared with the orchestrator (Kubernetes service links put
dozens of `<SERVICE>_PORT_*` variables under a prefix that matches a service
name). Those are filtered out; `Claims(prefixes...)` declares names the
application reads itself.

## References

    ${env:NAME}  ${env:NAME:-default}  ${file:/path}  $$ (a literal $)

This replaces the three spellings the downstream applications used
(payday's text-level `${env:}`, cr's `${file:}`, roster/shale's bare
`env:`/`file:`).

- `${env:}` is resolved once, inside any string value of the file, also in the
  middle of a string (`postgres://u:${env:PW}@h`). Unset without a default is
  an error. Inside YAML flow syntax (`[...]`, `{...}`) a reference must be
  quoted (`["${env:A}"]`), because `{` and `}` delimit a flow mapping there;
  payday substituted the text before parsing and did not need that. The
  parse error says so.
- `${file:}` is only allowed in secret fields (below), because a file is how a
  credential gets rotated and only a secret field re-reads it.
- References are resolved per value after parsing, not by substituting text
  before parsing. That makes `$$` work everywhere (cr could not escape
  `${file:` because the text pass had already turned `$$` into `$`), and lets
  the origin record name the variable a value came through.
- Values from the environment and flags are not expanded, except in secret
  fields, which always read references.

## Secrets

    type SecretOf[T any, D Decoder[T]] struct{ ... }
    type Secret      = SecretOf[string, StringDecoder] // drops one trailing "\n" or "\r\n"
    type SecretBytes = SecretOf[[]byte, BytesDecoder]  // as read

A secret field holds a literal, `${env:NAME}`, `${file:/path}` or a reference
with a scheme registered with `WithScheme`. `Value()` returns the current value:
for `${file:}` it checks the file on each call and re-reads it when it changed.
The rules come from cr's `blob.SecretFile`, which gantry and bosun copy:

- changed means a different file at the path (`os.SameFile`, which catches the
  rename a rotation does) or a different size or modification time;
- a read that fails after a good one keeps the value in hand: a rotation
  renames first and fixes permissions after;
- the first read happens at load, but a file that cannot be read yet is a
  warning, not a failure: a credential may be minted after the process starts
  (cr's provisioning case). `Value` fails until the file has been read once,
  so a wrong path still shows on first use;
- an empty file is a failed read, and the file is capped at 64 KiB.

Only the trailing newline is removed from a string secret: whitespace inside or
before a credential is the credential's. Custom types implement `Decoder[T]`,
e.g. a 32-byte key or a set of S3 credentials that must be read together.

Secrets are redacted wherever `cfg` prints a configuration; a reference is
printed as written, since it says where the secret is, not what it is.

## Flags

    cfg.Bind(l, &c.Ldap.Addr, &flg.String{Name: "listen"})
    cfg.BindFunc(l, &c.Ldap.Tls, &flg.String{Name: "tls"}, parseCertPair)
    cfg.BindText(l, &c.Ldap.Key, &flg.String{Name: "deployment-key"})

The binding is declared on the flag, by pointer to the field. A struct tag was
considered and rejected: blocks such as payday's `DbConfig` are shared between
applications and must not fix flag names, and the same field may be reachable
from different commands under different flags.

- `Bind` requires the flag's value type to be the field's type, checked at
  compile time. `BindFunc` converts; `BindText` parses a string flag into a
  field that implements `encoding.TextUnmarshaler` (e.g. a `Secret`).
- A flag may also be bound to a block (a struct of fields), which it then sets
  whole, e.g. `--tls cert.pem,key.pem` into a `TlsConfig` with `BindFunc`. The
  origin of every field in the block is the flag. A block has no single
  environment variable, so help shows none for it.
- The flag is applied only if the user gave it (`Count() > 0`). Its `Default`
  is the default layer for the field.
- The returned flag reports the field's environment variable in `Info().Env`,
  so help and generated documentation show `[$ROSTER_LDAP_ADDR]`.
- Two flags on the executed command path bound to the same field, or to a block
  and a field in it, is an error.
- Flags are parsed before any handler runs, so the loader, mounted on the root
  command, sees the flags of every command on the path, including the
  subcommand's. This removes the "parse flags, load the file, copy the flags
  over by hand" step the downstream applications repeat.

## Origins

    o, ok := l.Origin(&c.Ldap.Addr)
    o.Source  // Unset, Default, File, Env, Flag
    o.Name    // "--listen", "ROSTER_LDAP_ADDR", "/etc/roster.yaml"
    o.Line, o.Column
    o.Refs    // references the value went through: ["${env:DB_PW}"]
    o.Cleared // set to empty
    o.IsSet() // File, Env or Flag

`ok` is false for a pointer that is not a leaf field of the loaded struct (for
example a field of a copy), instead of a guess. Lists and maps are one field.

## Errors and validation

A load collects every error instead of stopping at the first: bad values in the
file, unknown keys, bad environment values, failed flag conversions, unreadable
secrets. Afterwards `Validate() error` is called on every value in the tree that
implements it (structs, slice elements, map values), so each block checks only
itself. Every error is a `*FieldError` naming the field and its origin
(`--bind: ...`, `/etc/roster.yaml:12: ldap.bind: ...`), joined with
`errors.Join`.

## Reload

Environment variables cannot change under a running process; files can (a
Kubernetes ConfigMap update, a rotated Secret). So reload covers files only:

- secrets re-read their file on use (above);
- `Watch` re-reads the configuration file every interval (5 s by default), and
  when its content hash changed builds a new snapshot with the same
  environment and flags. A snapshot that fails to load or validate is reported
  once per content, and the previous one stays in force.
- Watch keeps to the file the first load found. If it disappears, that is
  reported and the snapshot in force stays; it does not fall back to the
  defaults or switch to another default path (`.yml` for `.yaml`).
- A file written in place can be read half-written. New content is therefore
  applied only once it read the same on two checks in a row, and an empty file
  is skipped during reload (an empty file is valid at the first load). Neither
  matters to a file replaced by rename, which is what Kubernetes does.

A reload never writes into the struct the application holds: it produces a new
`*Snapshot[T]` (as cr's policy store swaps an `atomic.Pointer`). The struct
passed to `New` holds the first load.

`NewFile[T](path)` is the same machinery for a file of its own, without
environment or flags, such as cr's `cr.auth.yaml`.

## Commands

`NewCmdConfig(l)` is `config` (the loaded configuration as YAML, secrets
redacted, each value commented with its origin) and `config env` (every
environment variable the struct reads; `--set` says which are set, never to
what).

## Not in scope

Remote sources (vault, consul; possible through `WithScheme`), formats other
than YAML, file system notifications (polling a small file is cheap and survives
the symlink swap Kubernetes does), live-reloading the environment.
