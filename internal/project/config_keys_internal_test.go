package project

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

const keysSpec = `name: demo
input:
  first: string
  age: int
  address: {city: string, zip: string}
  orders: [{id: int, total: float}]
output:
  fullName: string
  isAdult: bool
options:
  targets: [go]
`

func mustSpec(t *testing.T, src string) *Spec {
	t.Helper()
	s, err := ParseSpecYAML([]byte(src))
	if err != nil {
		t.Fatal(err)
	}

	return s
}

func TestParseKey(t *testing.T) {
	ok := map[string]string{
		"name":                    "name",
		"options.targets":         "options.targets",
		"input":                   "input",
		"input.a":                 "input:a",
		"input.a.b":               "input:a.b",
		"output.items[].id":       "output:items[].id",
		"input.m[][].x":           "input:m[][].x",
		"options.float_tolerance": "options.float_tolerance",
	}
	for in, want := range ok {
		k, err := ParseKey(in)
		if err != nil {
			t.Errorf("%s: %v", in, err)

			continue
		}
		got := k.Root
		if len(k.Path) > 0 {
			segs := make([]string, len(k.Path))
			for i, s := range k.Path {
				segs[i] = s.String()
			}
			got += ":" + strings.Join(segs, ".")
		}
		if got != want {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
	}
	bad := map[string]string{
		"foo":           "unknown key",
		"options":       "unknown key",
		"options.foo":   "unknown key",
		"name.x":        "unknown key",
		"input.":        "not a valid field name",
		"input.a..b":    "not a valid field name",
		"input.1x":      "not a valid field name",
		"input.a-b":     "not a valid field name",
		"output.[]":     "not a valid field name",
		"description.x": "unknown key",
	}
	for in, want := range bad {
		if _, err := ParseKey(in); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want error containing %q", in, err, want)
		}
	}
}

func TestParseTypeExprAndFormat(t *testing.T) {
	cases := map[string]string{ // input -> FormatType
		"int":                        "int",
		" list<string> ":             "list<string>",
		"list<list<int>>":            "list<list<int>>",
		"[int]":                      "list<int>",
		"{x: int, y: int}":           "{x: int, y: int}",
		"[{id: int}]":                "[{id: int}]",
		"list<{id: int}>":            "[{id: int}]",
		"list<[{id: int}]>":          "[[{id: int}]]",
		"{a: {b: [{c: string}]}}":    "{a: {b: [{c: string}]}}",
		"{tags: list<string>}":       "{tags: list<string>}",
		"{xs: [{p: float, q: int}]}": "{xs: [{p: float, q: int}]}",
	}
	for in, want := range cases {
		typ, err := ParseTypeExpr(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)

			continue
		}
		got := FormatType(typ)
		if got != want {
			t.Errorf("%q: formatted %q, want %q", in, got, want)
		}
		// FormatType output must parse back to the same type.
		back, err := ParseTypeExpr(got)
		if err != nil || !back.Equal(typ) {
			t.Errorf("%q: round trip of %q failed: %v", in, got, err)
		}
	}
	bad := map[string]string{
		"":                 "empty type",
		"integer":          "unknown type",
		"list<int":         "missing closing",
		"{x: int, x: int}": "", // yaml itself rejects duplicate keys; any error is fine
		"{}":               "at least one field",
		"[int, string]":    "exactly one element",
		"{1a: int}":        "not a valid identifier",
		"{x: int":          "",
		"list<{x: nope}>":  "unknown type",
		"{x: int}extra":    "unexpected",
		"[int] z":          "unexpected",
	}
	for in, want := range bad {
		if _, err := ParseTypeExpr(in); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want error containing %q", in, err, want)
		}
	}
}

func TestList(t *testing.T) {
	s := mustSpec(t, keysSpec)
	s.Description = "Join names"
	s.Options.FloatTolerance = 0.001
	var lines []string
	for _, kv := range s.List() {
		lines = append(lines, kv.Key+" = "+kv.Value)
	}
	want := []string{
		"name = demo",
		"description = Join names",
		"input.first = string",
		"input.age = int",
		"input.address = {city: string, zip: string}",
		"input.orders = [{id: int, total: float}]",
		"output.fullName = string",
		"output.isAdult = bool",
		"options.float_tolerance = 0.001",
		"options.targets = go",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}

	s.Output = Scalar(Int)
	s.Description = ""
	s.Options = Options{}
	kvs := s.List()
	if last := kvs[len(kvs)-1]; last.Key != "output" || last.Value != "int" {
		t.Fatalf("scalar output listed as %v", last)
	}
	for _, kv := range kvs {
		if strings.HasPrefix(kv.Key, "options") || kv.Key == "description" {
			t.Fatalf("unset key listed: %v", kv)
		}
	}
}

