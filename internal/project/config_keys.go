package project

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// This file implements the dotted keys used by `synth config`:
//
//	name                     project name
//	description              free text
//	input                    the whole input record
//	input.<param>            one parameter's type
//	input.<a>.<b>            a field of a record-typed parameter
//	input.<a>[].<b>          a field of the record inside a list<record> parameter
//	output                   the whole output type
//	output.<field>[...]      a field of a record output (same path rules as input)
//	options.float_tolerance  number >= 0
//	options.targets          comma-separated subset of KnownTargets
//
// Type values use the same syntax as spec.yaml, written on one line:
// int, float, bool, string, list<T>, [T], {name: T, ...}. list<{...}> is
// accepted as a convenience and means the same as [{...}].

// Key names that are not type paths.
const (
	KeyName           = "name"
	KeyDescription    = "description"
	KeyInput          = "input"
	KeyOutput         = "output"
	KeyFloatTolerance = "options.float_tolerance"
	KeyTargets        = "options.targets"
)

// ErrNotSet is returned by Spec.Get for an optional key that has no value.
var ErrNotSet = errors.New("no value set")

// KeyValue is one entry of Spec.List.
type KeyValue struct {
	Key   string
	Value string
}

// Key is a parsed config key.
type Key struct {
	Raw  string
	Root string   // one of the Key* constants above; for type paths "input" or "output"
	Path []KeySeg // record path below input/output; empty for the root itself
}

// KeySeg is one dotted segment of a type path: a field name, followed by
// zero or more "[]" that step into list element types.
type KeySeg struct {
	Name  string
	Lists int
}

func (s KeySeg) String() string { return s.Name + strings.Repeat("[]", s.Lists) }

// IsType reports whether the key addresses a type (input/output or below).
func (k Key) IsType() bool { return k.Root == KeyInput || k.Root == KeyOutput }

// ParseKey parses and checks the syntax of a config key. It does not look
// at a spec, so fields named in the path need not exist.
func ParseKey(s string) (Key, error) {
	k := Key{Raw: s}
	switch s {
	case KeyName, KeyDescription, KeyFloatTolerance, KeyTargets, KeyInput, KeyOutput:
		k.Root = s
		return k, nil
	}
	root, rest, ok := strings.Cut(s, ".")
	if !ok || (root != KeyInput && root != KeyOutput) {
		return k, unknownKey(s)
	}
	k.Root = root
	for part := range strings.SplitSeq(rest, ".") {
		name, lists := part, 0
		for {
			n, ok := strings.CutSuffix(name, "[]")
			if !ok {
				break
			}
			name = n
			lists++
		}
		if !ValidIdent(name) {
			return k, fmt.Errorf("key %q: %q is not a valid field name", s, part)
		}
		k.Path = append(k.Path, KeySeg{Name: name, Lists: lists})
	}
	return k, nil
}

func unknownKey(s string) error {
	return fmt.Errorf("unknown key %q (known: name, description, input, input.<param>, output, output.<field>, %s, %s)",
		s, KeyFloatTolerance, KeyTargets)
}

// ParseTypeExpr parses a type written on one line in spec.yaml syntax, e.g.
// "list<int>", "{first: string, age: int}" or "[{id: int}]".
func ParseTypeExpr(s string) (*Type, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("empty type")
	}
	// list<{...}> / list<[...]>: the string form cannot hold records, so
	// parse the element as an expression of its own.
	if inner, ok := strings.CutPrefix(s, "list<"); ok {
		if inner, ok = strings.CutSuffix(inner, ">"); ok {
			if t := strings.TrimSpace(inner); strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[") || strings.HasPrefix(t, "list<") {
				elem, err := ParseTypeExpr(inner)
				if err != nil {
					return nil, err
				}
				return ListOf(elem), nil
			}
		}
	}
	if err := checkTrailing(s); err != nil {
		return nil, err
	}
	var n yaml.Node
	if err := yaml.Unmarshal([]byte(s), &n); err != nil {
		return nil, fmt.Errorf("invalid type %q: %w", s, cleanYAMLErr(err))
	}
	if n.Kind != yaml.DocumentNode || len(n.Content) != 1 {
		return nil, fmt.Errorf("invalid type %q: expected a single type", s)
	}
	t, err := typeFromNode(n.Content[0])
	if err != nil {
		// typeFromNode's messages already name the offending part.
		return nil, errors.New(strings.TrimPrefix(err.Error(), "line 1: "))
	}
	return t, nil
}

