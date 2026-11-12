set windows-shell := ["pwsh", "-NoLogo", "-NoProfile", "-Command"]

binary := "synth"
version := if os_family() == "windows" { `git describe --tags --always --dirty 2>$null || echo dev` } else { `git describe --tags --always --dirty 2>/dev/null || echo dev` }
commit := if os_family() == "windows" { `git rev-parse --short HEAD 2>$null || echo ""` } else { `git rev-parse --short HEAD 2>/dev/null || echo ""` }
ldflags := "-s -w -X github.com/DaanV2/program-synthesizer/infrastructure/version.Version=" + version + " -X github.com/DaanV2/program-synthesizer/infrastructure/version.Commit=" + commit
bin := "bin/" + binary + if os_family() == "windows" { ".exe" } else { "" }

# List available recipes
default:
    @just --list

# Install the development tools (keep golangci-lint in sync with .github/workflows/ci.yml)
[group("setup")]
onboard:
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
    go install github.com/goreleaser/goreleaser/v2@latest

# Build the binary into ./bin
[group("build")]
build:
    go build -trimpath -ldflags "{{ ldflags }}" -o {{ bin }} .

# Run the tool with the given arguments, e.g. `just run data validate`
[group("build")]
run *args:
    go run . {{ args }}

# Remove build artifacts
[group("build")]
clean:
    go clean
    {{ if os_family() == "windows" { "'bin', 'dist', 'coverage.out' | Where-Object { Test-Path $_ } | Remove-Item -Recurse -Force" } else { "rm -rf bin dist coverage.out" } }}

# Run all tests (includes the docs/cli staleness check); extra flags pass through
[group("test")]
test *args:
    go test ./... {{ args }}

# Run all tests with the race detector (requires cgo / a C toolchain)
[group("test")]
[env("CGO_ENABLED", "1")]
test-race:
    go test -race ./...

# Run tests with coverage report
[group("test")]
cover:
    go test -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out

# Run benchmarks
[group("test")]
bench *args:
    go test -run '^$' -bench . -benchmem ./... {{ args }}

# Run golangci-lint
[group("lint")]
lint:
    golangci-lint run

# Run golangci-lint and apply fixes
[group("lint")]
lint-fix:
    golangci-lint run --fix

# Format code
[group("lint")]
fmt:
    golangci-lint fmt

# Tidy and verify go modules
[group("lint")]
tidy:
    go mod tidy
    go mod verify

# Regenerate the CLI reference in docs/cli from the cobra command tree
[group("docs")]
docs:
    go run ./tools/gendocs docs/cli

# Fail if docs/cli is out of date (also part of `just test`)
[group("docs")]
docs-check:
    go test ./internal/clidocs -run TestCLIReferenceUpToDate

# Quick pre-commit check: lint (includes formatting) and tests
[group("ci")]
check: lint test

# Run everything CI runs
[group("ci")]
ci: tidy fmt lint test build

# Build a local snapshot release with goreleaser
[group("release")]
snapshot:
    goreleaser release --snapshot --clean
