# Getting started

This page installs synth, creates a project and fills it with examples. It
ends where synth currently ends: `synth solve` is not implemented yet, so
there is no search to run. Everything before that works.

Commands are written for PowerShell. They work in bash and zsh as well; the
one place where the shells differ (inline JSON) is called out.

## Install

You need [Go](https://go.dev/dl/) 1.27 or newer and, for the recipes in
the `.justfile`, [just](https://github.com/casey/just):

```powershell
winget install GoLang.Go
winget install Casey.Just
```

Clone the repository, then build and install from its root:

```powershell
just build               # builds bin\synth.exe
go install .             # installs synth.exe into $(go env GOPATH)\bin
synth --version
```

`go install` puts `synth.exe` in the `bin` folder of `go env GOPATH`
(normally `%USERPROFILE%\go\bin`). If `synth` is not found, add that folder
to your PATH. The rest of this page assumes `synth` is on the PATH.

## The problem

The worked example is a function that takes a person's first name, last
name and age, and returns their full name and whether they are an adult:

```text
{first: "Ada", last: "Lovelace", age: 36}  ->  {fullName: "Ada Lovelace", isAdult: true}
```

## Create a project

`synth init` creates a directory with a `spec.yaml` (the types) and empty
example folders. Give it everything as flags:

```powershell
synth init fullname --yes --name full-name `
  --description 'Join first and last name; flag adults' `
  --input 'first=string,last=string,age=int' `
  --output 'fullName=string,isAdult=bool'
```

```text
Created project "full-name" in demo/fullname

Next steps:
  cd fullname
  synth data add ...          add examples (see `synth data --help`)
                              or write JSON files to data/train/in/<name>.json and data/train/out/<name>.json
  synth config list           review or change the spec
```

Run `synth init fullname` without `--yes` to get an interactive wizard that
asks for the same things and shows the `spec.yaml` before writing it. Set
`$env:ACCESSIBLE = 1` first for a line-by-line version that suits screen
readers.

Keep the single quotes around `--input` and `--output`. Types can contain
`< > { } [ ]` and commas, which PowerShell (and sh) would otherwise
interpret. Single quotes pass the text through unchanged in both shells.

The result:

```powershell
cd fullname
Get-Content spec.yaml
```

```yaml
name: full-name
description: Join first and last name; flag adults
input:
  first: string
  last: string
  age: int
output:
  fullName: string
  isAdult: bool
options:
  targets:
    - go
```

The order of the input parameters is the order of the generated function's
arguments. [project-format.md](project-format.md) describes every key.

## Add examples one at a time

An example is an input (an object with one key per parameter) and the
expected output. `synth data add` type-checks both and writes them as two
files, `data/train/in/<name>.json` and `data/train/out/<name>.json`.

### From files: works in every shell

```powershell
Set-Content in.json '{"first": "Alan", "last": "Turing", "age": 41}'
Set-Content out.json '{"fullName": "Alan Turing", "isAdult": true}'
synth data add --input in.json --output out.json --name turing
```

```text
INFO  added example split=train name=turing
data/train/in/turing.json
data/train/out/turing.json
```

The first line is a log message on stderr; the two paths are the result, on
stdout. On Windows the paths are printed with backslashes.

Use `Set-Content` to write JSON files. In Windows PowerShell 5.1, `>` and
`Out-File` write UTF-16 and `Set-Content -Encoding utf8` adds a byte order
mark, and synth rejects both:

```text
reading in.json: in.json: invalid character '\xff' looking for beginning of value.
```

### Inline JSON: depends on your PowerShell

`--in` and `--out` take the JSON on the command line. How PowerShell passes
double quotes to a program changed in version 7.3. Check yours with
`$PSVersionTable.PSVersion`.

PowerShell 7.3 or later (and bash, zsh):

```powershell
synth data add --in '{"first": "Ada", "last": "Lovelace", "age": 36}' --out '{"fullName": "Ada Lovelace", "isAdult": true}'
```

```text
INFO  added example split=train name=001
data/train/in/001.json
data/train/out/001.json
```

Windows PowerShell 5.1 (the `powershell.exe` that comes with Windows)
removes the inner double quotes, so escape each one as `\"`:

```powershell
synth data add --in '{\"first\": \"Ada\", \"last\": \"Lovelace\", \"age\": 36}' --out '{\"fullName\": \"Ada Lovelace\", \"isAdult\": true}'
```

Each form fails in the other shell, with one of these messages:

```text
invalid JSON in --in: invalid character 'f' looking for beginning of object key string.
invalid JSON in --in: invalid character '\\' looking for beginning of object key string.
```

The first means the quotes were stripped (use `\"`); the second means the
backslashes arrived (drop them). When in doubt, use files.

Without `--name`, examples are numbered: `001`, `002`, and so on, per split.

### Mistakes are caught before anything is written

```powershell
synth data add --in '{"first": "Grace", "last": "Hopper"}' --out '{"fullName": "Grace Hopper", "isAdult": true}'
```

```text
ERROR

invalid input from --in: input: missing field "age" (spec input is {first: string, last: string, age: int}).
```

Adding the same name twice is refused unless you pass `--overwrite`:

```text
example already exists: data/train/in/turing.json (use --overwrite to replace it, or pick another --name).
```

## Import many examples

### From CSV

CSV works when every input parameter is a scalar and the output is a scalar
or a record of scalars, as here. The header names the parameters, then
`out.<field>` for each output field (or just `out` for a scalar output):

```powershell
@'
first,last,age,out.fullName,out.isAdult
Grace,Hopper,85,Grace Hopper,true
Linus,Torvalds,54,Linus Torvalds,true
Greta,Thunberg,17,Greta Thunberg,false
Margaret,Hamilton,88,Margaret Hamilton,true
Ken,Thompson,81,Ken Thompson,true
Sam,Young,9,Sam Young,false
Barbara,Liskov,84,Barbara Liskov,true
Mo,Chen,18,Mo Chen,true
'@ | Set-Content people.csv
```

Some examples should be kept out of the search to check the result
afterwards: that is the holdout split. A program can fit every training
example and still be the wrong function; holdout examples it has never seen
catch that. `--holdout 0.25` sends a quarter of the rows there, chosen at
random; `--seed` makes the choice repeatable. Try it with `--dry-run` first:

```powershell
synth data import people.csv --holdout 0.25 --seed 7 --dry-run
```

```text
people.csv:2 -> holdout/001
people.csv:3 -> train/002
people.csv:4 -> train/003
people.csv:5 -> train/004
people.csv:6 -> train/005
people.csv:7 -> holdout/002
people.csv:8 -> train/006
people.csv:9 -> train/007
dry run: would import 8 example(s): 6 train, 2 holdout
```

Then for real:

```powershell
synth data import people.csv --holdout 0.25 --seed 7
```

```text
imported 8 example(s): 6 train, 2 holdout
```

Import is all or nothing. If any row is wrong, every problem is listed with
its line number and nothing is written:

```text
ERROR more.csv:2: column "age": invalid int "thirty-six"
ERROR more.csv:3: column "out.isAdult": invalid bool "yes" (want true/false or 1/0)

ERROR

found 2 problem(s) in 2 example(s); nothing was written.
```

### From two folders of JSON files

If your examples already exist as files, put inputs and outputs in two
folders with matching file names. The file name becomes the example name.

```powershell
New-Item -ItemType Directory -Force raw/in, raw/out | Out-Null
Set-Content raw/in/knuth.json '{"first": "Donald", "last": "Knuth", "age": 88}'
Set-Content raw/out/knuth.json '{"fullName": "Donald Knuth", "isAdult": true}'
Set-Content raw/in/mia.json '{"first": "Mia", "last": "Lee", "age": 4}'
Set-Content raw/out/mia.json '{"fullName": "Mia Lee", "isAdult": false}'
Set-Content raw/in/dahl.json '{"first": "Ole", "last": "Dahl", "age": 70}'
synth data import --inputs raw/in --outputs raw/out
```

```text
WARN  raw/in/dahl.json: no matching output in raw/out, skipped
imported 2 example(s): 2 train, 0 holdout
```

A file without a partner is skipped with a warning, not an error.

JSONL (one `{"input": ..., "output": ...}` object per line) is the third
source; see [`synth data import`](cli/synth_data_import.md).

## Look at the data

```powershell
synth data list
```

```text
┌─────────┬────────┬──────────────────────────────────────────┬──────────────────────────────────────────┐
│ SPLIT   │ NAME   │ INPUT                                    │ OUTPUT                                   │
├─────────┼────────┼──────────────────────────────────────────┼──────────────────────────────────────────┤
│ train   │ 001    │ {"first":"Ada","last":"Lovelace","age":… │ {"fullName":"Ada Lovelace","isAdult":tr… │
│ train   │ 002    │ {"first":"Linus","last":"Torvalds","age… │ {"fullName":"Linus Torvalds","isAdult":… │
│ train   │ 003    │ {"first":"Greta","last":"Thunberg","age… │ {"fullName":"Greta Thunberg","isAdult":… │
│ train   │ 004    │ {"first":"Margaret","last":"Hamilton","… │ {"fullName":"Margaret Hamilton","isAdul… │
│ train   │ 005    │ {"first":"Ken","last":"Thompson","age":… │ {"fullName":"Ken Thompson","isAdult":tr… │
│ train   │ 006    │ {"first":"Barbara","last":"Liskov","age… │ {"fullName":"Barbara Liskov","isAdult":… │
│ train   │ 007    │ {"first":"Mo","last":"Chen","age":18}    │ {"fullName":"Mo Chen","isAdult":true}    │
│ train   │ knuth  │ {"first":"Donald","last":"Knuth","age":… │ {"fullName":"Donald Knuth","isAdult":tr… │
│ train   │ mia    │ {"first":"Mia","last":"Lee","age":4}     │ {"fullName":"Mia Lee","isAdult":false}   │
│ train   │ turing │ {"first":"Alan","last":"Turing","age":4… │ {"fullName":"Alan Turing","isAdult":tru… │
│ holdout │ 001    │ {"first":"Grace","last":"Hopper","age":… │ {"fullName":"Grace Hopper","isAdult":tr… │
│ holdout │ 002    │ {"first":"Sam","last":"Young","age":9}   │ {"fullName":"Sam Young","isAdult":false} │
└─────────┴────────┴──────────────────────────────────────────┴──────────────────────────────────────────┘
```

Names are unique per split: `train/001` and `holdout/001` are different
examples. `--width 0` shows the values in full.

## Change the spec

`synth config` reads and changes single settings, so you do not need to edit
`spec.yaml` by hand (you can, though).

```powershell
synth config list
```

```text
name = full-name
description = Join first and last name; flag adults
input.first = string
input.last = string
input.age = int
output.fullName = string
output.isAdult = bool
options.targets = go
```

Generate Rust as well as Go:

```powershell
synth config set options.targets 'go,rust'
synth config get options.targets
```

```text
go,rust
```

`set` prints nothing when it succeeds. When a change touches the types,
synth checks the existing examples against the new spec. Adding a
parameter would break every example, because none of them has it:

```powershell
synth config set input.title string --strict
```

```text
ERROR 12 example problem(s) with the new spec:
  data/train/in/001.json: input: missing field "title"
  data/train/in/002.json: input: missing field "title"
  data/train/in/003.json: input: missing field "title"
  data/train/in/004.json: input: missing field "title"
  data/train/in/005.json: input: missing field "title"
  ... and 7 more

ERROR

not changed: 12 example(s) would no longer conform (--strict).
```

With `--strict` the change is refused. Without it the change is saved and
the same list is printed as a warning, so you can fix the data next. Other
useful forms:

```powershell
synth config move input.age --before first    # reorder parameters
synth config set output.initials string       # add an output field
synth config unset output.initials            # and remove it again
```

See [`synth config`](cli/synth_config.md) for every key.

## Validate

`synth data validate` checks every example: both files present, valid JSON,
values matching the types, and no contradictions.

```powershell
synth data validate
```

```text
ok: 12 example(s): 10 train, 2 holdout
```

A contradiction is two examples with the same input and different outputs.
No program can satisfy both, so it is reported as a problem:

```powershell
synth data add --in '{"first": "Ada", "last": "Lovelace", "age": 36}' --out '{"fullName": "Ada Lovelace", "isAdult": false}' --name ada-again
synth data validate
```

```text
contradiction: .../demo/fullname/data/train/in/001.json and .../demo/fullname/data/train/in/ada-again.json have the same input but different outputs

ERROR

found 1 problem(s).
```

`data add` and `data import` check each example against the types, but not
against the other examples, so run `validate` after adding data. Remove the
wrong example and validate again:

```powershell
Remove-Item data/train/in/ada-again.json, data/train/out/ada-again.json
synth data validate
```

```text
ok: 12 example(s): 10 train, 2 holdout
```

## Next: solve (not implemented yet)

```powershell
synth solve examples.json
```

```text
INFO  solving examples=examples.json timeout=30s

ERROR

solve: not implemented yet.
```

The search is the next milestone; see [architecture.md](architecture.md#milestones)
for where the project stands.

## Running from elsewhere

Commands look for `spec.yaml` in the current directory and then its parents,
so they work from any subdirectory of the project. From outside it, point at
the project with `-C`:

```powershell
cd ..
synth -C fullname data validate
```

## Shell completion

synth has cobra's standard `completion` command. For PowerShell, add this line to your
profile (`notepad $PROFILE`):

```powershell
synth completion powershell | Out-String | Invoke-Expression
```

Completion knows the `synth config` keys of the current project.
