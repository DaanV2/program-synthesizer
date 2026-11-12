package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/DaanV2/program-synthesizer/internal/project"
)

const configKeysHelp = `Keys:
  name                      project name
  description               free text
  input                     all parameters at once, as a record: '{x: int, y: int}'
  input.<param>             one parameter's type; set adds it at the end if missing
  input.<a>.<b>             a field of a record parameter (missing records are created)
  input.<a>[].<b>           a field of the records in a list-of-records parameter
  output                    the whole output type: 'int', 'list<int>', '{fullName: string}'
  output.<field>            a field of a record output (same path rules as input)
  options.float_tolerance   number >= 0 used when comparing float outputs
  options.targets           comma-separated back ends: go, rust, c

Types use spec.yaml syntax on one line: int, float, bool, string, list<T>,
[T], and records as {name: type, ...}. Quote them in the shell: single quotes
work in both PowerShell and sh.`

const configSaveNote = `Saving rewrites spec.yaml. Comments and flow/block style are carried over
for keys that still exist; comments on removed keys and blank lines are lost.
A project with spec.json is rewritten as JSON. Nothing is printed on success.`

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show or change project settings in spec.yaml",
		Long: "Read and change single settings of the project spec without an editor.\n\n" +
			configKeysHelp + "\n\n" + configSaveNote,
		Example: `  synth config list
  synth config get input.age
  synth config set description "Join first and last name"
  synth config set input.tags 'list<string>'
  synth config set output '{fullName: string, isAdult: bool}'
  synth config set options.targets 'go,rust'
  synth config unset input.tags
  synth config move input.age --before first`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newConfigListCmd(), newConfigGetCmd(), newConfigSetCmd(), newConfigUnsetCmd(), newConfigMoveCmd())

	return cmd
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Print every setting as key = value",
		Long: "Print every setting that has a value, one per line as `key = value`, in spec order.\n" +
			"Input parameters are listed one per line; so are output fields when the output is a record.",
		Example: "  synth config list\n  synth -C path/to/project config list",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := loadProject(cmd)
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			for _, kv := range p.Spec.List() {
				fmt.Fprintf(w, "%s = %s\n", kv.Key, kv.Value)
			}

			return nil
		},
	}
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Print the value of one setting",
		Long: "Print the value of one setting, in the form `config set` accepts.\n" +
			"Fails if an optional setting has no value.\n\n" + configKeysHelp,
		Example: `  synth config get name
  synth config get input
  synth config get input.address.city
  synth config get input.orders[].total`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeKeys(false),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := loadProject(cmd)
			if err != nil {
				return err
			}
			v, err := p.Spec.Get(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), v)

			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set or add one setting",
		Long: "Set one setting. Setting a parameter or record field that does not exist adds it\n" +
			"at the end of its record.\n\n" +
			"After a type change the existing examples are checked against the new spec. Examples\n" +
			"that no longer conform are reported as a warning; with --strict the change is refused.\n\n" +
			configKeysHelp + "\n\n" + configSaveNote,
		Example: `  synth config set name full-name
  synth config set input.age int
  synth config set input.address '{city: string, zip: string}'
  synth config set input.address.country string
  synth config set input.orders '[{id: int, total: float}]'
  synth config set output 'list<int>'
  synth config set output.isAdult bool --strict
  synth config set options.float_tolerance 1e-6`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeKeys(true),
		RunE: func(cmd *cobra.Command, args []string) error {
			return changeSpec(cmd, args[0], strict, func(s *project.Spec) error {
				return s.Set(args[0], args[1])
			})
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "refuse the change if existing examples would no longer conform")

	return cmd
}

func newConfigUnsetCmd() *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove one setting",
		Long: "Remove an optional setting (description, options.*) or a parameter or record field.\n" +
			"name, input and output themselves are required and cannot be removed; neither can the\n" +
			"last field of a record. Unsetting an optional setting that has no value does nothing.\n\n" +
			configSaveNote,
		Example: `  synth config unset description
  synth config unset options.float_tolerance
  synth config unset input.tags
  synth config unset output.isAdult --strict`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeKeys(false),
		RunE: func(cmd *cobra.Command, args []string) error {
			return changeSpec(cmd, args[0], strict, func(s *project.Spec) error {
				return s.Unset(args[0])
			})
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "refuse the change if existing examples would no longer conform")

	return cmd
}