// checkTrailing rejects text after a top-level flow collection, which the
// YAML decoder silently ignores ("{x: int} y" would parse as {x: int}).
func checkTrailing(s string) error {
	if s[0] != '{' && s[0] != '[' {
		return nil
	}
	depth := 0
	var quote rune
	for i, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '{' || r == '[':
			depth++
		case r == '}' || r == ']':
			depth--
			if depth == 0 {
				if rest := strings.TrimSpace(s[i+1:]); rest != "" && !strings.HasPrefix(rest, "#") {
					return fmt.Errorf("invalid type %q: unexpected %q after the closing %q", s, rest, r)
				}
				return nil
			}
		}
	}
	return nil // unclosed: the decoder reports it
}

func cleanYAMLErr(err error) error {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	msg = strings.TrimPrefix(msg, "line 1: ")
	return errors.New(msg)
}

// FormatType renders t on one line in a form that ParseTypeExpr reads back:
// like Type.String, except that lists containing records use [T].
func FormatType(t *Type) string {
	if t == nil {
		return ""
	}
	switch t.Kind {
	case Record:
		parts := make([]string, len(t.Fields))
		for i, f := range t.Fields {
			parts[i] = f.Name + ": " + FormatType(f.Type)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case List:
		if t.hasRecord() {
			return "[" + FormatType(t.Elem) + "]"
		}
	}
	return t.String()
}

// List returns every set key with its value, in spec order: name,
// description, one line per input parameter, output (one line per field when
// it is a record), then options.
func (s *Spec) List() []KeyValue {
	kvs := []KeyValue{{KeyName, s.Name}}
	if s.Description != "" {
		kvs = append(kvs, KeyValue{KeyDescription, s.Description})
	}
	if s.Input != nil {
		for _, f := range s.Input.Fields {
			kvs = append(kvs, KeyValue{KeyInput + "." + f.Name, FormatType(f.Type)})
		}
	}
	if s.Output != nil {
		if s.Output.Kind == Record {
			for _, f := range s.Output.Fields {
				kvs = append(kvs, KeyValue{KeyOutput + "." + f.Name, FormatType(f.Type)})
			}
		} else {
			kvs = append(kvs, KeyValue{KeyOutput, FormatType(s.Output)})
		}
	}
	if s.Options.FloatTolerance != 0 {
		kvs = append(kvs, KeyValue{KeyFloatTolerance, formatFloat(s.Options.FloatTolerance)})
	}
	if len(s.Options.Targets) > 0 {
		kvs = append(kvs, KeyValue{KeyTargets, strings.Join(s.Options.Targets, ",")})
	}
	return kvs
}

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

// Get returns the value of key as it would be passed to Set. Optional keys
// without a value return an error wrapping ErrNotSet.
func (s *Spec) Get(key string) (string, error) {
	k, err := ParseKey(key)
	if err != nil {
		return "", err
	}
	notSet := fmt.Errorf("%w for %s", ErrNotSet, key)
	switch k.Root {
	case KeyName:
		return s.Name, nil
	case KeyDescription:
		if s.Description == "" {
			return "", notSet
		}
		return s.Description, nil
	case KeyFloatTolerance:
		if s.Options.FloatTolerance == 0 {
			return "", notSet
		}
		return formatFloat(s.Options.FloatTolerance), nil
	case KeyTargets:
		if len(s.Options.Targets) == 0 {
			return "", notSet
		}
		return strings.Join(s.Options.Targets, ","), nil
	}
	t, err := s.lookup(k)
	if err != nil {
		return "", err
	}
	return FormatType(t), nil
}

// rootType returns a pointer to the Input or Output field of s.
func (s *Spec) rootType(root string) **Type {
	if root == KeyInput {
		return &s.Input
	}
	return &s.Output
}

// lookup follows a type path and returns the type it names.
func (s *Spec) lookup(k Key) (*Type, error) {
	t := *s.rootType(k.Root)
	if t == nil {
		return nil, fmt.Errorf("%w for %s", ErrNotSet, k.Root)
	}
	at := k.Root
	for _, seg := range k.Path {
		rec, err := asRecord(t, at)
		if err != nil {
			return nil, err
		}
		f, ok := rec.Field(seg.Name)
		if !ok {
			return nil, fmt.Errorf("no field %q in %s", seg.Name, at)
		}
		at += "." + seg.Name
		t = f.Type
		for range seg.Lists {
			if t.Kind != List {
				return nil, fmt.Errorf("expected a list at %s, found %s", at, FormatType(t))
			}
			at += "[]"
			t = t.Elem
		}
	}
	return t, nil
}

// asRecord checks that t (found at path at) is a record.
func asRecord(t *Type, at string) (*Type, error) {
	if t.Kind == Record {
		return t, nil
	}
	hint := ""
	if t.Kind == List && t.Elem.Kind == Record {
		hint = fmt.Sprintf(" (use %s[] to reach its element's fields)", at)
	}
	return nil, fmt.Errorf("expected a record at %s, found %s%s", at, FormatType(t), hint)
}

// parent follows all but the last segment of a type path and returns the
// record that holds the last one. With create, missing intermediate fields
// (without []) are added as new single-field records on the way down.
func (s *Spec) parent(k Key, create bool) (*Type, string, error) {
	t := *s.rootType(k.Root)
	at := k.Root
	if t == nil {
		return nil, at, fmt.Errorf("%w for %s", ErrNotSet, k.Root)
	}
	for _, seg := range k.Path[:len(k.Path)-1] {
		rec, err := asRecord(t, at)
		if err != nil {
			return nil, at, err
		}
		f, ok := rec.Field(seg.Name)
		if !ok {
			if !create || seg.Lists > 0 {
				return nil, at, fmt.Errorf("no field %q in %s", seg.Name, at)
			}
			// Create the missing record; the caller adds the last field.
			nt := &Type{Kind: Record}
			rec.Fields = append(rec.Fields, F(seg.Name, nt))
			t, at = nt, at+"."+seg.Name
			continue
		}
		at += "." + seg.Name
		t = f.Type
		for range seg.Lists {
			if t.Kind != List {
				return nil, at, fmt.Errorf("expected a list at %s, found %s", at, FormatType(t))
			}
			at += "[]"
			t = t.Elem
		}
	}
	rec, err := asRecord(t, at)
	return rec, at, err
}

// Set assigns value to key. For record paths a missing last field is
// appended to its record, and missing intermediate records are created.
func (s *Spec) Set(key, value string) error {
	k, err := ParseKey(key)
	if err != nil {
		return err
	}
	switch k.Root {
	case KeyName:
		if strings.TrimSpace(value) == "" {
			return errors.New("name must not be empty")
		}
		s.Name = value
		return nil
	case KeyDescription:
		s.Description = value
		return nil
	case KeyFloatTolerance:
		f, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("invalid %s: %q is not a number", key, value)
		}
		if f < 0 {
			return fmt.Errorf("invalid %s: must be >= 0", key)
		}
		s.Options.FloatTolerance = f
		return nil
	case KeyTargets:
		targets, err := ParseTargets(value)
		if err != nil {
			return fmt.Errorf("invalid %s: %w", key, err)
		}
		if len(targets) == 0 {
			return fmt.Errorf("no targets given for %s (use unset to remove the setting)", key)
		}
		s.Options.Targets = targets
		return nil
	}

	t, err := ParseTypeExpr(value)
	if err != nil {
		return err
	}
	if len(k.Path) == 0 {
		if k.Root == KeyInput && t.Kind != Record {
			return fmt.Errorf("input must be a record of named parameters, e.g. {x: int, y: int}; got %s", FormatType(t))
		}
		*s.rootType(k.Root) = t
		return nil
	}
	last := k.Path[len(k.Path)-1]
	if last.Lists > 0 {
		return fmt.Errorf("key %q: cannot set a list element type directly; set %s to a list type instead",
			key, strings.TrimSuffix(key, strings.Repeat("[]", last.Lists)))
	}
	rec, _, err := s.parent(k, true)
	if err != nil {
		return err
	}
	if f, ok := rec.Field(last.Name); ok {
		f.Type = t
	} else {
		rec.Fields = append(rec.Fields, F(last.Name, t))
	}
	return nil
}

