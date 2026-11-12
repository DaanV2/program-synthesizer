package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"go.yaml.in/yaml/v3"
)

// Spec is the contents of a project's spec.yaml (or spec.json).
type Spec struct {
	Name        string  `yaml:"name" json:"name"`
	Description string  `yaml:"description,omitempty" json:"description,omitempty"`
	Input       *Type   `yaml:"input" json:"input"`   // always a record: named parameters
	Output      *Type   `yaml:"output" json:"output"` // any type
	Options     Options `yaml:"options,omitempty" json:"options,omitzero"`
}

// Options are optional search and codegen settings.
type Options struct {
	FloatTolerance float64  `yaml:"float_tolerance,omitempty" json:"float_tolerance,omitempty"`
	Targets        []string `yaml:"targets,omitempty" json:"targets,omitempty"`
}

// KnownTargets are the code generation back ends a spec may list.
var KnownTargets = []string{"go", "rust", "c"}

// Validate checks the spec for problems that the YAML parser cannot catch.
func (s *Spec) Validate() error {
	var errs []error
	if s.Name == "" {
		errs = append(errs, errors.New("name is required"))
	}
	switch {
	case s.Input == nil:
		errs = append(errs, errors.New("input is required"))
	case s.Input.Kind != Record:
		errs = append(errs, fmt.Errorf("input must be a mapping of parameter names to types, got %s", s.Input))
	}
	if s.Output == nil {
		errs = append(errs, errors.New("output is required"))
	}
	if s.Options.FloatTolerance < 0 {
		errs = append(errs, errors.New("options.float_tolerance must be >= 0"))
	}
	for _, t := range s.Options.Targets {
		if !slices.Contains(KnownTargets, t) {
			errs = append(errs, fmt.Errorf("options.targets: unknown target %q (known: %v)", t, KnownTargets))
		}
	}
	return errors.Join(errs...)
}

// ParseSpecYAML decodes and validates a spec from YAML.
func ParseSpecYAML(b []byte) (*Spec, error) {
	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	return &s, s.Validate()
}

// ParseSpecJSON decodes and validates a spec from JSON. JSON objects are
// parsed via YAML (a superset) so record field order is preserved.
func ParseSpecJSON(b []byte) (*Spec, error) {
	if !json.Valid(b) {
		return nil, errors.New("invalid JSON")
	}
	return ParseSpecYAML(b)
}

// EncodeYAML writes the spec as YAML, keeping record field order.
func (s *Spec) EncodeYAML() ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return buf.Bytes(), enc.Close()
}

// EncodeJSON writes the spec as JSON, keeping record field order.
func (s *Spec) EncodeJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{\n")
	writeKV := func(key string, val []byte, last bool) {
		fmt.Fprintf(&buf, "  %q: %s", key, val)
		if !last {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	name, _ := json.Marshal(s.Name)
	entries := [][2]any{{"name", name}}
	if s.Description != "" {
		d, _ := json.Marshal(s.Description)
		entries = append(entries, [2]any{"description", d})
	}
	entries = append(entries, [2]any{"input", typeJSON(s.Input)}, [2]any{"output", typeJSON(s.Output)})
	if s.Options.FloatTolerance != 0 || len(s.Options.Targets) > 0 {
		o, err := json.Marshal(s.Options)
		if err != nil {
			return nil, err
		}
		entries = append(entries, [2]any{"options", o})
	}
	for i, e := range entries {
		writeKV(e[0].(string), e[1].([]byte), i == len(entries)-1)
	}
	buf.WriteString("}\n")
	return buf.Bytes(), nil
}

func typeJSON(t *Type) []byte {
	if t == nil {
		return []byte("null")
	}
	switch t.Kind {
	case Record:
		var b bytes.Buffer
		b.WriteString("{")
		for i, f := range t.Fields {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%q: %s", f.Name, typeJSON(f.Type))
		}
		b.WriteString("}")
		return b.Bytes()
	case List:
		if t.hasRecord() {
			return append(append([]byte("["), typeJSON(t.Elem)...), ']')
		}
	}
	return jsonString(t.String())
}

// jsonString quotes s without escaping <, > and & (json.Marshal would turn
// list<int> into list\u003cint\u003e).
func jsonString(s string) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return bytes.TrimRight(b.Bytes(), "\n")
}
