<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in cmd/ instead. -->

## synth data import

Import many examples from directories, JSONL or CSV

### Synopsis

Import many examples at once. The source is one of:

```text
--inputs DIR --outputs DIR
    Pairs *.json files with the same basename. The basename becomes the
    example name (if it is a valid name). Unpaired files are reported and skipped.

FILE.jsonl (or .ndjson)
    One example per line: {"input": {...}, "output": ...}. Names are generated
    (next free number in the split).

FILE.csv
    Only when every input parameter is a scalar (int, float, bool, string) and the
    output is a scalar or a record of scalars. The header names the parameters,
    plus 'out' for a scalar output or 'out.<field>' for each output field.
    Bools may be true/false or 1/0. String cells are taken verbatim.
```

Use '-' as FILE to read stdin (needs --format). A UTF-8 byte order mark at the
start of a JSONL or CSV file is ignored.

Every example is checked against the spec first; if any of them has a problem,
all problems are printed and nothing is written. Examples are not compared with
each other or with existing data, so run `synth data validate` afterwards to
find contradictions.

--holdout sends round(fraction \* count) randomly chosen examples to the holdout
split. The choice depends only on the number of examples, the fraction and
--seed, so the same import gives the same split on every machine.

```
synth data import [file] [flags]
```

### Examples

```
  # two directories of matching files
  synth data import --inputs raw/in --outputs raw/out

  # JSONL, sending a reproducible 20% to the holdout split
  synth data import examples.jsonl --holdout 0.2 --seed 42

  # CSV for spec input {a: int, b: int} and output int:
  #   a,b,out
  #   1,2,3
  synth data import sums.csv --dry-run

  # from stdin
  cat examples.jsonl | synth data import - --format jsonl
```

### Options

```
  -n, --dry-run          check everything and show what would be written, but write nothing
      --format string    file format: jsonl or csv (default: from the file extension)
  -h, --help             help for import
      --holdout float    send this fraction (0-1) of the examples to holdout, chosen at random
      --inputs string    directory of input *.json files (with --outputs)
      --outputs string   directory of output *.json files (with --inputs)
      --overwrite        replace existing examples with the same name
      --seed uint        random seed for --holdout (same seed and data give the same split) (default 1)
      --split string     split to import into: train or holdout (default "train")
```

### Options inherited from parent commands

```
      --log-format string   log format: text, json, logfmt (default "text")
      --log-level string    log level: debug, info, warn, error, fatal (default "info")
  -C, --project string      project directory; spec.yaml or spec.json is looked for here, then in parent directories (default ".")
```

### SEE ALSO

* [synth data](synth_data.md)	 - Add, import, list and validate examples

