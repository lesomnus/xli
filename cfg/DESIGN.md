# cfg design

`cfg` is the optional configuration layer for xli applications. It loads an
application's own Go struct from a file, the environment and command-line flags,
records where every value came from, and keeps file-backed values current while
the process runs.

It is a separate module (`github.com/lesomnus/xli/cfg`) so that xli itself stays
free of a YAML dependency. It depends on xli's public API only, not on its
internal packages, so that it builds against any later xli.

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

- **default**: what the root holds when the loader is made, and the `Default`
  of a bound flag on the command path being run.

      c := Config{Db: DbConfig{MaxConn: 10}}
      l := cfg.New("roster", &c)

  Defaults compose as struct literals do: a shared block brings its defaults
  as a value (`Db: payday.DefaultDb()`), the application sets its own around
  it, and a test fills in what it needs before making the loader. Every load
  starts from a deep copy of them, and a value that decodes itself is decoded
  into a new one, so nothing a load does writes into the defaults, nor into
  what they share with a package variable.
- **file**: the file named by `--config`, otherwise the first of
  `<name>.yaml`, `<name>.yml` that exists. A file that was named must exist;
  `--config=` reads none.
- **environment**: `<PREFIX>_<PATH>` for every field (see Names).
- **flag**: flags bound with `Bind` that the user actually gave.

The first load is read into the root, and the root is its snapshot's Config:
`config` prints what the application changed after loading, and `Origin` takes
pointers into it.

## Absent, present, empty

| | environment | flag | file | effect |
|---|---|---|---|---|
| absent | unset | not given | key missing | lower layer stays |
| present | `X=v` | `--x=v` | `x: v` | applied |
| empty | `X=` | `--x=` | `x:` / `x: null` | cleared to the zero value |

The unset/empty distinction uses `os.LookupEnv`. An accidental empty variable
(e.g. a compose template with nothing to substitute) clears a value; the origin
record shows `cleared`, so `config` makes it visible. A flag can be empty if its
value is text: a string, or a list, which `--x=` clears rather than making
`[""]`. Clearing a field does not make the nil block it is in.

## Names

A field's name is taken from the first non-empty of: the `cfg` tag, the `yaml`
tag, the `json` tag, the field name in snake_case. `-` excludes the field.
Anonymous struct fields without a name, and fields tagged `,inline`, are inlined.

The environment variable is the prefix (the application name upper-cased, with
anything other than a letter or digit turned into `_`) plus the path joined with
`_`, upper-cased: `db.dsn` of `roster` is `ROSTER_DB_DSN`. An `env:"NAME"` tag
replaces the whole name; `env:"-"` takes the field out of the environment.

What a file or the environment could not read is refused when the loader is
made, rather than ignored: two fields, or two blocks of inlined structs, with
one name; two fields with one variable (ff does the same); a name starting with
`x-`, which the file ignores; an `env` tag that is not a variable name (such as
another library's `env:"DB_DSN,required"`); a pointer to a pointer.

## Environment values

A value from the environment, or from a flag, is taken as it is given: it is
not expanded, in flow syntax neither. A string field takes the value as it is.
Other scalars are parsed with `strconv` (`time.Duration` with
`time.ParseDuration`), or by `encoding.TextUnmarshaler` when the type
implements it.

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
  starting with `x-` are ignored in every block, as in compose, so anchors have
  a place to live; in a map they are data like any other key;
- raw scalars: `version: 1.10` into a string field is `"1.10"`, not `"1.1"`;
- positions for the origin record;
- references resolved per value (below).

A file is one document; a `%YAML` directive and a byte order mark are allowed.

Anchors, aliases and merge keys (`<<`) are supported. An alias names the
nearest anchor of that name before it; an alias to a value it is in is an
error, as are aliases that expand to more than a million values. Merge keys
are read as specified: a key of the mapping replaces a merged one whole (blocks
are not merged deeply), and an earlier mapping in a list of them overrides a
later one. The origin of a value given by an alias is where the value is
written, at its anchor.

Integers are read as YAML 1.2 reads them, in `any` fields too: decimal, or
`0x`/`0o`/`0b` prefixed (`010` is ten), no underscores; floats accept `.inf`
and `.nan`. `!!str` makes any scalar a string (`!!str ~` is `"~"`), and
`!!binary` is read into a `[]byte`. Map keys written differently that read the
same, such as `1` and `01`, are an error.

`any` fields get plain Go values with references resolved. A type that
implements goccy/go-yaml's unmarshaler interfaces (payday's `OtelConfig`,
through `mkot.Config`) is given its block as plain values, with references
resolved and aliases and merges expanded, mappings in order and keys of their
own types: handed the block as written, it would see neither.

