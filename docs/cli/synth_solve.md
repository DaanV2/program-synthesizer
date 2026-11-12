<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in internal/cli instead. -->

## synth solve

Search for a program that fits the examples (not implemented yet)

### Synopsis

Not implemented yet: the command exists as a placeholder and always fails.

It will run the search described in spec.md (milestones M1 and M2 first) and
print the program it finds. The argument and flags are placeholders and will
change; the plan is for solve to work on the current project, like the data
commands, instead of taking an examples file.

```
synth solve <examples-file> [flags]
```

### Examples

```
  synth solve examples.json --timeout 1m
```

### Options

```
  -h, --help             help for solve
      --timeout string   search time budget (default "30s")
```

### Options inherited from parent commands

```
      --log-format string   log format: text, json, logfmt (default "text")
      --log-level string    log level: debug, info, warn, error (default "info")
  -C, --project string      project directory; spec.yaml or spec.json is looked for here, then in parent directories (default ".")
```

### SEE ALSO

* [synth](synth.md)	 - Synthesise programs from input/output examples

