<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in internal/cli instead. -->

## synth data validate

Check every example against the spec

### Synopsis

Check that every example has both files, that every file is valid JSON,
that every value matches the types in spec.yaml, and that no two examples
(in either split) have the same input but different outputs. No program can
fit such a contradiction.

Prints each problem on stdout and exits with an error if there are any.
Otherwise prints the number of examples per split, and warns when there are
no training or no holdout examples.

```
synth data validate [flags]
```

### Examples

```
  synth data validate
  synth -C path/to/project data validate
```

### Options

```
  -h, --help   help for validate
```

### Options inherited from parent commands

```
      --log-format string   log format: text, json, logfmt (default "text")
      --log-level string    log level: debug, info, warn, error (default "info")
  -C, --project string      project directory; spec.yaml or spec.json is looked for here, then in parent directories (default ".")
```

### SEE ALSO

* [synth data](synth_data.md)	 - Add, import, list and validate examples