// Unset removes key. name, input and output cannot be removed; neither can
// the last field of a record. Unsetting an optional setting that has no
// value is not an error; removing a field that does not exist is.
func (s *Spec) Unset(key string) error {
	k, err := ParseKey(key)
	if err != nil {
		return err
	}
	switch k.Root {
	case KeyName, KeyInput, KeyOutput:
		if len(k.Path) == 0 {
			return fmt.Errorf("cannot unset %s: it is required (use set to change it)", key)
		}
	case KeyDescription:
		s.Description = ""
		return nil
	case KeyFloatTolerance:
		s.Options.FloatTolerance = 0
		return nil
	case KeyTargets:
		s.Options.Targets = nil
		return nil
	}
	last := k.Path[len(k.Path)-1]
	if last.Lists > 0 {
		return fmt.Errorf("key %q: cannot unset a list element type", key)
	}
	rec, at, err := s.parent(k, false)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(rec.Fields, func(f Field) bool { return f.Name == last.Name })
	if i < 0 {
		return fmt.Errorf("no field %q in %s", last.Name, at)
	}
	if len(rec.Fields) == 1 {
		return fmt.Errorf("cannot remove %s: it is the only field of %s (set %s to a new type instead)", key, at, at)
	}
	rec.Fields = slices.Delete(rec.Fields, i, i+1)
	return nil
}