func newConfigMoveCmd() *cobra.Command {
	var before, after string
	cmd := &cobra.Command{
		Use:   "move <key> --before|--after <field>",
		Short: "Reorder a parameter or record field",
		Long: "Move a parameter or record field before or after one of its siblings. The order of\n" +
			"input parameters is the order of the generated function's arguments. Example files\n" +
			"are JSON objects, so reordering never breaks them.\n\n" + configSaveNote,
		Example: `  synth config move input.age --before first
  synth config move input.address.zip --after city
  synth config move output.isAdult --before fullName`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeKeys(false),
		RunE: func(cmd *cobra.Command, args []string) error {
			if (before == "") == (after == "") {
				return errors.New("give exactly one of --before or --after")
			}

			return changeSpec(cmd, args[0], false, func(s *project.Spec) error {
				if before != "" {
					return s.Move(args[0], before, false)
				}

				return s.Move(args[0], after, true)
			})
		},
	}
	cmd.Flags().StringVar(&before, "before", "", "sibling `field` to move in front of")
	cmd.Flags().StringVar(&after, "after", "", "sibling `field` to move behind")

	return cmd
}

// maxShownProblems limits how many example problems a warning lists.
const maxShownProblems = 5

// changeSpec loads the project, applies change, checks examples when the key
// touches input/output types, and saves.
func changeSpec(cmd *cobra.Command, key string, strict bool, change func(*project.Spec) error) error {
	p, err := loadProject(cmd)
	if err != nil {
		return err
	}
	k, err := project.ParseKey(key)
	if err != nil {
		return err
	}
	var before map[string]bool
	if k.IsType() {
		before = problemSet(p.Validate())
	}
	if err := change(p.Spec); err != nil {
		return err
	}
	if err := p.Spec.Validate(); err != nil {
		return err
	}
	if k.IsType() {
		var added []string
		for _, e := range p.Validate() {
			if !before[e.Error()] {
				added = append(added, e.Error())
			}
		}
		if len(added) > 0 {
			msg := fmt.Sprintf("%d example problem(s) with the new spec:%s", len(added), problemList(p, added))
			if strict {
				// fang reflows error text, so the list goes to the log.
				log.Error(msg)

				return fmt.Errorf("not changed: %d example(s) would no longer conform (--strict)", len(added))
			}
			log.Warn(msg + "\nfix the data or change the spec back")
		}
	}
	if err := p.SaveKeepingComments(); err != nil {
		return err
	}
	log.Debug("saved", "spec", p.SpecPath, "key", key)

	return nil
}

// problemList formats up to maxShownProblems problems, one per line, with
// paths shortened to be relative to the project directory.
func problemList(p *project.Project, problems []string) string {
	var b strings.Builder
	prefix := p.Dir + string(filepath.Separator)
	for _, s := range problems[:min(len(problems), maxShownProblems)] {
		b.WriteString("\n  " + strings.ReplaceAll(s, prefix, ""))
	}
	if len(problems) > maxShownProblems {
		fmt.Fprintf(&b, "\n  ... and %d more", len(problems)-maxShownProblems)
	}

	return b.String()
}

func problemSet(errs []error) map[string]bool {
	m := make(map[string]bool, len(errs))
	for _, e := range errs {
		m[e.Error()] = true
	}

	return m
}

// staticKeys are offered by completion even when they have no value yet.
var staticKeys = []string{project.KeyName, project.KeyDescription, project.KeyInput, project.KeyOutput, project.KeyFloatTolerance, project.KeyTargets}

// completeKeys completes the key argument from the current project and, with
// values, the value argument of type keys and options.targets.
func completeKeys(values bool) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		switch {
		case len(args) == 0:
			keys := slices.Clone(staticKeys)
			if p, err := loadProject(cmd); err == nil {
				for _, kv := range p.Spec.List() {
					if !slices.Contains(keys, kv.Key) {
						keys = append(keys, kv.Key)
					}
				}
			}

			return keys, cobra.ShellCompDirectiveNoFileComp
		case len(args) == 1 && values:
			if args[0] == project.KeyTargets {
				return project.KnownTargets, cobra.ShellCompDirectiveNoFileComp
			}
			if k, err := project.ParseKey(args[0]); err == nil && k.IsType() {
				return basicTypes, cobra.ShellCompDirectiveNoFileComp
			}
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// basicTypes are the types offered by completion and the init wizard.
var basicTypes = []string{"int", "float", "bool", "string", "list<int>", "list<float>", "list<string>"}
