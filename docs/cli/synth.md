<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in internal/cli instead. -->

## synth

Synthesise programs from input/output examples

### Synopsis

synth searches typed syntax trees built from a fixed operation library
for a program that maps every example input to its expected output.

A project is a directory with a spec.yaml (the input and output types) and
the examples under data/. Create one with `synth init`, add examples with
`synth data`, and adjust the spec with `synth config`. The search itself
(`synth solve`) is not implemented yet.

Commands that work on a project look for spec.yaml or spec.json in the
--project directory and then in its parents, so they work from any
subdirectory. Logs go to stderr; results go to stdout.

### Examples

```
  synth init fullname --yes --input 'first=string,last=string' --output string
  synth -C fullname data add --input in.json --output out.json
  synth -C fullname data validate
```

### Options

```
  -h, --help                help for synth
      --log-format string   log format: text, json, logfmt (default "text")
      --log-level string    log level: debug, info, warn, error (default "info")
  -C, --project string      project directory; spec.yaml or spec.json is looked for here, then in parent directories (default ".")
```

### SEE ALSO

* [synth config](synth_config.md)	 - Show or change project settings in spec.yaml
* [synth data](synth_data.md)	 - Add, import, list and validate examples
* [synth init](synth_init.md)	 - Create a new project (interactive wizard or flags)
* [synth solve](synth_solve.md)	 - Search for a program that fits the examples (not implemented yet)

