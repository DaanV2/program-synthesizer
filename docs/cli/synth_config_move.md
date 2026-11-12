<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in internal/cli instead. -->

## synth config move

Reorder a parameter or record field

### Synopsis

Move a parameter or record field before or after one of its siblings. The order of
input parameters is the order of the generated function's arguments. Example files
are JSON objects, so reordering never breaks them.

Saving rewrites spec.yaml. Comments and flow/block style are carried over
for keys that still exist; comments on removed keys and blank lines are lost.
A project with spec.json is rewritten as JSON. Nothing is printed on success.

```
synth config move <key> --before|--after <field> [flags]
```

### Examples

```
  synth config move input.age --before first
  synth config move input.address.zip --after city
  synth config move output.isAdult --before fullName
```

### Options

```
      --after field    sibling field to move behind
      --before field   sibling field to move in front of
  -h, --help           help for move
```

### Options inherited from parent commands

```
      --log-format string   log format: text, json, logfmt (default "text")
      --log-level string    log level: debug, info, warn, error (default "info")
  -C, --project string      project directory; spec.yaml or spec.json is looked for here, then in parent directories (default ".")
```

### SEE ALSO

* [synth config](synth_config.md)	 - Show or change project settings in spec.yaml

