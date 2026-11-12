# synth

A hobby brute-force program synthesizer. You give it input/output examples;
it searches typed syntax trees built from a fixed library of operations for
a program that maps every input to its output, and compiles the result to
Go, Rust or C. The design is in [spec.md](spec.md).

## Status

Works today: creating a project (`synth init`), adding, importing, listing
and validating examples (`synth data`), and editing the spec
(`synth config`).

Not yet: the search itself. `synth solve` exists as a placeholder and
fails. Code generation comes after it. See
[docs/architecture.md](docs/architecture.md#milestones).

## Quick start

Needs Go 1.27 or newer and [just](https://github.com/casey/just)
(`winget install Casey.Just`).

```powershell
go install .
synth init fullname --yes --input 'first=string,last=string' --output string
cd fullname
Set-Content in.json '{"first": "Ada", "last": "Lovelace"}'
Set-Content out.json '"Ada Lovelace"'
synth data add --input in.json --output out.json
synth data list
synth data validate
```

[docs/getting-started.md](docs/getting-started.md) walks through a complete
example, including CSV import and the holdout split.

## Documentation

- [docs/getting-started.md](docs/getting-started.md): install and a worked example
- [docs/project-format.md](docs/project-format.md): spec.yaml, types, example files, validation
- [docs/architecture.md](docs/architecture.md): code layout, for contributors
- [docs/cli/synth.md](docs/cli/synth.md): command reference (generated)

## Development

```text
just          # list recipes
just onboard  # install golangci-lint and goreleaser
just build    # build bin/synth (with version info)
just test     # run tests (including the docs/cli staleness check)
just lint     # golangci-lint
just docs     # regenerate docs/cli after changing commands or help text
just check    # lint + tests: run before committing
just ci       # everything CI runs: tidy, fmt, lint, test, build
```

CI runs lint and tests on Linux, macOS and Windows for every push and PR.
To release, publish the draft release on GitHub; GoReleaser then builds and
attaches the binaries (see [docs/architecture.md](docs/architecture.md#tooling)).

[CLAUDE.md](CLAUDE.md) has the conventions; the main one is that a change
is not done until its documentation is.
