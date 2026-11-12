package cli

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/huh/v2"

	"synth/internal/project"
)

// maxSlots is how many input parameters (and output fields) the one-form
// wizard offers. More can be added afterwards with `synth config set`.
const maxSlots = 12

// Output kinds in the wizard.
const (
	outputSingle = "single"
	outputRecord = "record"
)

// typeSlot is one name/type row of the wizard. Fields are exported because
// huh hashes bindings with hashstructure, which skips unexported fields.
type typeSlot struct {
	Name   string
	Choice string // one of basicTypes or customType
	Custom string // type expression when Choice is customType
}

// wizardState holds every answer of the one-form wizard.
type wizardState struct {
	Name, Description string
	Inputs            []typeSlot
	OutputKind        string
	Output            typeSlot // Choice/Custom for a single-value output
	OutputFields      []typeSlot
	Tolerance         string
	Targets           []string
	Confirm           bool
}

func newWizardState(d *specDraft) *wizardState {
	st := &wizardState{
		Name:         d.Name,
		Description:  d.Description,
		Inputs:       newSlots(),
		OutputKind:   outputSingle,
		Output:       typeSlot{Choice: basicTypes[0]},
		OutputFields: newSlots(),
		Targets:      slices.Clone(d.Targets),
		Confirm:      true,
	}
	if d.FloatTolerance != 0 {
		st.Tolerance = strconv.FormatFloat(d.FloatTolerance, 'g', -1, 64)
	}
	if len(st.Targets) == 0 {
		st.Targets = []string{"go"}
	}
	return st
}

func newSlots() []typeSlot {
	s := make([]typeSlot, maxSlots)
	for i := range s {
		s[i].Choice = basicTypes[0]
	}
	return s
}

// slotVisible reports whether slot i is reachable: every slot before it
// has a name.
func slotVisible(slots []typeSlot, i int) bool {
	for _, s := range slots[:i] {
		if strings.TrimSpace(s.Name) == "" {
			return false
		}
	}
	return true
}

// slotType resolves the type picked in a slot.
func slotType(s typeSlot) (*project.Type, error) {
	if s.Choice == customType {
		return project.ParseTypeExpr(s.Custom)
	}
	return project.ParseTypeExpr(s.Choice)
}

// collectFields turns the named prefix of slots into record fields.
func collectFields(slots []typeSlot) ([]project.Field, error) {
	var fields []project.Field
	for _, s := range slots {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			break
		}
		if err := checkFieldName(name, fields); err != nil {
			return nil, err
		}
		t, err := slotType(s)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", name, err)
		}
		fields = append(fields, project.F(name, t))
	}
	return fields, nil
}

// outputType resolves the output from the state.
func (st *wizardState) outputType() (*project.Type, error) {
	if st.OutputKind == outputSingle {
		return slotType(st.Output)
	}
	fields, err := collectFields(st.OutputFields)
	if err != nil {
		return nil, fmt.Errorf("output: %w", err)
	}
	if len(fields) == 0 {
		return nil, errors.New("output record needs at least one field")
	}
	return project.RecordOf(fields...), nil
}

// apply copies the answers into d. Params and Output already in d (from
// flags) are kept.
func (st *wizardState) apply(d *specDraft) error {
	d.Name = strings.TrimSpace(st.Name)
	d.Description = strings.TrimSpace(st.Description)
	if len(d.Params) == 0 {
		params, err := collectFields(st.Inputs)
		if err != nil {
			return fmt.Errorf("input: %w", err)
		}
		d.Params = params
	}
	if d.Output == nil {
		out, err := st.outputType()
		if err != nil {
			return err
		}
		d.Output = out
	}
	tol, err := parseTolerance(st.Tolerance)
	if err != nil {
		return err
	}
	if d.Output != nil && !hasFloat(d.Output) {
		tol = 0 // the question was hidden
	}
	d.FloatTolerance = tol
	d.Targets = slices.Clone(st.Targets)
	return nil
}

