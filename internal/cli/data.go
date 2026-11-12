package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"synth/internal/dataio"
	"synth/internal/project"
)

func newDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "data",
		Short: "Add, import, list and validate examples",
		Long: "Manage the input/output examples of a project.\n\n" +
			"Examples live in data/<split>/in/<name>.json and data/<split>/out/<name>.json,\n" +
			"where <split> is train (used for search) or holdout (used to catch overfitting).\n" +
			"An example is the pair of files with the same name.",
	}
	cmd.AddCommand(newDataAddCmd(), newDataImportCmd(), newDataListCmd(), newDataValidateCmd())
	return cmd
}

// ---------------------------------------------------------------- add

type dataAddFlags struct {
	inFile, outFile string
	inJSON, outJSON string
	split, name     string
	overwrite       bool
}

func newDataAddCmd() *cobra.Command {
	var f dataAddFlags
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add one example",
		Long: "Add one example to the project. The input and the output each come from a\n" +
			"JSON file (--input/--output, '-' reads stdin) or from inline JSON (--in/--out).\n" +
			"Both are type-checked against spec.yaml before anything is written.\n\n" +
			"The input is always an object with one key per input parameter.",
		Example: `  # inline JSON (bash, zsh)
  synth data add --in '{"x": 3}' --out '7'

  # inline JSON (PowerShell: escape inner quotes, or prefer files)
  synth data add --in '{\"x\": 3}' --out 7

  # from files, into the holdout split, with a chosen name
  synth data add --input in.json --output out.json --split holdout --name edge-case

  # input from stdin
  echo '{"x": 3}' | synth data add --input - --out 7`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runDataAdd(cmd, f) },
	}
	fl := cmd.Flags()
	fl.StringVar(&f.inFile, "input", "", "read the input from this JSON file ('-' for stdin)")
	fl.StringVar(&f.outFile, "output", "", "read the output from this JSON file ('-' for stdin)")
	fl.StringVar(&f.inJSON, "in", "", "the input as inline JSON")
	fl.StringVar(&f.outJSON, "out", "", "the output as inline JSON")
	fl.StringVar(&f.split, "split", string(project.Train), "split to add to: train or holdout")
	fl.StringVar(&f.name, "name", "", "example name (default: next free number, e.g. 004)")
	fl.BoolVar(&f.overwrite, "overwrite", false, "replace an existing example with the same name")
	cmd.MarkFlagsMutuallyExclusive("input", "in")
	cmd.MarkFlagsMutuallyExclusive("output", "out")
	cmd.MarkFlagsOneRequired("input", "in")
	cmd.MarkFlagsOneRequired("output", "out")
	return cmd
}

func runDataAdd(cmd *cobra.Command, f dataAddFlags) error {
	if f.inFile == "-" && f.outFile == "-" {
		return errors.New("only one of --input and --output can read from stdin ('-')")
	}
	sp, err := project.ParseSplit(f.split)
	if err != nil {
		return err
	}
	p, err := loadProject(cmd)
	if err != nil {
		return err
	}
	in, inLabel, err := readValueArg(cmd, f.inFile, f.inJSON, "--in")
	if err != nil {
		return err
	}
	out, outLabel, err := readValueArg(cmd, f.outFile, f.outJSON, "--out")
	if err != nil {
		return err
	}
	if err := p.Spec.Input.Check(in, "input"); err != nil {
		return fmt.Errorf("invalid input from %s: %w (spec input is %s)", inLabel, err, p.Spec.Input)
	}
	if err := p.Spec.Output.Check(out, "output"); err != nil {
		return fmt.Errorf("invalid output from %s: %w (spec output is %s)", outLabel, err, p.Spec.Output)
	}
	name := f.name
	if name == "" {
		if name, err = p.NextName(sp); err != nil {
			return err
		}
	}
	if !f.overwrite {
		for _, side := range []project.Side{project.Input, project.Output} {
			if path := p.ExamplePath(sp, name, side); fileExists(path) {
				return fmt.Errorf("example already exists: %s (use --overwrite to replace it, or pick another --name)", displayPath(path))
			}
		}
	}
	e, err := p.WriteExample(sp, name, in, out, f.overwrite)
	if err != nil {
		return err
	}
	log.Info("added example", "split", sp, "name", name)
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, displayPath(e.InputPath))
	fmt.Fprintln(w, displayPath(e.OutputPath))
	return nil
}

