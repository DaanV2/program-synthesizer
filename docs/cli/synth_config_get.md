<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in internal/cli instead. -->

## synth config get

Print the value of one setting

### Synopsis

Print the value of one setting, in the form `config set` accepts.
Fails if an optional setting has no value.

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

```
synth config get <key> [flags]
```

### Examples

```
  synth config get name
  synth config get input
  synth config get input.address.city
  synth config get input.orders[].total
```

### Options

```
  -h, --help   help for get
```

### Options inherited from parent commands

```
      --log-format string   log format: text, json, logfmt (default "text")
      --log-level string    log level: debug, info, warn, error (default "info")
  -C, --project string      project directory; spec.yaml or spec.json is looked for here, then in parent directories (default ".")
```

### SEE ALSO

* [synth config](synth_config.md)	 - Show or change project settings in spec.yaml

