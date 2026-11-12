# synth — notes for Claude and other agents

Hobby brute-force program synthesizer: give it input/output examples, it
searches typed syntax trees built from a fixed operation library and compiles
the result to Go, Rust or C. The design is in `spec.md`; read it before working
on search, scoring or code generation.

## Documentation is part of every change

A feature is not done until it is documented. In the same commit as the code:

- **New or changed command, flag or behaviour** → update the matching page in
  `docs/` (and `README.md` if it is in the quick start), then run `just docs`
  to regenerate the CLI reference in `docs/cli/`. `just check` fails when the
  generated reference is stale (the test is `TestCLIReferenceUpToDate` in
  `internal/clidocs`; `just docs-check` runs it alone). The reference is
  built from the cobra help text, so write a good `Long` and `Example` for
  every command, and never edit `docs/cli/` by hand.
- **Change to the project format** (`spec.yaml`, type syntax, `data/` layout,
  validation rules) → update `docs/project-format.md`.
- **New package or moved responsibility** → update `docs/architecture.md`.
- **Design decision or deviation from `spec.md`** → note it in the relevant doc;
  update `spec.md` itself only when the plan changes, not for implementation detail.
- Examples in docs must actually run. If you change behaviour, re-check the
  examples that show it.

When you finish, say which docs you updated. If a change needs no doc update,
say why.

## Environment

- The user is on **Windows** (PowerShell, Windows Terminal). Use `filepath`,
  never assume `/`, and keep `.justfile` recipes working under PowerShell
  (use `[unix]` / `[windows]` recipe variants where shells differ).
- Go 1.27 or newer (`go.mod`), required by `charm.land/fang/v2`.
- The repo follows [go-tool-template](https://github.com/DaanV2/go-tool-template):
  same layout, `.justfile`, golangci-lint config, CI and release setup.
- **Charm libraries: v2 stack only.** Use `charm.land/log/v2`, `charm.land/huh/v2`,
  `charm.land/lipgloss/v2`, `github.com/charmbracelet/fang`. Do not add
  `github.com/charmbracelet/log`, `.../huh` (v0/v1) or `.../lipgloss` v1: they
  pull in an older `charmbracelet/x/ansi` that breaks the build.

## Layout

```
main.go           entry point: calls cmd.Execute
cmd/              cobra commands (init, config, data, solve) and fang setup; thin layer
infrastructure/   logging setup, version info (set via -ldflags)
tools/gendocs/    writes docs/cli from the command tree (`just docs`)
internal/clidocs/ CLI reference generator and its staleness test
internal/project/ spec.yaml, type system, data/ layout, validation
internal/dataio/  import sources (dirs, JSONL, CSV) and import planning
internal/synth/   core synthesis (M1+, mostly empty so far)
docs/             user and developer documentation; docs/cli is generated
```

## Conventions

- `just check` (golangci-lint, which includes formatting, plus tests and the
  docs/cli staleness check) must pass before committing; `just ci` runs
  everything CI does. Fix lint findings rather than adding `//nolint`; when
  a nolint is unavoidable it needs a specific linter and an explanation.
- White-box tests go in `*_internal_test.go`; tests that only use the
  exported API go in a `<pkg>_test` package.
- Error strings are lower case and wrap the file path so users can find the
  problem. fang renders errors; don't add trailing punctuation.
- Logs (`charm.land/log/v2`) go to stderr; command results go to
  `cmd.OutOrStdout()` so they can be piped.
- Keep logic out of `cmd/`: put it in a package with unit tests and
  keep the cobra layer to flag parsing and printing.
- Example files are written through `project.WriteExample` (type-checked,
  atomic, field order kept); don't write them by hand in code.
