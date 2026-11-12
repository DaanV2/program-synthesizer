package project

import (
	"fmt"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Kind is the shape of a Type.
type Kind int

const (
	Int Kind = iota
	Float
	Bool
	String
	List
	Record
)

var kindNames = map[Kind]string{Int: "int", Float: "float", Bool: "bool", String: "string", List: "list", Record: "record"}

func (k Kind) String() string { return kindNames[k] }

// Type is a value type in the synthesizer's type system.
//
// In spec.yaml a type is written as:
//   - a scalar name:          int, float, bool, string
//   - a list in string form:  list<int>, list<list<string>>
//   - a list as a sequence:   [int] or [{name: string}]   (needed for lists of records)
//   - a record as a mapping:  {first: string, age: int}   (field order is kept)
type Type struct {
	Kind   Kind
	Elem   *Type   // List only
	Fields []Field // Record only, in declaration order
}

// Field is one named member of a record.
type Field struct {
	Name string
	Type *Type
}

func Scalar(k Kind) *Type          { return &Type{Kind: k} }
func ListOf(elem *Type) *Type      { return &Type{Kind: List, Elem: elem} }
func RecordOf(f ...Field) *Type    { return &Type{Kind: Record, Fields: f} }
func F(name string, t *Type) Field { return Field{Name: name, Type: t} }

// Field returns the record field with the given name.
func (t *Type) Field(name string) (*Field, bool) {
	for i := range t.Fields {
		if t.Fields[i].Name == name {
			return &t.Fields[i], true
		}
	}
	return nil, false
}

// String renders the type compactly; records use {a: int, b: string}.
func (t *Type) String() string {
	switch t.Kind {
	case List:
		return "list<" + t.Elem.String() + ">"
	case Record:
		parts := make([]string, len(t.Fields))
		for i, f := range t.Fields {
			parts[i] = f.Name + ": " + f.Type.String()
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return t.Kind.String()
	}
}

// Equal reports structural equality (record field order matters).
func (t *Type) Equal(o *Type) bool {
	if t == nil || o == nil {
		return t == o
	}
	if t.Kind != o.Kind {
		return false
	}
	switch t.Kind {
	case List:
		return t.Elem.Equal(o.Elem)
	case Record:
		if len(t.Fields) != len(o.Fields) {
			return false
		}
		for i := range t.Fields {
			if t.Fields[i].Name != o.Fields[i].Name || !t.Fields[i].Type.Equal(o.Fields[i].Type) {
				return false
			}
		}
	}
	return true
}

// ParseType parses the string form: int, float, bool, string, list<T>.
// Records have no string form; write them as YAML mappings.
func ParseType(s string) (*Type, error) {
	s = strings.TrimSpace(s)
	switch s {
	case "int":
		return Scalar(Int), nil
	case "float":
		return Scalar(Float), nil
	case "bool":
		return Scalar(Bool), nil
	case "string":
		return Scalar(String), nil
	}
	if inner, ok := strings.CutPrefix(s, "list<"); ok {
		inner, ok = strings.CutSuffix(inner, ">")
		if !ok {
			return nil, fmt.Errorf("type %q: missing closing '>'", s)
		}
		elem, err := ParseType(inner)
		if err != nil {
			return nil, err
		}
		return ListOf(elem), nil
	}
	return nil, fmt.Errorf("unknown type %q (want int, float, bool, string, list<T>, or a mapping for a record)", s)
}

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidIdent reports whether name can be used as a field/parameter name in
// every code generation target.
func ValidIdent(name string) bool { return identRe.MatchString(name) }

// UnmarshalYAML implements yaml.Unmarshaler.
func (t *Type) UnmarshalYAML(n *yaml.Node) error {
	parsed, err := typeFromNode(n)
	if err != nil {
		return err
	}
	*t = *parsed
	return nil
}

func typeFromNode(n *yaml.Node) (*Type, error) {
	switch n.Kind {
	case yaml.ScalarNode:
		t, err := ParseType(n.Value)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n.Line, err)
		}
		return t, nil
	case yaml.SequenceNode:
		if len(n.Content) != 1 {
			return nil, fmt.Errorf("line %d: list type must have exactly one element type, e.g. [int]", n.Line)
		}
		elem, err := typeFromNode(n.Content[0])
		if err != nil {
			return nil, err
		}
		return ListOf(elem), nil
	case yaml.MappingNode:
		if len(n.Content) == 0 {
			return nil, fmt.Errorf("line %d: record must have at least one field", n.Line)
		}
		rec := &Type{Kind: Record}
		for i := 0; i < len(n.Content); i += 2 {
			name := n.Content[i].Value
			if !ValidIdent(name) {
				return nil, fmt.Errorf("line %d: field name %q is not a valid identifier", n.Content[i].Line, name)
			}
			if _, dup := rec.Field(name); dup {
				return nil, fmt.Errorf("line %d: duplicate field %q", n.Content[i].Line, name)
			}
			ft, err := typeFromNode(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			rec.Fields = append(rec.Fields, F(name, ft))
		}
		return rec, nil
	case yaml.AliasNode:
		return typeFromNode(n.Alias)
	}
	return nil, fmt.Errorf("line %d: invalid type", n.Line)
}

// MarshalYAML implements yaml.Marshaler, producing the form ParseType/UnmarshalYAML read.
func (t *Type) MarshalYAML() (any, error) { return t.node(), nil }

func (t *Type) node() *yaml.Node {
	switch t.Kind {
	case Record:
		n := &yaml.Node{Kind: yaml.MappingNode}
		for _, f := range t.Fields {
			n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: f.Name}, f.Type.node())
		}
		return n
	case List:
		if t.hasRecord() {
			return &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle, Content: []*yaml.Node{t.Elem.node()}}
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Value: t.String()}
	default:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: t.String()}
	}
}

func (t *Type) hasRecord() bool {
	switch t.Kind {
	case Record:
		return true
	case List:
		return t.Elem.hasRecord()
	}
	return false
}
