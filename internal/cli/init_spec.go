package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"synth/internal/project"
)

// specDraft collects the answers of `synth init` (from flags or the wizard)
// before they become a project.Spec. It holds no UI code so it can be tested
// without a terminal.
type specDraft struct {
	Name           string
	Description    string
	Params         []project.Field
	Output         *project.Type
	FloatTolerance float64
	Targets        []string
}

// Build checks the draft and turns it into a spec.
func (d *specDraft) Build() (*project.Spec, error) {
	if strings.TrimSpace(d.Name) == "" {
		return nil, errors.New("project name is required")
	}
	if len(d.Params) == 0 {
		return nil, errors.New("at least one input parameter is required")
	}
	seen := map[string]bool{}
	for _, f := range d.Params {
		if err := checkFieldName(f.Name, nil); err != nil {
			return nil, fmt.Errorf("invalid input parameter: %w", err)
		}
		if seen[f.Name] {
			return nil, fmt.Errorf("duplicate input parameter %q", f.Name)
		}
		seen[f.Name] = true
	}
	if d.Output == nil {
		return nil, errors.New("output type is required")
	}
	s := &project.Spec{
		Name:        strings.TrimSpace(d.Name),
		Description: strings.TrimSpace(d.Description),
		Input:       project.RecordOf(slices.Clone(d.Params)...),
		Output:      d.Output,
		Options:     project.Options{FloatTolerance: d.FloatTolerance, Targets: d.Targets},
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}

// checkFieldName validates a parameter or field name against the names
// already taken.
func checkFieldName(name string, taken []project.Field) error {
	if name == "" {
		return errors.New("name must not be empty")
	}
	if !project.ValidIdent(name) {
		return fmt.Errorf("invalid name %q (use letters, digits and _, not starting with a digit)", name)
	}
	if slices.ContainsFunc(taken, func(f project.Field) bool { return f.Name == name }) {
		return fmt.Errorf("name %q is already used", name)
	}
	return nil
}

// defaultProjectName derives a project name from the project directory.
func defaultProjectName(absDir string) string {
	base := filepath.Base(absDir)
	if base == "." || base == string(filepath.Separator) || strings.HasSuffix(base, ":") || base == "" {
		return ""
	}
	return base
}

// parseFieldList parses a record given on the command line, either as
// comma-separated name=type pairs ("first=string,age=int") or as a YAML flow
// mapping ("{first: string, age: int}"). Types inside name=type pairs may
// contain commas when they are bracketed: "pos={x: int, y: int},n=int".
func parseFieldList(s string) ([]project.Field, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("empty field list")
	}
	if strings.HasPrefix(s, "{") {
		t, err := project.ParseTypeExpr(s)
		if err != nil {
			return nil, err
		}
		if t.Kind != project.Record {
			return nil, fmt.Errorf("not a record: %q", s)
		}
		return t.Fields, nil
	}
	parts, err := splitTopLevel(s, ',')
	if err != nil {
		return nil, err
	}
	var fields []project.Field
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, typ, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("invalid field %q: want name=type", part)
		}
		name = strings.TrimSpace(name)
		if err := checkFieldName(name, fields); err != nil {
			return nil, err
		}
		t, err := project.ParseTypeExpr(typ)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", name, err)
		}
		fields = append(fields, project.F(name, t))
	}
	if len(fields) == 0 {
		return nil, errors.New("empty field list")
	}
	return fields, nil
}

// splitTopLevel splits s on sep where sep is not nested inside <>, {} or [].
func splitTopLevel(s string, sep rune) ([]string, error) {
	var parts []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '<', '{', '[':
			depth++
		case '>', '}', ']':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced %q in %q", r, s)
			}
		case sep:
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("unclosed bracket in %q", s)
	}
	return append(parts, s[start:]), nil
}

// parseInputFlags combines repeated --input values into one parameter list.
func parseInputFlags(values []string) ([]project.Field, error) {
	var all []project.Field
	for _, v := range values {
		fields, err := parseFieldList(v)
		if err != nil {
			return nil, fmt.Errorf("invalid --input: %w", err)
		}
		for _, f := range fields {
			if err := checkFieldName(f.Name, all); err != nil {
				return nil, fmt.Errorf("invalid --input: %w", err)
			}
			all = append(all, f)
		}
	}
	return all, nil
}

// parseOutputFlag reads --output: a type ("int", "list<string>",
// "{a: int}", "[{id: int}]") or a record as name=type pairs.
func parseOutputFlag(s string) (*project.Type, error) {
	s = strings.TrimSpace(s)
	parts, err := splitTopLevel(s, ',')
	if err != nil {
		return nil, fmt.Errorf("invalid --output: %w", err)
	}
	if strings.Contains(parts[0], "=") {
		fields, err := parseFieldList(s)
		if err != nil {
			return nil, fmt.Errorf("invalid --output: %w", err)
		}
		return project.RecordOf(fields...), nil
	}
	t, err := project.ParseTypeExpr(s)
	if err != nil {
		return nil, fmt.Errorf("invalid --output: %w", err)
	}
	return t, nil
}

// hasFloat reports whether t contains a float anywhere.
func hasFloat(t *project.Type) bool {
	switch t.Kind {
	case project.Float:
		return true
	case project.List:
		return hasFloat(t.Elem)
	case project.Record:
		return slices.ContainsFunc(t.Fields, func(f project.Field) bool { return hasFloat(f.Type) })
	}
	return false
}
