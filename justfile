# Run `just` to list recipes.
set windows-shell := ["powershell.exe", "-NoLogo", "-NoProfile", "-Command"]

default:
    @just --list

# Build the CLI into bin/
build:
    go build -o bin/ ./cmd/synth

# Run the CLI, e.g. `just run examples.json`
run *ARGS:
    go run ./cmd/synth {{ARGS}}

# Run tests; extra flags pass through: `just test -run Enumerate -v`
test *ARGS:
    go test ./... {{ARGS}}

# Run benchmarks
bench *ARGS:
    go test -run '^$' -bench . -benchmem ./... {{ARGS}}

# Test coverage summary
cover:
    go test -coverprofile coverage.out ./...
    go tool cover -func coverage.out

# Format code
fmt:
    go fmt ./...

# Static checks
vet:
    go vet ./...

# Fail if any file is not gofmt-formatted
[unix]
fmt-check:
    test -z "$(gofmt -l .)" || (gofmt -l . && exit 1)

[windows]
fmt-check:
    $f = gofmt -l .; if ($f) { $f; exit 1 }

# Everything CI would check: formatting, vet, tests (the tests include docs-check)
check: fmt-check vet test

# Regenerate the CLI reference in docs/cli from the cobra command tree
docs:
    go run ./cmd/gendocs docs/cli

# Fail if docs/cli is out of date (also part of `just test` and `just check`)
docs-check:
    go test ./internal/clidocs -run TestCLIReferenceUpToDate

# Tidy go.mod / go.sum
tidy:
    go mod tidy

[unix]
clean:
    go clean
    rm -rf bin coverage.out

[windows]
clean:
    go clean
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue bin, coverage.out
