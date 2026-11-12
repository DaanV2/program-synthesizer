package dataio

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	"github.com/DaanV2/program-synthesizer/internal/project"
)

// Preview renders v as compact one-line JSON, listing record fields in the
// order t declares them (falling back to sorted order for anything that
// does not match t), and truncates the result to at most maxRunes runes
// with a trailing "…". maxRunes <= 0 means no limit.
func Preview(v any, t *project.Type, maxRunes int) string {
	var b strings.Builder
	writeCompact(&b, v, t)

	return truncate(b.String(), maxRunes)
}

func writeCompact(b *strings.Builder, v any, t *project.Type) {
	switch x := v.(type) {
	case map[string]any:
		b.WriteByte('{')
		first := true
		emit := func(k string, fv any, ft *project.Type) {
			if !first {
				b.WriteByte(',')
			}
			first = false
			kb, _ := marshal(k)
			b.Write(kb)
			b.WriteByte(':')
			writeCompact(b, fv, ft)
		}
		done := map[string]bool{}
		if t != nil && t.Kind == project.Record {
			for _, f := range t.Fields {
				if fv, ok := x[f.Name]; ok {
					emit(f.Name, fv, f.Type)
					done[f.Name] = true
				}
			}
		}
		// Unknown fields (invalid data) in sorted order so they still show.
		rest := make([]string, 0, len(x))
		for k := range x {
			if !done[k] {
				rest = append(rest, k)
			}
		}
		slices.Sort(rest)
		for _, k := range rest {
			emit(k, x[k], nil)
		}
		b.WriteByte('}')
	case []any:
		var elem *project.Type
		if t != nil && t.Kind == project.List {
			elem = t.Elem
		}
		b.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			writeCompact(b, item, elem)
		}
		b.WriteByte(']')
	default:
		enc, err := marshal(x)
		if err != nil {
			b.WriteString("?")

			return
		}
		b.Write(enc)
	}
}

func truncate(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	if maxRunes == 1 {
		return "…"
	}

	return string(r[:maxRunes-1]) + "…"
}

// marshal encodes a JSON scalar or key without escaping <, > and &.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}

	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}
