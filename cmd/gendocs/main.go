// Command gendocs writes the Markdown CLI reference (default: docs/cli)
// from synth's cobra command tree. Run it with `just docs`.
//
// It is a separate program rather than a hidden `synth` subcommand so the
// shipped binary and its command tree carry no documentation tooling.
package main

import (
	"fmt"
	"os"

	"synth/internal/clidocs"
)

func main() {
	dir := "docs/cli"
	switch len(os.Args) {
	case 1:
	case 2:
		dir = os.Args[1]
	default:
		fmt.Fprintln(os.Stderr, "usage: gendocs [dir]")
		os.Exit(2)
	}
	if err := clidocs.Generate(dir); err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
}