// readValueArg decodes a value from a file path ('-' = stdin) or inline
// JSON. It returns the value and a label for error messages.
func readValueArg(cmd *cobra.Command, file, inline, inlineFlag string) (any, string, error) {
	switch {
	case file == "-":
		v, err := project.DecodeValue(cmd.InOrStdin())
		if err != nil {
			return nil, "stdin", fmt.Errorf("reading stdin: %w", err)
		}
		return v, "stdin", nil
	case file != "":
		v, err := project.ReadValueFile(file)
		if err != nil {
			return nil, file, fmt.Errorf("reading %s: %w", file, err)
		}
		return v, file, nil
	default:
		v, err := project.DecodeValue(strings.NewReader(inline))
		if err != nil {
			return nil, inlineFlag, fmt.Errorf("invalid JSON in %s: %w", inlineFlag, err)
		}
		return v, inlineFlag, nil
	}
}

// ---------------------------------------------------------------- import

type dataImportFlags struct {
	inputs, outputs string
	format          string
	split           string
	holdout         float64
	seed            uint64
	overwrite       bool
	dryRun          bool
}

func newDataImportCmd() *cobra.Command {
	var f dataImportFlags
	cmd := &cobra.Command{
		Use:   "import [file]",
		Short: "Import many examples from directories, JSONL or CSV",
		Long: "Import many examples at once. The source is one of:\n\n" +
			"  --inputs DIR --outputs DIR\n" +
			"      Pairs *.json files with the same basename. The basename becomes the\n" +
			"      example name (if it is a valid name). Unpaired files are reported and skipped.\n\n" +
			"  FILE.jsonl (or .ndjson)\n" +
			"      One example per line: {\"input\": {...}, \"output\": ...}. Names are generated\n" +
			"      (next free number in the split).\n\n" +
			"  FILE.csv\n" +
			"      Only when every input parameter is a scalar (int, float, bool, string) and the\n" +
			"      output is a scalar or a record of scalars. The header names the parameters,\n" +
			"      plus 'out' for a scalar output or 'out.<field>' for each output field.\n" +
			"      Bools may be true/false or 1/0. String cells are taken verbatim.\n\n" +
			"Use '-' as FILE to read stdin (needs --format). Every example is checked first;\n" +
			"if any of them has a problem, all problems are printed and nothing is written.",
		Example: `  # two directories of matching files
  synth data import --inputs raw/in --outputs raw/out

  # JSONL, sending a reproducible 20% to the holdout split
  synth data import examples.jsonl --holdout 0.2 --seed 42

  # CSV for spec input {a: int, b: int} and output int:
  #   a,b,out
  #   1,2,3
  synth data import sums.csv --dry-run

  # from stdin
  cat examples.jsonl | synth data import - --format jsonl`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return runDataImport(cmd, args, f) },
	}
	fl := cmd.Flags()
	fl.StringVar(&f.inputs, "inputs", "", "directory of input *.json files (with --outputs)")
	fl.StringVar(&f.outputs, "outputs", "", "directory of output *.json files (with --inputs)")
	fl.StringVar(&f.format, "format", "", "file format: jsonl or csv (default: from the file extension)")
	fl.StringVar(&f.split, "split", string(project.Train), "split to import into: train or holdout")
	fl.Float64Var(&f.holdout, "holdout", 0, "send this fraction (0-1) of the examples to holdout, chosen at random")
	fl.Uint64Var(&f.seed, "seed", 1, "random seed for --holdout (same seed and data give the same split)")
	fl.BoolVar(&f.overwrite, "overwrite", false, "replace existing examples with the same name")
	fl.BoolVarP(&f.dryRun, "dry-run", "n", false, "check everything and show what would be written, but write nothing")
	cmd.MarkFlagsRequiredTogether("inputs", "outputs")
	return cmd
}

