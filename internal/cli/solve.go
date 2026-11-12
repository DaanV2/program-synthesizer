package cli

import (
	"errors"

	"charm.land/log/v2"
	"github.com/spf13/cobra"
)

func newSolveCmd() *cobra.Command {
	var timeout string

	cmd := &cobra.Command{
		Use:   "solve <examples-file>",
		Short: "Search for a program that fits the given examples",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("solving", "examples", args[0], "timeout", timeout)
			// TODO(M1/M2): load examples, run search, print program.
			return errors.New("solve: not implemented yet")
		},
	}

	cmd.Flags().StringVar(&timeout, "timeout", "30s", "search time budget")
	return cmd
}
