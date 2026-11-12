package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
)

// Values are decoded from JSON with UseNumber, so they are one of:
// json.Number, bool, string, []any, map[string]any (or nil, which is always invalid).

// DecodeValue reads exactly one JSON value, keeping numbers as json.Number so
// that int and float can be told apart.
func DecodeValue(r io.Reader) (any, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("trailing data after JSON value")
	}
	return v, nil
}

// ReadValueFile decodes the JSON value stored in path.
func ReadValueFile(path string) (any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	v, err := DecodeValue(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return v, nil
}

// WriteValueFile writes v as indented JSON with a trailing newline.
func WriteValueFile(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// ValueError points at the part of a value that does not match its type.
type ValueError struct {
	Path string // e.g. "items[2].name"; empty for the root
	Msg  string
}

func (e *ValueError) Error() string {
	if e.Path == "" {
		return e.Msg
	}
	return e.Path + ": " + e.Msg
}

// Check reports whether v (as returned by DecodeValue) conforms to t.
// root names the top of the path in errors, e.g. "input" or "output".
func (t *Type) Check(v any, root string) error {
	return t.check(v, root)
}

func (t *Type) check(v any, path string) error {
	fail := func(format string, a ...any) error {
		return &ValueError{Path: path, Msg: fmt.Sprintf(format, a...)}
	}
	if v == nil {
		return fail("expected %s, got null", t)
	}
	switch t.Kind {
	case Int:
		n, ok := v.(json.Number)
		if !ok {
			return fail("expected int, got %s", describe(v))
		}
		if _, err := strconv.ParseInt(n.String(), 10, 64); err != nil {
			return fail("expected int, got %s", n)
		}
	case Float:
		n, ok := v.(json.Number)
		if !ok {
			return fail("expected float, got %s", describe(v))
		}
		if _, err := n.Float64(); err != nil {
			return fail("expected float, got %s", n)
		}
	case Bool:
		if _, ok := v.(bool); !ok {
			return fail("expected bool, got %s", describe(v))
		}
	case String:
		if _, ok := v.(string); !ok {
			return fail("expected string, got %s", describe(v))
		}
	case List:
		items, ok := v.([]any)
		if !ok {
			return fail("expected %s, got %s", t, describe(v))
		}
		for i, item := range items {
			if err := t.Elem.check(item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case Record:
		obj, ok := v.(map[string]any)
		if !ok {
			return fail("expected object, got %s", describe(v))
		}
		for _, f := range t.Fields {
			fv, present := obj[f.Name]
			if !present {
				return fail("missing field %q", f.Name)
			}
			if err := f.Type.check(fv, join(path, f.Name)); err != nil {
				return err
			}
		}
		for k := range obj {
			if _, known := t.Field(k); !known {
				return fail("unexpected field %q", k)
			}
		}
	}
	return nil
}

func join(path, field string) string {
	if path == "" {
		return field
	}
	return path + "." + field
}

func describe(v any) string {
	switch x := v.(type) {
	case json.Number:
		return "number " + x.String()
	case bool:
		return "bool"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return fmt.Sprintf("%T", v)
}
