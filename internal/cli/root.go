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
			"for a program that maps every example input to its expected output.",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return setupLogger(f)
		},
	}

	cmd.PersistentFlags().StringVar(&f.logLevel, "log-level", "info", "log level: debug, info, warn, error")
	cmd.PersistentFlags().StringVar(&f.logFormat, "log-format", "text", "log format: text, json, logfmt")

	cmd.AddCommand(newSolveCmd())
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
