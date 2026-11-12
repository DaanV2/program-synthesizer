package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
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
		return nil, errors.New("trailing data after JSON value")
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
	e := valueEncoder{b: b, canonical: canonical}

	return e.value(t, v, indent)
}

// valueEncoder writes typed values as JSON, either indented for files or
// compact and normalised (canonical) for comparisons.
type valueEncoder struct {
	b         *bytes.Buffer
	canonical bool
}

func (e *valueEncoder) value(t *Type, v any, indent string) error {
	switch t.Kind {
	case Record:
		return e.record(t, v, indent)
	case List:
		return e.list(t, v, indent)
	default:
		return e.scalar(t.Kind, v)
	}
}

func (e *valueEncoder) record(t *Type, v any, indent string) error {
	obj, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("expected object, got %s", describe(v))
	}
	nl, inner, sep := "\n", indent+"  ", ": "
	if e.canonical {
		nl, inner, sep = "", "", ":"
	}
	e.b.WriteString("{" + nl)
	for i, f := range t.Fields {
		if i > 0 {
			e.b.WriteString("," + nl)
		}
		e.b.WriteString(inner)
		e.b.Write(jsonQuote(f.Name))
		e.b.WriteString(sep)
		if err := e.value(f.Type, obj[f.Name], inner); err != nil {
			return err
		}
	}
	if !e.canonical {
		e.b.WriteString("\n" + indent)
	}
	e.b.WriteString("}")

	return nil
}

func (e *valueEncoder) list(t *Type, v any, indent string) error {
	items, ok := v.([]any)
	if !ok {
		return fmt.Errorf("expected array, got %s", describe(v))
	}
	if len(items) == 0 {
		e.b.WriteString("[]")

		return nil
	}
	// Lists of scalars stay on one line; lists of records/lists get one item per line.
	multiline := !e.canonical && (t.Elem.Kind == Record || t.Elem.Kind == List)
	open, sep, itemIndent, closing := "[", ", ", "", "]"
	switch {
	case multiline:
		open, sep, itemIndent, closing = "[\n", ",\n", indent+"  ", "\n"+indent+"]"
	case e.canonical:
		sep = ","
	}
	e.b.WriteString(open)
	for i, item := range items {
		if i > 0 {
			e.b.WriteString(sep)
		}
		e.b.WriteString(itemIndent)
		if err := e.value(t.Elem, item, itemIndent); err != nil {
			return err
		}
	}
	e.b.WriteString(closing)

	return nil
}

func (e *valueEncoder) scalar(k Kind, v any) error {
	switch x := v.(type) {
	case json.Number:
		if k != Int && k != Float {
			break
		}
		if k == Float && e.canonical {
			f, err := x.Float64()
			if err != nil {
				return err
			}
			e.b.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
		} else {
			e.b.WriteString(x.String())
		}

		return nil
	case bool:
		if k == Bool {
			e.b.WriteString(strconv.FormatBool(x))

			return nil
		}
	case string:
		if k == String {
			e.b.Write(jsonQuote(x))

			return nil
		}
	}

	return fmt.Errorf("expected %s, got %s", k, describe(v))
}

func jsonQuote(s string) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		panic(err) // encoding a string into a bytes.Buffer cannot fail
	}

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
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()

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
	if v == nil {
		return failAt(path, "expected %s, got null", t)
	}
	switch t.Kind {
	case List:
		return t.checkList(v, path)
	case Record:
		return t.checkRecord(v, path)
	default:
		return checkScalar(t.Kind, v, path)
	}
}

func failAt(path, format string, a ...any) error {
	return &ValueError{Path: path, Msg: fmt.Sprintf(format, a...)}
}

func checkScalar(k Kind, v any, path string) error {
	var ok bool
	switch k {
	case Int, Float:
		var n json.Number
		if n, ok = v.(json.Number); ok {
			var err error
			if k == Int {
				_, err = strconv.ParseInt(n.String(), 10, 64)
			} else {
				_, err = n.Float64()
			}
			if err != nil {
				return failAt(path, "expected %s, got %s", k, n)
			}
		}
	case Bool:
		_, ok = v.(bool)
	case String:
		_, ok = v.(string)
	default:
		return failAt(path, "%s is not a scalar type", k)
	}
	if !ok {
		return failAt(path, "expected %s, got %s", k, describe(v))
	}

	return nil
}

func (t *Type) checkList(v any, path string) error {
	items, ok := v.([]any)
	if !ok {
		return failAt(path, "expected %s, got %s", t, describe(v))
	}
	for i, item := range items {
		if err := t.Elem.check(item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}

	return nil
}

func (t *Type) checkRecord(v any, path string) error {
	obj, ok := v.(map[string]any)
	if !ok {
		return failAt(path, "expected object, got %s", describe(v))
	}
	for _, f := range t.Fields {
		fv, present := obj[f.Name]
		if !present {
			return failAt(path, "missing field %q", f.Name)
		}
		if err := f.Type.check(fv, join(path, f.Name)); err != nil {
			return err
		}
	}
	// Sorted so the reported field is the same on every run.
	for _, k := range slices.Sorted(maps.Keys(obj)) {
		if _, known := t.Field(k); !known {
			return failAt(path, "unexpected field %q", k)
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
