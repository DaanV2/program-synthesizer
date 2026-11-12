package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// EncodeValue renders v (which must conform to t) as indented JSON with
// record fields in declaration order, without HTML escaping, and with a
// trailing newline.
func EncodeValue(t *Type, v any) ([]byte, error) {
	var b bytes.Buffer
	if err := encodeValue(&b, t, v, "", false); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

// CanonicalValue renders v compactly with numbers normalised (1.0 and 1
// are the same float), so equal values give equal strings.
func CanonicalValue(t *Type, v any) (string, error) {
	var b bytes.Buffer
	err := encodeValue(&b, t, v, "", true)
	return b.String(), err
}

func encodeValue(b *bytes.Buffer, t *Type, v any, indent string, canonical bool) error {
	nl, inner, sep := "\n", indent+"  ", ": "
	if canonical {
		nl, inner, sep = "", "", ":"
	}
	switch t.Kind {
	case Record:
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("expected object, got %s", describe(v))
		}
		b.WriteString("{" + nl)
		for i, f := range t.Fields {
			if i > 0 {
				b.WriteString("," + nl)
			}
			b.WriteString(inner)
			b.Write(jsonQuote(f.Name))
			b.WriteString(sep)
			if err := encodeValue(b, f.Type, obj[f.Name], inner, canonical); err != nil {
				return err
			}
		}
		if canonical {
			b.WriteString("}")
		} else {
			b.WriteString("\n" + indent + "}")
		}
	case List:
		items, ok := v.([]any)
		if !ok {
			return fmt.Errorf("expected array, got %s", describe(v))
		}
		if len(items) == 0 {
			b.WriteString("[]")
			return nil
		}
		// Lists of scalars stay on one line; lists of records/lists get one item per line.
		multiline := !canonical && (t.Elem.Kind == Record || t.Elem.Kind == List)
		itemNL, itemIndent := "", ""
		if multiline {
			itemNL, itemIndent = "\n", inner
		}
		b.WriteString("[" + itemNL)
		for i, item := range items {
			if i > 0 {
				b.WriteString(",")
				if multiline {
					b.WriteString("\n")
				} else if !canonical {
					b.WriteString(" ")
				}
			}
			b.WriteString(itemIndent)
			if err := encodeValue(b, t.Elem, item, itemIndent, canonical); err != nil {
				return err
			}
		}
		if multiline {
			b.WriteString("\n" + indent)
		}
		b.WriteString("]")
	case Float:
		n, ok := v.(json.Number)
		if !ok {
			return fmt.Errorf("expected number, got %s", describe(v))
		}
		if canonical {
			f, err := n.Float64()
			if err != nil {
				return err
			}
			b.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
		} else {
			b.WriteString(n.String())
		}
	case Int:
		n, ok := v.(json.Number)
		if !ok {
			return fmt.Errorf("expected number, got %s", describe(v))
		}
		b.WriteString(n.String())
	case Bool:
		x, ok := v.(bool)
		if !ok {
			return fmt.Errorf("expected bool, got %s", describe(v))
		}
		b.WriteString(strconv.FormatBool(x))
	case String:
		x, ok := v.(string)
		if !ok {
			return fmt.Errorf("expected string, got %s", describe(v))
		}
		b.Write(jsonQuote(x))
	}
	return nil
}

func jsonQuote(s string) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return bytes.TrimRight(b.Bytes(), "\n")
}

// WriteValueFile writes v as JSON (see EncodeValue) atomically: it writes a
// temporary file next to path and renames it into place.
func WriteValueFile(path string, t *Type, v any) error {
	b, err := EncodeValue(t, v)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return writeFileAtomic(path, b)
}

func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
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
