<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in cmd/ instead. -->

## synth config unset

Remove one setting

### Synopsis

Remove an optional setting (description, options.\*) or a parameter or record field.
name, input and output themselves are required and cannot be removed; neither can the
last field of a record. Unsetting an optional setting that has no value does nothing.

Saving rewrites spec.yaml. Comments and flow/block style are carried over
for keys that still exist; comments on removed keys and blank lines are lost.
A project with spec.json is rewritten as JSON. Nothing is printed on success.

```
synth config unset <key> [flags]
```

### Examples

```
  synth config unset description
  synth config unset options.float_tolerance
  synth config unset input.tags
  synth config unset output.isAdult --strict
```

### Options

```
  -h, --help     help for unset
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