// Move reorders a record field among its siblings: it is placed just before
// (or, with after, just after) the sibling field named other.
func (s *Spec) Move(key, other string, after bool) error {
	k, err := ParseKey(key)
	if err != nil {
		return err
	}
	if !k.IsType() || len(k.Path) == 0 {
		return fmt.Errorf("key %q: only input/output record fields can be moved", key)
	}
	last := k.Path[len(k.Path)-1]
	if last.Lists > 0 {
		return fmt.Errorf("key %q: not a record field", key)
	}
	rec, at, err := s.parent(k, false)
	if err != nil {
		return err
	}
	from := slices.IndexFunc(rec.Fields, func(f Field) bool { return f.Name == last.Name })
	if from < 0 {
		return fmt.Errorf("no field %q in %s", last.Name, at)
	}
	if other == last.Name {
		return fmt.Errorf("cannot move %s relative to itself", key)
	}
	if slices.IndexFunc(rec.Fields, func(f Field) bool { return f.Name == other }) < 0 {
		return fmt.Errorf("no field %q in %s", other, at)
	}
	f := rec.Fields[from]
	rec.Fields = slices.Delete(rec.Fields, from, from+1)
	to := slices.IndexFunc(rec.Fields, func(f Field) bool { return f.Name == other })
	if after {
		to++
	}
	rec.Fields = slices.Insert(rec.Fields, to, f)
	return nil
}

// ParseTargets parses a comma-separated target list such as "go,rust"
// (spaces and surrounding [ ] are allowed). Duplicates are dropped and
// unknown targets rejected.
func ParseTargets(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		t := strings.ToLower(strings.TrimSpace(part))
		if t == "" {
			continue
		}
		if !slices.Contains(KnownTargets, t) {
			return nil, fmt.Errorf("unknown target %q (known: %s)", t, strings.Join(KnownTargets, ", "))
		}
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out, nil
}