func runDataImport(cmd *cobra.Command, args []string, f dataImportFlags) error {
	sp, err := project.ParseSplit(f.split)
	if err != nil {
		return err
	}
	if f.holdout != 0 {
		if f.holdout < 0 || f.holdout > 1 {
			return fmt.Errorf("the --holdout fraction must be between 0 and 1, got %v", f.holdout)
		}
		if sp == project.Holdout {
			return errors.New("cannot combine --holdout with --split holdout (--holdout already sends a fraction to holdout)")
		}
	} else if cmd.Flags().Changed("seed") {
		return errors.New("the --seed flag only applies together with --holdout")
	}

	useDirs := f.inputs != "" || f.outputs != ""
	switch {
	case useDirs && len(args) > 0:
		return fmt.Errorf("give either a file (%s) or --inputs/--outputs, not both", args[0])
	case useDirs && f.format != "":
		return errors.New("the --format flag applies to a file source, not to --inputs/--outputs")
	case !useDirs && len(args) == 0:
		return errors.New("nothing to import: give a .jsonl or .csv file, or --inputs DIR --outputs DIR")
	}

	p, err := loadProject(cmd)
	if err != nil {
		return err
	}

	var batch *dataio.Batch
	if useDirs {
		batch, err = dataio.ReadDirs(f.inputs, f.outputs)
	} else {
		batch, err = readImportFile(cmd, p, args[0], f.format)
	}
	if err != nil {
		return err
	}
	for _, w := range batch.Warnings {
		log.Warn(w)
	}

	plan, err := dataio.MakePlan(p, batch.Records, dataio.Options{
		Split:           sp,
		HoldoutFraction: f.holdout,
		Seed:            f.seed,
		Overwrite:       f.overwrite,
	})
	if err != nil {
		return err
	}
	for _, w := range plan.Warnings {
		log.Warn(w)
	}
	problems := slices.Concat(batch.Problems, plan.Problems)
	if len(problems) > 0 {
		dataio.SortProblems(problems)
		for _, e := range problems {
			log.Error(e.Error())
		}
		return fmt.Errorf("found %d problem(s) in %d example(s); nothing was written",
			len(problems), dataio.CountSources(problems))
	}
	if len(plan.Examples) == 0 {
		return errors.New("no examples found to import")
	}

	w := cmd.OutOrStdout()
	if f.dryRun {
		for _, e := range plan.Examples {
			fmt.Fprintf(w, "%s -> %s/%s\n", e.Source, e.Split, e.Name)
		}
		fmt.Fprintf(w, "dry run: would import %s\n", formatCounts(plan.Counts()))
		return nil
	}
	if err := dataio.Apply(p, plan, f.overwrite); err != nil {
		return err
	}
	fmt.Fprintf(w, "imported %s\n", formatCounts(plan.Counts()))
	return nil
}

func readImportFile(cmd *cobra.Command, p *project.Project, path, format string) (*dataio.Batch, error) {
	if format == "" {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jsonl", ".ndjson":
			format = "jsonl"
		case ".csv":
			format = "csv"
		default:
			if path == "-" {
				return nil, errors.New("reading from stdin needs --format jsonl or --format csv")
			}
			return nil, fmt.Errorf("cannot tell the format of %s from its extension; use --format jsonl or --format csv", path)
		}
	}
	if format != "jsonl" && format != "csv" {
		return nil, fmt.Errorf("unknown --format %q (want jsonl or csv)", format)
	}

	var r io.Reader
	source := path
	if path == "-" {
		r, source = cmd.InOrStdin(), "stdin"
	} else {
		fh, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer fh.Close()
		r = fh
	}
	if format == "csv" {
		return dataio.ReadCSV(r, source, p.Spec.Input, p.Spec.Output)
	}
	return dataio.ReadJSONL(r, source)
}

func formatCounts(c map[project.Split]int) string {
	total := 0
	var parts []string
	for _, sp := range project.Splits {
		total += c[sp]
		parts = append(parts, fmt.Sprintf("%d %s", c[sp], sp))
	}
	return fmt.Sprintf("%d example(s): %s", total, strings.Join(parts, ", "))
}

