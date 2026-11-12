// Command synth synthesises programs from input/output examples.
package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"

	"synth/internal/cli"
)

// Set at build time: go build -ldflags "-X main.version=... -X main.commit=..."
var (
	version = "dev"
	commit  = ""
)

func main() {
	err := fang.Execute(
		context.Background(),
		cli.NewRootCmd(),
		fang.WithVersion(version),
		fang.WithCommit(commit),
		fang.WithNotifySignal(os.Interrupt), // Ctrl+C cancels cmd.Context(), so long searches can stop cleanly
	)
	if err != nil {
		os.Exit(1)
	}
}
