package cmd

import (
	"errors"

	"charm.land/log/v2"
	"github.com/spf13/cobra"
)

func newSolveCmd() *cobra.Command {
	var timeout string

	cmd := &cobra.Command{
		Use:   "solve <examples-file>",
		Short: "Search for a program that fits the examples (not implemented yet)",
		Long: "Not implemented yet: the command exists as a placeholder and always fails.\n\n" +
			"It will run the search described in spec.md (milestones M1 and M2 first) and\n" +
			"print the program it finds. The argument and flags are placeholders and will\n" +
			"change; the plan is for solve to work on the current project, like the data\n" +
			"commands, instead of taking an examples file.",
		Example: "  synth solve examples.json --timeout 1m",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("solving", "examples", args[0], "timeout", timeout)
			// TODO(M1/M2): load examples, run search, print program.
			return errors.New("solve: not implemented yet")
		},
	}

	cmd.Flags().StringVar(&timeout, "timeout", "30s", "search time budget")

	return cmd
}