// ---------------------------------------------------------------- list

func newDataListCmd() *cobra.Command {
	var split string
	var width int
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List examples with a preview of their values",
		Long: "List the examples of the project as a table with a one-line preview of each\n" +
			"input and output. Files without a partner are reported as warnings.\n" +
			"Values are not type-checked here; use `synth data validate` for that.",
		Example: `  synth data list
  synth data list --split holdout --width 80`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return runDataList(cmd, split, width) },
	}
	cmd.Flags().StringVar(&split, "split", "", "only list this split: train or holdout (default: both)")
	cmd.Flags().IntVar(&width, "width", 40, "truncate previews to this many characters (0 = no limit)")
	return cmd
}

func runDataList(cmd *cobra.Command, split string, width int) error {
	splits := project.Splits
	if split != "" {
		sp, err := project.ParseSplit(split)
		if err != nil {
			return err
		}
		splits = []project.Split{sp}
	}
	p, err := loadProject(cmd)
	if err != nil {
		return err
	}

	var rows [][]string
	for _, sp := range splits {
		exs, pairErrs := p.Examples(sp)
		for _, e := range pairErrs {
			log.Warn(displayErr(e))
		}
		for _, e := range exs {
			rows = append(rows, []string{
				string(sp), e.Name,
				previewFile(e.InputPath, p.Spec.Input, width),
				previewFile(e.OutputPath, p.Spec.Output, width),
			})
		}
	}
	w := cmd.OutOrStdout()
	if len(rows) == 0 {
		fmt.Fprintln(w, "no examples (add some with `synth data add` or `synth data import`)")
		return nil
	}
	header := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	cell := lipgloss.NewStyle().Padding(0, 1)
	t := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("SPLIT", "NAME", "INPUT", "OUTPUT").
		Rows(rows...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return header
			}
			return cell
		})
	_, err = lipgloss.Fprintln(w, t)
	return err
}

func previewFile(path string, t *project.Type, width int) string {
	v, err := project.ReadValueFile(path)
	if err != nil {
		log.Warn("cannot read example file", "err", err)
		return "(unreadable)"
	}
	return dataio.Preview(v, t, width)
}

// ---------------------------------------------------------------- validate

func newDataValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Check every example against the spec",
		Long: "Check that every example has both files, that every file is valid JSON and\n" +
			"that every value matches the types in spec.yaml. Prints each problem and exits\n" +
			"with an error if there are any.",
		Example: `  synth data validate
  synth -C path/to/project data validate`,
		Args: cobra.NoArgs,
		RunE: runDataValidate,
	}
}

func runDataValidate(cmd *cobra.Command, _ []string) error {
	p, err := loadProject(cmd)
	if err != nil {
		return err
	}
	w := cmd.OutOrStdout()
	problems := p.Validate()
	if len(problems) > 0 {
		for _, e := range problems {
			fmt.Fprintln(w, displayErr(e))
		}
		return fmt.Errorf("found %d problem(s)", len(problems))
	}
	counts := map[project.Split]int{}
	for _, sp := range project.Splits {
		exs, _ := p.Examples(sp)
		counts[sp] = len(exs)
	}
	fmt.Fprintf(w, "ok: %s\n", formatCounts(counts))
	switch {
	case counts[project.Train] == 0:
		log.Warn("no training examples yet")
	case counts[project.Holdout] == 0:
		log.Warn("no holdout examples: overfitting will go unnoticed (try `synth data import --holdout 0.2`)")
	}
	return nil
}

// ---------------------------------------------------------------- helpers

// displayPath shortens path to be relative to the working directory when it
// is inside it, so output stays readable.
func displayPath(path string) string {
	wd, err := os.Getwd()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(wd, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}

// displayErr shortens absolute paths at the start of an error message.
func displayErr(err error) string {
	msg := err.Error()
	path, rest, ok := strings.Cut(msg, ": ")
	if !ok || !filepath.IsAbs(path) {
		return msg
	}
	return displayPath(path) + ": " + rest
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