func TestGet(t *testing.T) {
	s := mustSpec(t, keysSpec)
	cases := map[string]string{
		"name":                 "demo",
		"input":                "{first: string, age: int, address: {city: string, zip: string}, orders: [{id: int, total: float}]}",
		"input.age":            "int",
		"input.address":        "{city: string, zip: string}",
		"input.address.city":   "string",
		"input.orders":         "[{id: int, total: float}]",
		"input.orders[]":       "{id: int, total: float}",
		"input.orders[].total": "float",
		"output":               "{fullName: string, isAdult: bool}",
		"output.isAdult":       "bool",
		"options.targets":      "go",
	}
	for key, want := range cases {
		got, err := s.Get(key)
		if err != nil || got != want {
			t.Errorf("Get(%s) = %q, %v; want %q", key, got, err, want)
		}
	}
	for _, key := range []string{"description", "options.float_tolerance"} {
		if _, err := s.Get(key); !errors.Is(err, ErrNotSet) {
			t.Errorf("Get(%s): got %v, want ErrNotSet", key, err)
		}
	}
	bad := map[string]string{
		"input.nope":        `no field "nope" in input`,
		"input.age.x":       "expected a record at input.age, found int",
		"input.orders.id":   "use input.orders[] to reach",
		"input.first[]":     "expected a list at input.first",
		"output.fullName.x": "expected a record at output.fullName",
		"bogus":             "unknown key",
	}
	for key, want := range bad {
		if _, err := s.Get(key); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Get(%s): got %v, want error containing %q", key, err, want)
		}
	}
}

func TestSet(t *testing.T) {
	type step struct{ key, value string }
	cases := []struct {
		name  string
		steps []step
		check map[string]string // key -> expected Get value after the steps
	}{
		{
			"scalars",
			[]step{{"name", "other"}, {"description", "hello world"}, {"options.float_tolerance", "1e-6"}, {"options.targets", " Rust, go ,rust"}},
			map[string]string{"name": "other", "description": "hello world", "options.float_tolerance": "1e-06", "options.targets": "rust,go"},
		},
		{"targets with brackets", []step{{"options.targets", "[c]"}}, map[string]string{"options.targets": "c"}},
		{
			"replace param keeps position",
			[]step{{"input.first", "list<string>"}},
			map[string]string{"input": "{first: list<string>, age: int, address: {city: string, zip: string}, orders: [{id: int, total: float}]}"},
		},
		{"add param at end", []step{{"input.score", "float"}}, map[string]string{"input.score": "float"}},
		{"nested field", []step{{"input.address.country", "string"}}, map[string]string{"input.address": "{city: string, zip: string, country: string}"}},
		{"list element field", []step{{"input.orders[].qty", "int"}}, map[string]string{"input.orders": "[{id: int, total: float, qty: int}]"}},
		{"create intermediate records", []step{{"input.geo.pos.lat", "float"}}, map[string]string{"input.geo": "{pos: {lat: float}}"}},
		{"whole input", []step{{"input", "{x: int, y: int}"}}, map[string]string{"input": "{x: int, y: int}"}},
		{"whole output scalar", []step{{"output", "list<int>"}}, map[string]string{"output": "list<int>"}},
		{"output record field", []step{{"output.initials", "string"}}, map[string]string{"output": "{fullName: string, isAdult: bool, initials: string}"}},
		{"output becomes record", []step{{"output", "int"}, {"output", "{a: int}"}, {"output.b", "bool"}}, map[string]string{"output": "{a: int, b: bool}"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := mustSpec(t, keysSpec)
			for _, st := range tc.steps {
				if err := s.Set(st.key, st.value); err != nil {
					t.Fatalf("Set(%s, %s): %v", st.key, st.value, err)
				}
			}
			for key, want := range tc.check {
				if got, err := s.Get(key); err != nil || got != want {
					t.Errorf("Get(%s) = %q, %v; want %q", key, got, err, want)
				}
			}
			if err := s.Validate(); err != nil {
				t.Errorf("spec invalid after set: %v", err)
			}
			// The result must survive an encode/parse round trip.
			b, err := s.EncodeYAML()
			if err != nil {
				t.Fatal(err)
			}
			s2, err := ParseSpecYAML(b)
			if err != nil || !s2.Input.Equal(s.Input) || !s2.Output.Equal(s.Output) {
				t.Fatalf("round trip failed: %v\n%s", err, b)
			}
		})
	}
}

func TestSetErrors(t *testing.T) {
	bad := []struct{ key, value, want string }{
		{"name", "  ", "must not be empty"},
		{"options.float_tolerance", "abc", "not a number"},
		{"options.float_tolerance", "NaN", "not a number"},
		{"options.float_tolerance", "-1", ">= 0"},
		{"options.targets", "go,java", `unknown target "java"`},
		{"options.targets", " , ", "no targets given"},
		{"input", "int", "input must be a record"},
		{"input.age", "integer", "unknown type"},
		{"input.age.x", "int", "expected a record at input.age, found int"},
		{"input.orders.x", "int", "use input.orders[]"},
		{"input.orders[]", "{a: int}", "cannot set a list element type"},
		{"input.nope[].x", "int", `no field "nope" in input`},
		{"output", "", "empty type"},
		{"input.x", "{a: int", ""},
		{"unknown.key", "int", "unknown key"},
	}
	for _, tc := range bad {
		s := mustSpec(t, keysSpec)
		before, _ := s.EncodeYAML()
		err := s.Set(tc.key, tc.value)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Set(%s, %q): got %v, want error containing %q", tc.key, tc.value, err, tc.want)
		}
		if after, _ := s.EncodeYAML(); !bytes.Equal(after, before) {
			t.Errorf("Set(%s, %q) failed but changed the spec", tc.key, tc.value)
		}
	}
	s := mustSpec(t, keysSpec)
	s.Output = Scalar(Int)
	if err := s.Set("output.x", "int"); err == nil || !strings.Contains(err.Error(), "expected a record at output, found int") {
		t.Errorf("Set(output.x) on scalar output: %v", err)
	}
}