Unknown environment variables under the prefix are only warnings: the
environment is shared with the orchestrator. Kubernetes gives every service in
a namespace `<SERVICE>_SERVICE_HOST`, `<SERVICE>_PORT` and the like, under a
prefix that matches the application's when a service is named after it; the
variables of the services that are there are left out, and a variable of that
shape for no such service is a typo like any other. `Reads(prefixes...)`
declares names the application reads itself.

## References

    ${env:NAME}  ${env:NAME:-default}  ${file:/path}  $$ (a literal $)

This replaces the three spellings the downstream applications used
(payday's text-level `${env:}`, cr's `${file:}`, roster/shale's bare
`env:`/`file:`).

- `${env:}` is resolved once, inside any string value of the file, also in the
  middle of a string (`postgres://u:${env:PW}@h`). Unset without a default is
  an error, and so is a reference in a reference. Inside YAML flow syntax
  (`[...]`, `{...}`) a reference must be quoted (`["${env:A}"]`), because `{`
  and `}` delimit a flow mapping there; payday substituted the text before
  parsing and did not need that. The parse error says so.
- `${file:}` is only allowed in secret fields (below), because a file is how a
  credential gets rotated and only a secret field re-reads it.
- References are resolved per value after parsing, not by substituting text
  before parsing. That makes `$$` work everywhere (cr could not escape
  `${file:` because the text pass had already turned `$$` into `$`), and lets
  the origin record name the variable a value came through.
- Values from the environment and flags are taken as they are given. A secret
  field reads a reference from them only as the whole value: a password such as
  `Pa$$w0rd`, handed over by a Kubernetes Secret, must not lose a `$`.

## Secrets

    type SecretOf[T any, D Decoder[T]] struct{ ... }
    type Secret      = SecretOf[string, StringDecoder] // drops one trailing "\n" or "\r\n"
    type SecretBytes = SecretOf[[]byte, BytesDecoder]  // as read

    SecretOf[string, TrimSpaceDecoder]                 // drops the whitespace around it

A secret field holds a literal, `${env:NAME}`, `${file:/path}` or a reference
with a scheme registered with `WithScheme`. In the file, a literal may hold `$`
as `$$`; from the environment or a flag, anything but exactly one reference is
a literal as it is. A literal written as roster and shale wrote references,
`file:/path` or `env:NAME`, is an error that says how to write one, rather than
a credential.

`Value()` returns the current value: for `${file:}` it checks the file on each
call and re-reads it when it changed. The rules come from cr's
`blob.SecretFile`, which gantry and bosun copy:

- changed means a different file at the path (`os.SameFile`, which catches the
  rename a rotation does) or a different size or modification time;
- a read that fails after a good one keeps the value in hand: a rotation
  renames first and fixes permissions after;
