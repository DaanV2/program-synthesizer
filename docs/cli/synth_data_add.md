<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in cmd/ instead. -->

## synth data add

Add one example

### Synopsis

Add one example to the project. The input and the output each come from a
JSON file (--input/--output, '-' reads stdin) or from inline JSON (--in/--out).
Both are type-checked against spec.yaml before anything is written.

The input is always an object with one key per input parameter. Files must be
UTF-8 without a byte order mark.

Without --name the example gets the next free number in the split (001, 002, ...).
The two files are written with record fields in spec order, and their paths
are printed.

Inline JSON and PowerShell: PowerShell 7.3 and later pass '{"x": 3}' through
unchanged. Windows PowerShell 5.1 strips the inner double quotes, so there
write '{\"x\": 3}'. Files (--input/--output) work the same in every shell.

```
synth data add [flags]
```

### Examples

```
  # inline JSON (bash, zsh, PowerShell 7.3+)
  synth data add --in '{"x": 3}' --out 7

  # inline JSON in Windows PowerShell 5.1: escape the inner quotes
  synth data add --in '{\"x\": 3}' --out 7

  # from files (any shell), into the holdout split, with a chosen name
  synth data add --input in.json --output out.json --split holdout --name edge-case

  # input from stdin
  echo '{"x": 3}' | synth data add --input - --out 7
```

### Options

```
  -h, --help            help for add
      --in string       the input as inline JSON
      --input string    read the input from this JSON file ('-' for stdin)
      --name string     example name (default: next free number, e.g. 004)
      --out string      the output as inline JSON
      --output string   read the output from this JSON file ('-' for stdin)
      --overwrite       replace an existing example with the same name
      --split string    split to add to: train or holdout (default "train")
```

### Options inherited from parent commands

```
      --log-format string   log format: text, json, logfmt (default "text")
      --log-level string    log level: debug, info, warn, error, fatal (default "info")
  -C, --project string      project directory; spec.yaml or spec.json is looked for here, then in parent directories (default ".")
```

### SEE ALSO

* [synth data](synth_data.md)	 - Add, import, list and validate examples

