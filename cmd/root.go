// Package cmd defines the synth command tree.
package cmd

import (
	"context"
	"io"
	"os"

	"charm.land/fang/v2"
	"github.com/spf13/cobra"

	"github.com/DaanV2/program-synthesizer/infrastructure/logging"
	"github.com/DaanV2/program-synthesizer/infrastructure/version"
)

// NewRootCmd builds the root command with all subcommands attached.
func NewRootCmd() *cobra.Command {
	logOpts := logging.Options{}

	cmd := &cobra.Command{
		Use:   "synth",
		Short: "Synthesise programs from input/output examples",
		Long: "synth searches typed syntax trees built from a fixed operation library\n" +
			"for a program that maps every example input to its expected output.\n\n" +
			"A project is a directory with a spec.yaml (the input and output types) and\n" +
			"the examples under data/. Create one with `synth init`, add examples with\n" +
			"`synth data`, and adjust the spec with `synth config`. The search itself\n" +
			"(`synth solve`) is not implemented yet.\n\n" +
			"Commands that work on a project look for spec.yaml or spec.json in the\n" +
			"--project directory and then in its parents, so they work from any\n" +
			"subdirectory. Logs go to stderr; results go to stdout.",
		Example: `  synth init fullname --yes --input 'first=string,last=string' --output string
  synth -C fullname data add --input in.json --output out.json
  synth -C fullname data validate`,
		SilenceUsage: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return logging.Setup(logOpts)
		},
	}

	cmd.PersistentFlags().StringVar(&logOpts.Level, "log-level", "info", "log level: debug, info, warn, error, fatal")
	cmd.PersistentFlags().StringVar(&logOpts.Format, "log-format", "text", "log format: text, json, logfmt")
	cmd.PersistentFlags().StringP(projectFlag, "C", ".", "project directory; spec.yaml or spec.json is looked for here, then in parent directories")

	cmd.AddCommand(
		newInitCmd(),   // init.go   — interactive project wizard
		newConfigCmd(), // config.go — get/set/list/unset spec settings
		newDataCmd(),   // data.go   — add/import/list/validate examples
		newSolveCmd(),
	)

	return cmd
}

// Execute runs the root command with fang.
func Execute(ctx context.Context) error {
	opts := []fang.Option{
		fang.WithVersion(version.Version),
		fang.WithNotifySignal(os.Interrupt), // Ctrl+C cancels cmd.Context(), so long searches can stop cleanly
		fang.WithErrorHandler(errorHandler),
	}
	if version.Commit != "" {
		opts = append(opts, fang.WithCommit(version.Commit))
	}

	return fang.Execute(ctx, NewRootCmd(), opts...)
}

// errorHandler is fang's default, minus the title-casing of the message:
// errors often start with a file path or flag, which must stay as written.
//
//nolint:gocritic // hugeParam: signature is fixed by fang.ErrorHandler
func errorHandler(w io.Writer, styles fang.Styles, err error) {
	styles.ErrorText = styles.ErrorText.UnsetTransform()
	fang.DefaultErrorHandler(w, styles, err)
}