- the first read happens at load, but a file that cannot be read yet is a
  warning, not a failure: a credential may be minted after the process starts
  (cr's provisioning case). `Value` fails until the file has been read once,
  so a wrong path still shows on first use;
- an empty file is a failed read; only a regular file is read (a FIFO would
  block), and no more than 64 KiB of it.

Only the trailing newline is removed from a `Secret`: whitespace inside or
before a credential is the credential's. `TrimSpaceDecoder` is the other rule,
for a credential nothing at the edges of can be part of, as a token: the
whitespace around it goes, and nothing but whitespace is empty. cr, gantry and
bosun each kept that decoder before cfg did. Custom types implement
`Decoder[T]`, e.g. a 32-byte key, or a set of S3 credentials that must be read
together. A `[]byte` from `Value` is a copy the caller may clear.

`(*SecretOf).SetFile(path)` makes a secret from a path rather than from a
configuration value, by the same rules as `${file:path}`: for an application
whose configuration names the file, such as a `token_file`, rather than holding
a reference to it.

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
- A field in a block behind a pointer has an address only once the block is
  made: make it after `New`, so that the default stays nil.
- The flag is applied only if the user gave it (`Count() > 0`). Its `Default`
  is the default layer for the field, on the command path the flag is on:
  `serve --listen` with a default of `:389` makes `ldap.addr` `:389` for
  `serve`, not for `config`, which is another path. A default every command
  should see belongs in the root's value.
- The returned flag reports the field's environment variable in `Info().Env`,
  so help and generated documentation show `[$ROSTER_LDAP_ADDR]`.
- Two flags on the executed command path bound to the same field, or to a block
  and a field in it, is an error.
- Flags are parsed before any handler runs, so the loader sees the flags of
  every command on the path, above and below the command it is mounted on.
  This removes the "parse flags, load the file, copy the flags over by hand"
  step the downstream applications repeat.

## Origins

    o, ok := l.Origin(&c.Ldap.Addr)
    o.Source  // SourceUnset, SourceDefault, SourceFile, SourceEnv, SourceFlag
    o.Name    // "--listen", "ROSTER_LDAP_ADDR", "/etc/roster.yaml"
    o.Line, o.Column
    o.Refs    // references the value went through: ["${env:DB_PW}"]
    o.Cleared // set to empty
    o.IsSet() // file, environment or flag

`ok` is false for a pointer that is not a leaf field of the loaded struct (for
example a field of a copy), instead of a guess. Lists and maps are one field.

## Errors and validation

A load collects every error instead of stopping at the first: bad values in the
file, unknown keys, bad environment values, failed flag conversions, unreadable
secrets. Afterwards `Validate() error` is called on every value in the tree that
implements it (structs, slice elements, map values), so each block checks only
itself. Every error is a `*FieldError` naming the field and its origin
(`--bind: ...`, `/etc/roster.yaml:12:3: ldap.bind: ...`), gathered in a
`*LoadError`.

A load runs for every command that needs the configuration, so `Validate`
checks what holds for all of them: that a value is well formed, that two
values agree. That `serve` needs a database while `version` does not is
checked where `serve` builds what it serves, as payday does.

## Reload

Environment variables cannot change under a running process; files can (a
Kubernetes ConfigMap update, a rotated Secret). So reload covers files only:

- secrets re-read their file on use (above);
- `Reload` checks the configuration file once, and `Watch` does every interval
  (5 s by default). When the content changed, a new snapshot is built with the
  same environment and flags.
- New content is loaded once it read the same at two checks in a row: a file
  written in place can be read half-written. A file replaced by a rename, as
  Kubernetes does, waits the one check as well. An empty file is content like
  any other: emptying a policy file is how all of it is revoked.
- Content that fails to load or validate leaves the snapshot in force, and is
  tried again at every check: what failed may be a scheme's server, or a file a
  `Validate` looks at. `Watch` reports each failure once, and reports the
  snapshot in force when the file reads as it again, so that what reported the
  failure can tell it is over. An application that keeps its own schedule, or
  counts every check (cr's policy store does), calls `Reload` itself.
- Reload keeps to the file a load found, or the first one it found itself. If
  it disappears, that is reported and the snapshot in force stays; it does not
  fall back to the defaults or switch to another default path (`.yml` for
  `.yaml`).

A reload never writes into the struct the application holds: it produces a new
`*Snapshot[T]` (as cr's policy store swaps an `atomic.Pointer`). The root holds
the first load.

`NewFile[T](path)` is a `File[T]`: the same loading and reloading for a file of
its own, such as cr's `cr.auth.yaml`, with no environment variables or flags
over it (`${env:}` references in it are resolved).

## Commands

`Load(l, except...)` is the handler that loads the configuration, on the root
command or on each command that needs it, once per run. A configuration that
fails to load fails the command, so the commands that need none are given as
`except`, with the commands under them: `xli.NewCmdCompletion()` must work
before there is a configuration.

`NewCmdConfig(l)` is `config` and `config env`:

- `config` prints the configuration as YAML that reads back as the same
  configuration, each value commented with its origin. A configuration that
  fails to load is printed as far as it was read, followed by what is wrong.
- `config env` lists every environment variable the struct reads; `--set` says
  which are set, never to what. It does not load the configuration.

What `config` prints is meant to be pasted into a ticket, so it redacts:
`Secret`s and fields tagged `cfg:",secret"` (a tag on a block covers it), at
any depth; and, as payday's `config` did, every value under a name that says it
is secret (`token`, `password`, `secret(s)`, `seal`, `key(s)`,
`credential(s)`, and names ending in `_token`, `_password`, `_secret`, `_key`,
`_keys`), because a tag is what somebody forgets on the one field that matters;
and the password in a value named `dsn` or ending in `_dsn`. A value read
through references prints as written, which says where a secret is rather than
what it is.

## Not in scope

Remote sources (vault, consul; possible through `WithScheme`), formats other
than YAML, file system notifications (polling a small file is cheap and survives
the symlink swap Kubernetes does), live-reloading the environment.