func TestUnset(t *testing.T) {
	s := mustSpec(t, keysSpec)
	s.Description = "d"
	s.Options.FloatTolerance = 0.5
	for _, key := range []string{"description", "options.float_tolerance", "options.targets", "input.age", "input.address.zip", "input.orders[].total", "output.isAdult"} {
		if err := s.Unset(key); err != nil {
			t.Fatalf("Unset(%s): %v", key, err)
		}
	}
	// Unsetting an optional value again is a no-op.
	if err := s.Unset("description"); err != nil {
		t.Fatalf("second Unset(description): %v", err)
	}
	want := map[string]string{
		"input":  "{first: string, address: {city: string}, orders: [{id: int}]}",
		"output": "{fullName: string}",
	}
	for key, w := range want {
		if got, _ := s.Get(key); got != w {
			t.Errorf("Get(%s) = %q, want %q", key, got, w)
		}
	}
	if s.Description != "" || s.Options.FloatTolerance != 0 || s.Options.Targets != nil {
		t.Errorf("options not cleared: %+v", s.Options)
	}

	bad := map[string]string{
		"name":               "cannot unset",
		"input":              "cannot unset",
		"output":             "cannot unset",
		"input.nope":         `no field "nope" in input`,
		"input.address.city": "only field of input.address",
		"output.fullName":    "only field of output",
		"input.orders[]":     "cannot unset a list element",
		"input.first.x":      "expected a record at input.first",
	}
	for key, w := range bad {
		if err := s.Unset(key); err == nil || !strings.Contains(err.Error(), w) {
			t.Errorf("Unset(%s): got %v, want error containing %q", key, err, w)
		}
	}
}

func TestMove(t *testing.T) {
	s := mustSpec(t, keysSpec)
	steps := []struct {
		key, other string
		after      bool
		want       string
	}{
		{"input.age", "first", false, "age,first,address,orders"},
		{"input.age", "orders", true, "first,address,orders,age"},
		{"input.first", "age", true, "address,orders,age,first"},
		{"input.orders", "address", false, "orders,address,age,first"},
	}
	for _, st := range steps {
		if err := s.Move(st.key, st.other, st.after); err != nil {
			t.Fatalf("Move(%s, %s): %v", st.key, st.other, err)
		}
		var names []string
		for _, f := range s.Input.Fields {
			names = append(names, f.Name)
		}
		if got := strings.Join(names, ","); got != st.want {
			t.Fatalf("after Move(%s, %s, %v): %s, want %s", st.key, st.other, st.after, got, st.want)
		}
	}
	if err := s.Move("input.address.zip", "city", false); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get("input.address"); got != "{zip: string, city: string}" {
		t.Fatalf("nested move: %s", got)
	}
	bad := map[[2]string]string{
		{"name", "x"}:             "only input/output record fields",
		{"input", "x"}:            "only input/output record fields",
		{"input.age", "age"}:      "relative to itself",
		{"input.age", "nope"}:     `no field "nope"`,
		{"input.nope", "age"}:     `no field "nope"`,
		{"input.orders[]", "age"}: "not a record field",
	}
	for args, w := range bad {
		if err := s.Move(args[0], args[1], false); err == nil || !strings.Contains(err.Error(), w) {
			t.Errorf("Move(%v): got %v, want error containing %q", args, err, w)
		}
	}
}

func TestMergeYAMLComments(t *testing.T) {
	old := `# Project spec
name: demo # the name
input:
  # people
  first: string
  age: int # years
output: {fullName: string}
options:
  targets: [go] # backends
`
	s := mustSpec(t, old)
	if err := s.Set("input.age", "float"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("input.tags", "list<string>"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("options.targets", "go,rust"); err != nil {
		t.Fatal(err)
	}
	b, err := MergeYAMLComments([]byte(old), s)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{"# Project spec", "name: demo # the name", "# people", "age: float # years", "output: {fullName: string}", "targets: [go, rust] # backends", "tags: list<string>"} {
		if !strings.Contains(got, want) {
			t.Errorf("merged YAML lacks %q:\n%s", want, got)
		}
	}
	s2, err := ParseSpecYAML(b)
	if err != nil || !s2.Input.Equal(s.Input) {
		t.Fatalf("merged YAML does not parse back: %v\n%s", err, got)
	}
}
