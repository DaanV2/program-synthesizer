package dataio

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ReadJSONL reads one example per non-empty line, each an object of the form
// {"input": ..., "output": ...}. source names the file in error messages.
// Numbers are kept as json.Number so int and float stay distinguishable.
func ReadJSONL(r io.Reader, source string) (*Batch, error) {
	b := &Batch{}
	br := bufio.NewReader(r)
	for lineNo := 1; ; lineNo++ {
		line, err := br.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("reading %s: %w", source, err)
		}
		if lineNo == 1 {
			line = trimBOM(line)
		}
		if text := strings.TrimSpace(line); text != "" {
			loc := fmt.Sprintf("%s:%d", source, lineNo)
			rec, perr := parseJSONLLine(text)
			if perr != nil {
				b.Problems = append(b.Problems, &sourceError{Source: loc, Err: perr})
			} else {
				rec.Source = loc
				b.Records = append(b.Records, rec)
			}
		}
		if errors.Is(err, io.EOF) {
			return b, nil
		}
	}
}

func parseJSONLLine(text string) (Record, error) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil {
		return Record{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if dec.More() {
		return Record{}, errors.New("trailing data after JSON object (one example per line)")
	}
	if obj == nil {
		return Record{}, errors.New(`expected an object {"input": ..., "output": ...}, got null`)
	}
	for k := range obj {
		if k != "input" && k != "output" {
			return Record{}, fmt.Errorf("unexpected key %q (want only \"input\" and \"output\")", k)
		}
	}
	var rec Record
	for _, f := range []struct {
		key string
		dst *any
	}{{"input", &rec.Input}, {"output", &rec.Output}} {
		key, dst := f.key, f.dst
		raw, ok := obj[key]
		if !ok {
			return Record{}, fmt.Errorf("missing key %q", key)
		}
		v, err := decodeRaw(raw)
		if err != nil {
			return Record{}, fmt.Errorf("%s: %w", key, err)
		}
		*dst = v
	}
	return rec, nil
}

func decodeRaw(raw json.RawMessage) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	err := dec.Decode(&v)
	return v, err
}
