package cmd

import (
	"github.com/spf13/cobra"

	"github.com/DaanV2/program-synthesizer/internal/project"
)

// projectFlag is the persistent --project/-C flag name.
const projectFlag = "project"

// loadProject finds and loads the project for a command: the directory given
// with --project, or the nearest parent of the working directory that has a
// spec file.
func loadProject(cmd *cobra.Command) (*project.Project, error) {
	start, err := cmd.Flags().GetString(projectFlag)
	if err != nil {
		return nil, err
	}
	dir, err := project.Find(start)
	if err != nil {
		return nil, err
	}

	return project.Load(dir)
}
