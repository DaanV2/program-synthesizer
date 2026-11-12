<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in cmd/ instead. -->

## synth config set

Set or add one setting

### Synopsis

Set one setting. Setting a parameter or record field that does not exist adds it
at the end of its record.

After a type change the existing examples are checked against the new spec. Examples
that no longer conform are reported as a warning; with --strict the change is refused.

Keys:

```text
name                      project name
description               free text
input                     all parameters at once, as a record: '{x: int, y: int}'
input.<param>             one parameter's type; set adds it at the end if missing
input.<a>.<b>             a field of a record parameter (missing records are created)
input.<a>[].<b>           a field of the records in a list-of-records parameter
output                    the whole output type: 'int', 'list<int>', '{fullName: string}'
output.<field>            a field of a record output (same path rules as input)
options.float_tolerance   number >= 0 used when comparing float outputs
options.targets           comma-separated back ends: go, rust, c
```

Types use spec.yaml syntax on one line: int, float, bool, string, list&lt;T&gt;,
[T], and records as {name: type, ...}. Quote them in the shell: single quotes
work in both PowerShell and sh.

Saving rewrites spec.yaml. Comments and flow/block style are carried over
for keys that still exist; comments on removed keys and blank lines are lost.
A project with spec.json is rewritten as JSON. Nothing is printed on success.

```
synth config set <key> <value> [flags]
```

### Examples

```
  synth config set name full-name
  synth config set input.age int
  synth config set input.address '{city: string, zip: string}'
  synth config set input.address.country string
  synth config set input.orders '[{id: int, total: float}]'
  synth config set output 'list<int>'
  synth config set output.isAdult bool --strict
  synth config set options.float_tolerance 1e-6
```

### Options

```
  -h, --help     help for set
      --strict   refuse the change if existing examples would no longer conform
```

### Options inherited from parent commands

```
      --log-format string   log format: text, json, logfmt (default "text")
      --log-level string    log level: debug, info, warn, error, fatal (default "info")
  -C, --project string      project directory; spec.yaml or spec.json is looked for here, then in parent directories (default ".")
```

### SEE ALSO

* [synth config](synth_config.md)	 - Show or change project settings in spec.yaml

