<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in internal/cli instead. -->

## synth data

Add, import, list and validate examples

### Synopsis

Manage the input/output examples of a project.

Examples live in `data/<split>/in/<name>.json` and `data/<split>/out/<name>.json`,
where &lt;split&gt; is train (used for search) or holdout (used to catch overfitting).
An example is the pair of files with the same name in one split.

### Examples

```
  synth data add --input in.json --output out.json
  synth data import examples.csv --holdout 0.2
  synth data list
  synth data validate
```

### Options

```
  -h, --help   help for data
```

### Options inherited from parent commands

```
      --log-format string   log format: text, json, logfmt (default "text")
      --log-level string    log level: debug, info, warn, error (default "info")
  -C, --project string      project directory; spec.yaml or spec.json is looked for here, then in parent directories (default ".")
```

### SEE ALSO

* [synth](synth.md)	 - Synthesise programs from input/output examples
* [synth data add](synth_data_add.md)	 - Add one example
* [synth data import](synth_data_import.md)	 - Import many examples from directories, JSONL or CSV
* [synth data list](synth_data_list.md)	 - List examples with a preview of their values
* [synth data validate](synth_data_validate.md)	 - Check every example against the spec

