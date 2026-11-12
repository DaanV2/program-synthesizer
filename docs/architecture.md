# Architecture

This page is for working on synth. The design of the synthesizer itself
(search, scoring, decomposition, code generation) is in
[spec.md](../spec.md); this page describes the code that exists and how it
fits together.

## Packages

```text
cmd/synth/         main: runs the cobra tree through fang
cmd/gendocs/       main: writes the CLI reference in docs/cli (`just docs`)
internal/cli/      cobra commands: flag parsing and printing only
internal/project/  spec, type system, values, data/ layout, validation
internal/dataio/   import sources (folders, JSONL, CSV) and import planning
internal/clidocs/  Markdown generation for docs/cli, and its staleness test
internal/synth/    the synthesizer (M1 onwards); empty so far
```

Imports go one way: `cli` uses `dataio` and `project`; `dataio` uses
`project`; `project` uses neither. `clidocs` uses `cli` to build the same
command tree the binary runs.

**cmd/synth** calls `fang.Execute` with `cli.NewRootCmd()`. fang adds the
`--version` flag, the hidden `man` command, styled help and error output, and
cancels the command's context on Ctrl+C. The custom error handler only stops
fang from title-casing messages, because they often start with a path or a
flag.

**internal/cli** holds one file per command group: `init.go` (plus
`init_spec.go` for parsing `--input`/`--output` into a `specDraft`,
`init_form.go` for the line-by-line wizard and `init_tui.go` for the
full-screen one), `config.go`, `data.go`, `solve.go`. `project.go` has
`loadProject`, which every project command uses to honour `-C`.

**internal/project** is everything about a project on disk:

| File | Contents |
| --- | --- |
| `types.go` | `Type`, `Kind`, `Field`; parsing the string form and YAML nodes |
| `spec.go` | `Spec`, `Options`; YAML and JSON decoding, encoding, `Validate` |
| `config_keys.go` | dotted keys for `synth config`: `ParseKey`, `Spec.Get/Set/Unset/Move/List`, `ParseTypeExpr`, `FormatType` |
| `save_comments.go` | rewriting spec.yaml while keeping comments |
| `value.go` | decoding JSON values, `Type.Check`, `EncodeValue`, `CanonicalValue`, atomic writes |
| `project.go` | `Project`, `Find`, `Load`, `Create`, example pairing, `Validate`, `WriteExample` |

**internal/dataio** reads examples from outside a project and decides where
they go. It never writes except through `Apply`.

## Key types

`project.Type` is a value type: `Kind` is one of `Int`, `Float`, `Bool`,
`String`, `List`, `Record`; lists have `Elem`; records have `Fields`, an
ordered slice of `Field{Name, Type}`. Build types with `Scalar`, `ListOf`,
`RecordOf` and `F`. There are two string forms: `Type.String()` (always
`list<...>`, used in messages) and `FormatType` (uses `[T]` for lists that
contain records, so `ParseTypeExpr` can read it back). `Equal` compares
structurally and record order counts.

`project.Spec` mirrors spec.yaml: `Name`, `Description`, `Input` (always a
record), `Output`, `Options{FloatTolerance, Targets}`. `ParseSpecYAML`
decodes with unknown keys rejected and then calls `Validate`. spec.json is
decoded through the YAML decoder too, so field order is kept.

`project.Project` is a loaded project: `Dir`, `SpecPath` (spec.yaml or
spec.json; saving writes back to the same format) and `Spec`.

`project.Example` names one pair of files: `Split`, `Name`, `InputPath`,
`OutputPath`. `LoadExample` turns it into a `LoadedExample` with the
decoded, type-checked values.

**Values** are what `DecodeValue` returns: `json.Number`, `bool`, `string`,
`[]any` or `map[string]any`. Numbers stay `json.Number` so `int` and `float`
can be told apart, and so a value written back keeps its digits (`2.50`).
`Type.Check(v, "input")` returns a `*ValueError` whose `Path` points into the
value (`input.orders[1].id`). `CanonicalValue` renders a value compactly with
floats normalised; it is the key for contradiction detection.

Problems are typed errors where callers need to tell them apart:
`PairError` (a file without a partner), `ContradictionError`, `ValueError`.

`dataio.Record` is one example from a source, with a `Source` label such as
`people.csv:3`. Readers return a `Batch` of records plus `Problems` (fail the
import) and `Warnings` (do not).

## Data flow

### Import

`synth data import people.csv --holdout 0.25`:

1. `cli.runDataImport` checks the flag combination and loads the project.
2. A reader turns the source into a `Batch`: `ReadDirs`, `ReadJSONL` or
   `ReadCSV` (which needs the spec to map columns to parameters). Readers
   collect every problem instead of stopping at the first.
3. `dataio.MakePlan` gives each record a split (`HoldoutMask`, a seeded
   shuffle that gives the same result on every Go version and OS) and a
   name, and type-checks it against the spec. It reports name clashes and
   existing examples. It writes nothing.
4. If the batch or the plan has problems, they are sorted by source and
   line, logged, and the command fails with "nothing was written".
5. Otherwise `--dry-run` prints the plan, or `dataio.Apply` writes each
   example with `Project.WriteExample`.

`WriteExample` type-checks again, writes each file atomically (temp file and
rename) and removes the input file if writing the output fails. A failure in
the middle of `Apply` (disk full) can leave earlier examples written; the
error says how many.

