# synth

Hobby brute-force program synthesizer: give it input/output examples, it searches
a typed syntax tree built from a fixed operation library and compiles the result
to Go, Rust or C. Design: [spec.md](spec.md).

## Setup

- [Go](https://go.dev/dl/) 1.25.8 or newer (required by charm.land/log/v2)
- [just](https://github.com/casey/just): `winget install Casey.Just`

## Layout

```
cmd/synth/       entry point (fang)
internal/cli/    cobra commands, logger setup
internal/synth/  core: types, operations, trees, interpreter, search
```

## Common commands

```
just          # list recipes
just test     # run tests
just check    # gofmt + go vet + tests
just run <args>
```
