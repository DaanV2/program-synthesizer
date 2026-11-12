// Package logging configures the application-wide charm logger.
package logging

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/log/v2"
)

// Options controls how the default logger is configured.
type Options struct {
	Level  string
	Format string
}

// Setup configures the default logger based on opts and writes to stderr.
func Setup(opts Options) error {
	return SetupWriter(os.Stderr, opts)
}

// SetupWriter configures the default logger based on opts and writes to w.
func SetupWriter(w io.Writer, opts Options) error {
	level, err := log.ParseLevel(opts.Level)
	if err != nil {
		return fmt.Errorf("invalid log level %q: %w", opts.Level, err)
	}

	formatter, err := parseFormat(opts.Format)
	if err != nil {
		return err
	}

	logger := log.NewWithOptions(w, log.Options{
		Level:           level,
		Formatter:       formatter,
		ReportTimestamp: level <= log.DebugLevel,
		ReportCaller:    level <= log.DebugLevel,
	})
	log.SetDefault(logger)

	return nil
}

func parseFormat(format string) (log.Formatter, error) {
	switch strings.ToLower(format) {
	case "", "text":
		return log.TextFormatter, nil
	case "json":
		return log.JSONFormatter, nil
	case "logfmt":
		return log.LogfmtFormatter, nil
	default:
		return 0, fmt.Errorf("invalid log format %q (expected text, json or logfmt)", format)
	}
}
