<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in internal/cli instead. -->

## synth config list

Print every setting as key = value

### Synopsis

Print every setting that has a value, one per line as `key = value`, in spec order.
Input parameters are listed one per line; so are output fields when the output is a record.

Alias: `synth config ls`.

```
synth config list [flags]
```

### Examples

```
  synth config list
  synth -C path/to/project config list
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --log-format string   log format: text, json, logfmt (default "text")
      --log-level string    log level: debug, info, warn, error (default "info")
  -C, --project string      project directory; spec.yaml or spec.json is looked for here, then in parent directories (default ".")
```

### SEE ALSO

* [synth config](synth_config.md)	 - Show or change project settings in spec.yaml

