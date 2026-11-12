package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"synth/internal/project"
)

type initFlags struct {
	name           string
	description    string
	inputs         []string
	output         string
	targets        string
	floatTolerance float64
	yes            bool
}

func newInitCmd() *cobra.Command {
	var f initFlags
	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Create a new project with an interactive wizard",
		Long: `Create a project: spec.yaml plus empty data/train and data/holdout directories.

dir defaults to --project (the current directory unless -C is given) and is
created if missing. init refuses to run where a spec.yaml or spec.json exists.

In a terminal, init asks for the name, description, input parameters, output
type and options, shows the resulting spec.yaml and asks before writing it.
Values given as flags are used as defaults, and --input/--output skip those
steps. Set ACCESSIBLE=1 for a line-by-line wizard that suits screen readers.

With --yes, or when stdin is not a terminal (scripts, CI), no questions are
asked and --input and --output are required.

Writing --input and --output:
  name=type pairs, comma separated:   --input 'first=string,last=string,age=int'
  a YAML flow mapping:                --input '{first: string, tags: list<string>}'
  repeated flags are concatenated:    --input first=string --input age=int
Types with commas inside brackets are fine in pairs: 'pos={x: int, y: int},n=int'.
--output takes a single type ('int', 'list<string>', '[{id: int}]') or a record
in either of the forms above.`,
		Example: `  # Interactive wizard in a new directory
  synth init fullname

  # Non-interactive (quote types: < > { } [ ] are special in PowerShell and sh)
  synth init fullname --yes --name full-name --input 'first=string,last=string,age=int' --output 'fullName=string,isAdult=bool' --targets go,rust

  # Records and lists of records
  synth init orders -y --input '{customer: {name: string}, orders: [{id: int, total: float}]}' --output float --float-tolerance 1e-9`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := cmd.Flags().GetString(projectFlag)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				dir = args[0]
			}
			interactive := !f.yes && isTerminal(cmd.InOrStdin()) && isTerminal(cmd.OutOrStdout())
			return runInit(cmd, dir, f, interactive)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&f.name, "name", "", "project name (default: the directory name)")
	fl.StringVar(&f.description, "description", "", "project description")
	fl.StringArrayVar(&f.inputs, "input", nil, "input parameters: 'name=type,...' or '{name: type, ...}' (repeatable)")
	fl.StringVar(&f.output, "output", "", "output type, or a record as 'name=type,...' / '{name: type, ...}'")
	fl.StringVar(&f.targets, "targets", "", "code generation targets, comma separated: go, rust, c (default go)")
	fl.Float64Var(&f.floatTolerance, "float-tolerance", 0, "tolerance when comparing float outputs (default: unset)")
	fl.BoolVarP(&f.yes, "yes", "y", false, "do not ask anything; use the flags (requires --input and --output)")
	return cmd
}

// isTerminal reports whether r or w is an *os.File attached to a terminal.
func isTerminal(v any) bool {
	f, ok := v.(*os.File)
	return ok && term.IsTerminal(f.Fd())
}

func runInit(cmd *cobra.Command, dir string, f initFlags, interactive bool) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for _, name := range []string{project.SpecYAML, project.SpecJSON} {
		if _, err := os.Stat(filepath.Join(abs, name)); err == nil {
			return fmt.Errorf("a project already exists here: %s (use `synth config` to change it)", filepath.Join(abs, name))
		}
	}

	d, err := draftFromFlags(cmd, f, abs)
	if err != nil {
		return err
	}

	if interactive {
		w := &initWizard{ctx: cmd.Context(), in: cmd.InOrStdin(), out: cmd.OutOrStdout(), accessible: os.Getenv("ACCESSIBLE") != ""}
		ok, err := w.fill(d, abs)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("init cancelled; nothing was written")
		}
	} else {
		var missing []string
		if len(d.Params) == 0 {
			missing = append(missing, "--input")
		}
		if d.Output == nil {
			missing = append(missing, "--output")
		}
		if len(missing) > 0 {
			why := "with --yes"
			if !f.yes {
				why = "when stdin is not a terminal"
			}
			return fmt.Errorf("missing %s: required %s (see `synth init --help`)", strings.Join(missing, " and "), why)
		}
		if d.Name == "" {
			return errors.New("missing --name: the directory name cannot be used as a default")
		}
		if len(d.Targets) == 0 {
			d.Targets = []string{"go"}
		}
	}

	spec, err := d.Build()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	p, err := project.Create(abs, spec)
	if err != nil {
		return err
	}
	printNextSteps(cmd.OutOrStdout(), p, dir)
	return nil
}

// draftFromFlags fills a draft from the flags; unset flags stay empty so the
// wizard can ask for them.
func draftFromFlags(cmd *cobra.Command, f initFlags, absDir string) (*specDraft, error) {
	d := &specDraft{Name: f.name, Description: f.description}
	if d.Name == "" {
		d.Name = defaultProjectName(absDir)
	}
	var err error
	if len(f.inputs) > 0 {
		if d.Params, err = parseInputFlags(f.inputs); err != nil {
			return nil, err
		}
	}
	if f.output != "" {
		if d.Output, err = parseOutputFlag(f.output); err != nil {
			return nil, err
		}
	}
	if f.targets != "" {
		if d.Targets, err = project.ParseTargets(f.targets); err != nil {
			return nil, fmt.Errorf("invalid --targets: %w", err)
		}
	}
	if cmd.Flags().Changed("float-tolerance") {
		if f.floatTolerance < 0 {
			return nil, errors.New("invalid --float-tolerance: must be >= 0")
		}
		d.FloatTolerance = f.floatTolerance
	}
	return d, nil
}

func printNextSteps(w io.Writer, p *project.Project, dirArg string) {
	fmt.Fprintf(w, "Created project %q in %s\n\n", p.Spec.Name, p.Dir)
	fmt.Fprintln(w, "Next steps:")
	if wd, err := os.Getwd(); err != nil || filepath.Clean(wd) != filepath.Clean(p.Dir) {
		fmt.Fprintf(w, "  cd %s\n", quoteArg(dirArg))
	}
	fmt.Fprintln(w, "  synth data add ...          add examples (see `synth data --help`)")
	fmt.Fprintf(w, "                              or write JSON files to %s and %s\n",
		filepath.Join(project.DataDir, string(project.Train), project.InDir, "<name>.json"),
		filepath.Join(project.DataDir, string(project.Train), project.OutDir, "<name>.json"))
	fmt.Fprintln(w, "  synth config list           review or change the spec")
}

// quoteArg quotes a path for display when it contains spaces.
func quoteArg(s string) string {
	for _, r := range s {
		if r == ' ' || r == '\t' {
			return `"` + s + `"`
		}
	}
	return s
}