// preview renders the spec the current answers would produce.
func (st *wizardState) preview(base specDraft) string {
	d := base
	if err := st.apply(&d); err != nil {
		return "Cannot build the spec yet: " + err.Error()
	}
	s, err := d.Build()
	if err != nil {
		return "Cannot build the spec yet: " + err.Error()
	}
	b, err := s.EncodeYAML()
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// fillForm runs the whole wizard as one huh form.
func (w *initWizard) fillForm(d *specDraft, dir string) (bool, error) {
	st := newWizardState(d)
	askInputs, askOutput := len(d.Params) == 0, d.Output == nil
	defName := d.Name

	groups := []*huh.Group{huh.NewGroup(
		huh.NewInput().
			Title("Project name").
			Value(&st.Name).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" && defName == "" {
					return errors.New("name is required")
				}
				return nil
			}),
		huh.NewInput().
			Title("Description").
			Description("Optional: what the synthesised function should do.").
			Value(&st.Description),
	).Title("New synth project in " + dir)}

	if askInputs {
		groups = append(groups, slotGroups(st.Inputs, "Input parameter", "parameter",
			"Parameters become the arguments of the generated function, in this order.", nil)...)
	}

	if askOutput {
		isSingle := func() bool { return st.OutputKind == outputSingle }
		groups = append(groups,
			huh.NewGroup(huh.NewSelect[string]().
				Title("Output").
				Description("What the generated function returns.").
				Options(
					huh.NewOption("A single value (int, string, list, …)", outputSingle),
					huh.NewOption("A record with named fields", outputRecord),
				).
				Value(&st.OutputKind)),
			huh.NewGroup(typeSelect("Output type", &st.Output.Choice)).
				WithHideFunc(func() bool { return !isSingle() }),
			huh.NewGroup(customTypeInput("Output type", &st.Output.Custom)).
				WithHideFunc(func() bool { return !isSingle() || st.Output.Choice != customType }),
		)
		groups = append(groups, slotGroups(st.OutputFields, "Output field", "output field",
			"Each output field is synthesised as its own sub-problem.", isSingle)...)
	}

	outputHasFloat := func() bool {
		if d.Output != nil {
			return hasFloat(d.Output)
		}
		t, err := st.outputType()
		return err == nil && hasFloat(t)
	}
	groups = append(groups,
		huh.NewGroup(huh.NewInput().
			Title("Float tolerance").
			Description("Optional: how far a float output may be from the expected value, e.g. 1e-9.").
			Value(&st.Tolerance).
			Validate(func(s string) error {
				_, err := parseTolerance(s)
				return err
			})).
			Title("Options").
			WithHideFunc(func() bool { return !outputHasFloat() }),
		huh.NewGroup(huh.NewMultiSelect[string]().
			Title("Code generation targets").
			Options(huh.NewOptions(project.KnownTargets...)...).
			Value(&st.Targets).
			Validate(func(v []string) error {
				if len(v) == 0 {
					return errors.New("pick at least one target")
				}
				return nil
			})).
			Title("Options"),
		huh.NewGroup(
			huh.NewNote().
				Title("spec.yaml").
				DescriptionFunc(func() string { return st.preview(*d) }, st),
			huh.NewConfirm().
				Title("Create the project?").
				Affirmative("Create").
				Negative("Cancel").
				Value(&st.Confirm).
				Validate(func(ok bool) error {
					if !ok {
						return nil
					}
					dd := *d
					if err := st.apply(&dd); err != nil {
						return err
					}
					_, err := dd.Build()
					return err
				}),
		),
	)

	if err := w.run(huh.NewForm(groups...)); err != nil {
		return false, err
	}
	if !st.Confirm {
		return false, nil
	}
	if strings.TrimSpace(st.Name) == "" {
		st.Name = defName
	}
	return true, st.apply(d)
}

// slotGroups builds the name / type / custom-type groups for every slot.
// hideAll, if set, hides all of them (e.g. record fields while the output is
// a single value).
func slotGroups(slots []typeSlot, title, noun, help string, hideAll func() bool) []*huh.Group {
	hidden := func() bool { return hideAll != nil && hideAll() }
	var groups []*huh.Group
	for i := range slots {
		s := &slots[i]
		desc := help
		if i > 0 {
			desc = "Leave empty to finish."
		}
		if i == len(slots)-1 {
			desc += "\nThis is the last one the wizard offers; add more later with `synth config set`."
		}
		visible := func() bool { return !hidden() && slotVisible(slots, i) }
		named := func() bool { return visible() && strings.TrimSpace(s.Name) != "" }
		groups = append(groups,
			huh.NewGroup(huh.NewInput().
				Title(fmt.Sprintf("%s %d name", title, i+1)).
				Description(desc).
				Value(&s.Name).
				Validate(func(v string) error {
					v = strings.TrimSpace(v)
					if v == "" {
						if i == 0 {
							return fmt.Errorf("at least one %s is required", noun)
						}
						return nil
					}
					var earlier []project.Field
					for _, o := range slots[:i] {
						earlier = append(earlier, project.F(strings.TrimSpace(o.Name), nil))
					}
					return checkFieldName(v, earlier)
				})).
				WithHideFunc(func() bool { return !visible() }),
			huh.NewGroup(typeSelect(fmt.Sprintf("%s %d type", title, i+1), &s.Choice).
				TitleFunc(func() string { return "Type of " + strings.TrimSpace(s.Name) }, s)).
				WithHideFunc(func() bool { return !named() }),
			huh.NewGroup(customTypeInput(fmt.Sprintf("%s %d type", title, i+1), &s.Custom).
				TitleFunc(func() string { return "Type of " + strings.TrimSpace(s.Name) }, s)).
				WithHideFunc(func() bool { return !named() || s.Choice != customType }),
		)
	}
	return groups
}

func typeSelect(title string, value *string) *huh.Select[string] {
	opts := huh.NewOptions(basicTypes...)
	opts = append(opts, huh.NewOption("custom… (record, nested list, …)", customType))
	return huh.NewSelect[string]().Title(title).Options(opts...).Value(value)
}

func customTypeInput(title string, value *string) *huh.Input {
	return huh.NewInput().
		Title(title).
		Description("Type expression, e.g. list<list<int>>, {x: int, y: int} or [{id: int, name: string}]").
		Placeholder("{x: int, y: int}").
		Value(value).
		Validate(func(s string) error {
			_, err := project.ParseTypeExpr(s)
			return err
		})
}
