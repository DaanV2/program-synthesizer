package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"charm.land/huh/v2"

	"synth/internal/project"
)

// customType is the type picker's option for typing a type expression.
const customType = "custom…"

// initWizard asks for the project settings with huh forms.
//
// In a terminal the whole wizard is one form (see init_tui.go): parameters
// live in a fixed number of slots whose groups hide until they are needed,
// so users can step back with shift+tab and no keystrokes are lost between
// separate bubbletea programs. huh's accessible (line-by-line) mode ignores
// hidden groups, so there the wizard runs as a sequence of small forms and
// branches in Go (fillSequential below).
type initWizard struct {
	ctx        context.Context
	in         io.Reader
	out        io.Writer
	accessible bool
}

func (w *initWizard) run(form *huh.Form) error {
	form = form.WithInput(w.in).WithOutput(w.out).WithAccessible(w.accessible).WithShowHelp(true)
	err := form.RunWithContext(w.ctx)
	cancelled := errors.New("init cancelled; nothing was written")
	if errors.Is(err, huh.ErrUserAborted) || errors.Is(err, context.Canceled) {
		return cancelled
	}
	if err != nil {
		return err
	}
	// Run also returns nil when the program ends without the form being
	// submitted, e.g. when the terminal hangs up; never take the defaults
	// then. (Accessible mode does not track the state.)
	if !w.accessible && form.State != huh.StateCompleted {
		return cancelled
	}
	return nil
}

// fill completes d interactively. Parts already set (Params, Output) are not
// asked again. It returns false if the user declined to write the result.
func (w *initWizard) fill(d *specDraft, dir string) (bool, error) {
	if w.accessible {
		return w.fillSequential(d, dir)
	}
	return w.fillForm(d, dir)
}

// fillSequential is fill as a sequence of small forms.
func (w *initWizard) fillSequential(d *specDraft, dir string) (bool, error) {
	// An empty answer keeps the default. (Accessible mode validates the raw
	// line before it applies the default, so the validator must allow it.)
	defName := d.Name
	if err := w.run(huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Project name").
			Value(&d.Name).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" && defName == "" {
					return errors.New("name is required")
				}
				return nil
			}),
		huh.NewInput().
			Title("Description").
			Description("Optional: what the synthesised function should do.").
			Value(&d.Description),
	).Title("New synth project in " + dir))); err != nil {
		return false, err
	}
	if strings.TrimSpace(d.Name) == "" {
		d.Name = defName
	}

	if len(d.Params) == 0 {
		params, err := w.fields("Input parameter", "parameter",
			"Parameters become the arguments of the generated function, in this order.")
		if err != nil {
			return false, err
		}
		d.Params = params
	}

	if d.Output == nil {
		out, err := w.output()
		if err != nil {
			return false, err
		}
		d.Output = out
	}

	if err := w.options(d); err != nil {
		return false, err
	}

	spec, err := d.Build()
	if err != nil {
		return false, err
	}
	preview, err := spec.EncodeYAML()
	if err != nil {
		return false, err
	}
	ok := true
	if err := w.run(huh.NewForm(huh.NewGroup(
		huh.NewNote().Title("spec.yaml").Description(string(preview)),
		huh.NewConfirm().
			Title("Create the project?").
			Affirmative("Create").
			Negative("Cancel").
			Value(&ok),
	))); err != nil {
		return false, err
	}
	return ok, nil
}

// fields asks for name/type pairs until the user leaves the name empty.
func (w *initWizard) fields(title, noun, help string) ([]project.Field, error) {
	var fields []project.Field
	for {
		var name string
		desc := help
		if len(fields) > 0 {
			desc = "So far: " + project.FormatType(project.RecordOf(fields...)) + "\nLeave empty to finish."
		}
		if err := w.run(huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("%s %d name", title, len(fields)+1)).
				Description(desc).
				Value(&name).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						if len(fields) == 0 {
							return fmt.Errorf("at least one %s is required", noun)
						}
						return nil
					}
					return checkFieldName(s, fields)
				}),
		))); err != nil {
			return nil, err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			if len(fields) == 0 { // only reachable when input ends early (EOF)
				return nil, fmt.Errorf("at least one %s is required", noun)
			}
			return fields, nil
		}
		t, err := w.pickType(fmt.Sprintf("Type of %s", name))
		if err != nil {
			return nil, err
		}
		fields = append(fields, project.F(name, t))
	}
}

// pickType offers the common types plus a free-form type expression.
func (w *initWizard) pickType(title string) (*project.Type, error) {
	choice := basicTypes[0]
	if err := w.run(huh.NewForm(huh.NewGroup(typeSelect(title, &choice)))); err != nil {
		return nil, err
	}
	if choice != customType {
		return project.ParseTypeExpr(choice)
	}
	var expr string
	if err := w.run(huh.NewForm(huh.NewGroup(customTypeInput(title, &expr)))); err != nil {
		return nil, err
	}
	return project.ParseTypeExpr(expr)
}

func (w *initWizard) output() (*project.Type, error) {
	kind := outputSingle
	if err := w.run(huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Output").
			Description("What the generated function returns.").
			Options(
				huh.NewOption("A single value (int, string, list, …)", outputSingle),
				huh.NewOption("A record with named fields", outputRecord),
			).
			Value(&kind),
	))); err != nil {
		return nil, err
	}
	if kind == outputSingle {
		return w.pickType("Output type")
	}
	fields, err := w.fields("Output field", "output field", "Each output field is synthesised as its own sub-problem.")
	if err != nil {
		return nil, err
	}
	return project.RecordOf(fields...), nil
}

func (w *initWizard) options(d *specDraft) error {
	tol := ""
	if d.FloatTolerance != 0 {
		tol = strconv.FormatFloat(d.FloatTolerance, 'g', -1, 64)
	}
	targets := d.Targets
	if len(targets) == 0 {
		targets = []string{"go"}
	}
	var fields []huh.Field
	if hasFloat(d.Output) {
		fields = append(fields, huh.NewInput().
			Title("Float tolerance").
			Description("Optional: how far a float output may be from the expected value, e.g. 1e-9.").
			Value(&tol).
			Validate(func(s string) error {
				_, err := parseTolerance(s)
				return err
			}))
	}
	fields = append(fields, huh.NewMultiSelect[string]().
		Title("Code generation targets").
		Options(huh.NewOptions(project.KnownTargets...)...).
		Value(&targets).
		Validate(func(v []string) error {
			if len(v) == 0 {
				return errors.New("pick at least one target")
			}
			return nil
		}))
	if err := w.run(huh.NewForm(huh.NewGroup(fields...).Title("Options"))); err != nil {
		return err
	}
	f, err := parseTolerance(tol)
	if err != nil {
		return err
	}
	d.FloatTolerance = f
	d.Targets = targets
	return nil
}

// parseTolerance reads an optional float tolerance; empty means unset (0).
func parseTolerance(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("invalid tolerance %q: want a number >= 0", s)
	}
	return f, nil
}
