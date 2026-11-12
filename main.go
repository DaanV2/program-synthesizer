// Command synth synthesises programs from input/output examples.
package main

import (
	"context"
	"os"

	"github.com/DaanV2/program-synthesizer/cmd"
)

func main() {
	if err := cmd.Execute(context.Background()); err != nil {
		os.Exit(1)
	}
}
