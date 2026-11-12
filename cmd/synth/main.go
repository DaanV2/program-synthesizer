// Command synth synthesises programs from input/output examples.
package main

import (
	"context"
	"io"
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
		fang.WithErrorHandler(errorHandler),
	)
	if err != nil {
		os.Exit(1)
	}
}

// errorHandler is fang's default, minus the title-casing of the message:
// errors often start with a file path or flag, which must stay as written.
func errorHandler(w io.Writer, styles fang.Styles, err error) {
	styles.ErrorText = styles.ErrorText.UnsetTransform()
	fang.DefaultErrorHandler(w, styles, err)
}
