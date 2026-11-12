// Package version holds build information injected at link time.
package version

// These values are overridden via -ldflags at build time (see .justfile and .goreleaser.yaml).
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)
