// Package cli defines the synth command tree.
package cli

import (
	"fmt"
	"os"
	"time"

	"charm.land/log/v2"
	"github.com/spf13/cobra"
)

type rootFlags struct {
	logLevel  string
	logFormat string
}

// NewRootCmd builds the root command with all subcommands attached.
func NewRootCmd() *cobra.Command {
	var f rootFlags

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
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return setupLogger(f)
		},
	}

	cmd.PersistentFlags().StringVar(&f.logLevel, "log-level", "info", "log level: debug, info, warn, error")
	cmd.PersistentFlags().StringVar(&f.logFormat, "log-format", "text", "log format: text, json, logfmt")
	cmd.PersistentFlags().StringP(projectFlag, "C", ".", "project directory; spec.yaml or spec.json is looked for here, then in parent directories")

	cmd.AddCommand(
		newInitCmd(),   // init.go   — interactive project wizard
		newConfigCmd(), // config.go — get/set/list/unset spec settings
		newDataCmd(),   // data.go   — add/import/list/validate examples
		newSolveCmd(),
	)
	return cmd
}

func setupLogger(f rootFlags) error {
	level, err := log.ParseLevel(f.logLevel)
	if err != nil {
		return fmt.Errorf("invalid --log-level %q: %w", f.logLevel, err)
	}

	var formatter log.Formatter
	switch f.logFormat {
	case "text":
		formatter = log.TextFormatter
	case "json":
		formatter = log.JSONFormatter
	case "logfmt":
		formatter = log.LogfmtFormatter
	default:
		return fmt.Errorf("invalid --log-format %q: want text, json or logfmt", f.logFormat)
	}

	logger := log.NewWithOptions(os.Stderr, log.Options{
		Level:           level,
		Formatter:       formatter,
		ReportTimestamp: true,
		TimeFormat:      time.TimeOnly,
	})
	log.SetDefault(logger)
	return nil
}