Import does not check for contradictions; `validate` does.

### Validation

`Project.Validate` returns every problem in the project:

1. For each split, `Examples` lists `in/` and `out/` and pairs files by
   name; unpaired files become `PairError`s.
2. `LoadExample` reads both files and runs `Type.Check`.
3. Each valid input's `CanonicalValue` goes into a map; a second example with
   the same input and a different output becomes a `ContradictionError`.
   The map spans both splits.

`synth data validate` prints the problems; `synth config set` runs
`Validate` before and after a type change and reports only the new
problems.

### Changing the spec

`cli.changeSpec` loads the project, applies one `Spec.Set/Unset/Move`, runs
`Spec.Validate`, re-checks examples if the key is a type, and saves with
`SaveKeepingComments`, which encodes the new spec and copies comments and
flow/block style from the old YAML onto the keys that still exist.

## Errors, logging and output

- Commands return errors from `RunE`; fang prints them in an error box and
  the process exits with status 1. Error strings are lower case, without
  trailing punctuation (fang adds a period), and name the file they are
  about.
- fang reflows the error text, so multi-line detail (a list of problems)
  goes to the log first and the returned error is a one-line summary. See
  `data import` and `config set --strict`.
- Logs use `charm.land/log/v2`, set up in the root's `PersistentPreRunE`
  from `--log-level` and `--log-format`, and always go to stderr.
- Results go to `cmd.OutOrStdout()` so they can be piped and tested.
- Paths in output are shortened to be relative to the working directory
  when they are inside it (`displayPath`, `displayErr`).

## Tests

Each package has unit tests next to the code. Command tests in
`internal/cli` build the tree with `NewRootCmd`, call `SetArgs`, `SetIn` and
`SetOut`, and run `Execute` without fang (see `runCLI` in `data_test.go`).
The wizard is tested in accessible mode, which reads plain lines from stdin.

`internal/clidocs` has `TestCLIReferenceUpToDate`, which regenerates the
CLI reference into a temporary directory and fails with
"CLI reference is stale; run `just docs`" if it differs from `docs/cli`.
It runs with `go test ./...`, so `just test` and `just check` include it;
`just docs-check` runs it alone.

## The CLI reference

`docs/cli/` is generated, one page per command, by `internal/clidocs`
using `github.com/spf13/cobra/doc` on the tree from `cli.NewRootCmd`.
`cmd/gendocs` is a separate program rather than a hidden `synth gen-docs`
command, so the shipped binary and its command tree carry no documentation
tooling. The generator:

- sets `DisableAutoGenTag`, so pages carry no date;
- links pages by bare file name (`synth_data_add.md`), which works on GitHub;
- deletes old pages first, so removed commands disappear;
- converts the help text to Markdown: runs of indented lines become code
  blocks, and `<`, `>` and `*` outside backticks are escaped, so
  `data/<split>/in` does not turn into an HTML tag;
- adds aliases (`synth data ls`), which cobra's generator leaves out.

fang's `--version` flag and `man` command, and cobra's `completion` and
`help` commands, are added at execute time, so they are not in the
reference.

When writing help text: put commands, paths and keys in backticks, indent
tables and flag forms by two spaces (they become code blocks), and give
every command a `Long` and an `Example`. Quote example arguments with
single quotes so they work in PowerShell and sh.

## Adding a command

1. Write the logic in `internal/project`, `internal/dataio` or a new
   package, with unit tests. Keep `internal/cli` to flags and printing.
2. Add a `newXxxCmd()` in `internal/cli` and attach it in `NewRootCmd` or
   its parent command. Use `loadProject(cmd)` if it works on a project.
   Print results to `cmd.OutOrStdout()`, log to `log`, return errors.
3. Give it `Short`, `Long` and `Example`. These are the documentation: they
   become `docs/cli/synth_<path>.md`.
4. Add a command test using `NewRootCmd` and `SetArgs`.
5. Run `just docs` and commit the changed pages in `docs/cli/`.
6. Update the hand-written pages it affects: `getting-started.md` if it is
   part of the normal workflow, `project-format.md` if it changes the
   project format, this page if it adds a package, and the root
   `README.md` if it changes what works today.
7. Run `just check`.

## Milestones

How the code maps onto the milestones in [spec.md](../spec.md#milestones):

| Milestone | State | Where |
| --- | --- | --- |
| Project and data tooling (not a spec milestone) | done | `init`, `config`, `data` commands; `internal/project`, `internal/dataio` |
| M1: interpreter (types, ~20 operations, trees, trace, scoring) | next | `internal/synth` (empty). `project.Type` covers the value types the spec lists. |
| M2: bottom-up enumerator | planned | `internal/synth`; `synth solve` becomes real |
| M3: field-to-field decomposition for records | planned | |
| M4: Go back end | planned | uses `options.targets` |
| M5: repair loop | planned | |
| M6: learned ranker | planned | |
| M7: Rust and C back ends | planned | |

What exists today feeds M1 and M2: a typed spec, type-checked examples, a
train/holdout split and contradiction checks. Open points for the next
steps:

- `synth solve` takes an examples file argument and a `--timeout` string;
  both are placeholders. It should load the current project like the `data`
  commands do.
- `options.float_tolerance` is validated and stored but nothing reads it
  yet. Scoring (M1) should use it.
- The spec asks for a fixed benchmark set of 30 to 50 problems during M1.
  Projects in this format are the natural way to store them.
