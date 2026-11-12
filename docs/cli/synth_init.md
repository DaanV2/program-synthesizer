<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in cmd/ instead. -->

## synth init

Create a new project (interactive wizard or flags)

### Synopsis

Create a project: spec.yaml plus empty data/train and data/holdout directories.

dir defaults to --project (the current directory unless -C is given) and is
created if missing. init refuses to run where a spec.yaml or spec.json exists.
--name defaults to the directory name.

In a terminal, init asks for the name, description, input parameters, output
type and options, shows the resulting spec.yaml and asks before writing it.
Values given as flags are used as defaults, and --input/--output skip those
steps. Set ACCESSIBLE=1 for a line-by-line wizard that suits screen readers.

With --yes, or when stdin is not a terminal (scripts, CI), no questions are
asked and --input and --output are required.

Writing --input and --output:

```text
name=type pairs, comma separated:   --input 'first=string,last=string,age=int'
a YAML flow mapping:                --input '{first: string, tags: list<string>}'
repeated flags are concatenated:    --input first=string --input age=int
```

Types with commas inside brackets are fine in pairs: 'pos={x: int, y: int},n=int'.
--output takes a single type ('int', 'list&lt;string&gt;', '[{id: int}]') or a record
in either of the forms above. Quote these values: &lt; &gt; { } [ ] and , mean
something to PowerShell and sh. Single quotes work in both.

```
synth init [dir] [flags]
```

### Examples

```
  # Interactive wizard in a new directory
  synth init fullname

  # Non-interactive
  synth init fullname --yes --name full-name --input 'first=string,last=string,age=int' --output 'fullName=string,isAdult=bool' --targets 'go,rust'

  # Records and lists of records
  synth init orders -y --input '{customer: {name: string}, orders: [{id: int, total: float}]}' --output float --float-tolerance 1e-9
```

### Options

```
      --description string      project description
      --float-tolerance float   tolerance when comparing float outputs (default: unset)
  -h, --help                    help for init
      --input stringArray       input parameters: 'name=type,...' or '{name: type, ...}' (repeatable)
      --name string             project name (default: the directory name)
      --output string           output type, or a record as 'name=type,...' / '{name: type, ...}'
      --targets string          code generation targets, comma separated: go, rust, c (default go)
  -y, --yes                     do not ask anything; use the flags (requires --input and --output)
```

### Options inherited from parent commands

```
      --log-format string   log format: text, json, logfmt (default "text")
      --log-level string    log level: debug, info, warn, error, fatal (default "info")
  -C, --project string      project directory; spec.yaml or spec.json is looked for here, then in parent directories (default ".")
```

### SEE ALSO

* [synth](synth.md)	 - Synthesise programs from input/output examples

