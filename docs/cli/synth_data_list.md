<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in cmd/ instead. -->

## synth data list

List examples with a preview of their values

### Synopsis

List the examples of the project as a table with a one-line preview of each
input and output. Files without a partner are reported as warnings.
Values are not type-checked here; use `synth data validate` for that.

Alias: `synth data ls`.

```
synth data list [flags]
```

### Examples

```
  synth data list
  synth data list --split holdout --width 80
```

### Options

```
  -h, --help           help for list
      --split string   only list this split: train or holdout (default: both)
      --width int      truncate previews to this many characters (0 = no limit) (default 40)
```

### Options inherited from parent commands

```
      --log-format string   log format: text, json, logfmt (default "text")
      --log-level string    log level: debug, info, warn, error, fatal (default "info")
  -C, --project string      project directory; spec.yaml or spec.json is looked for here, then in parent directories (default ".")
```

### SEE ALSO

* [synth data](synth_data.md)	 - Add, import, list and validate examples

