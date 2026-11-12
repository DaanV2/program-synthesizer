<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in internal/cli instead. -->

## synth config

Show or change project settings in spec.yaml

### Synopsis

Read and change single settings of the project spec without an editor.

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

### Examples

```
  synth config list
  synth config get input.age
  synth config set description "Join first and last name"
  synth config set input.tags 'list<string>'
  synth config set output '{fullName: string, isAdult: bool}'
  synth config set options.targets 'go,rust'
  synth config unset input.tags
  synth config move input.age --before first
```

### Options

```
  -h, --help   help for config
```

### Options inherited from parent commands

```
      --log-format string   log format: text, json, logfmt (default "text")
      --log-level string    log level: debug, info, warn, error (default "info")
  -C, --project string      project directory; spec.yaml or spec.json is looked for here, then in parent directories (default ".")
```

### SEE ALSO

* [synth](synth.md)	 - Synthesise programs from input/output examples
* [synth config get](synth_config_get.md)	 - Print the value of one setting
* [synth config list](synth_config_list.md)	 - Print every setting as key = value
* [synth config move](synth_config_move.md)	 - Reorder a parameter or record field
* [synth config set](synth_config_set.md)	 - Set or add one setting
* [synth config unset](synth_config_unset.md)	 - Remove one setting

