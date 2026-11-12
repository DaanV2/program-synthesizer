package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleSpec = `name: full-name
description: Join names
input:
  first: string
  last: string
  age: int
  tags: list<string>
  orders: [{id: int, total: float}]
output:
  fullName: string
  isAdult: bool
options:
  float_tolerance: 0.001
  targets: [go]
`

func TestSpecRoundTripKeepsFieldOrder(t *testing.T) {
	s, err := ParseSpecYAML([]byte(sampleSpec))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"first", "last", "age", "tags", "orders"}
	for i, f := range s.Input.Fields {
		if f.Name != want[i] {
			t.Fatalf("field %d = %s, want %s", i, f.Name, want[i])
		}
	}
	if got := s.Input.Fields[4].Type.String(); got != "list<{id: int, total: float}>" {
		t.Fatalf("orders type = %s", got)
	}
	for _, enc := range []func() ([]byte, error){s.EncodeYAML, s.EncodeJSON} {
		b, err := enc()
		if err != nil {
			t.Fatal(err)
		}
		s2, err := ParseSpecYAML(b)
		if err != nil {
			t.Fatalf("reparse: %v\n%s", err, b)
		}
		if !s.Input.Equal(s2.Input) || !s.Output.Equal(s2.Output) || s2.Options.FloatTolerance != 0.001 {
			t.Fatalf("round trip changed spec:\n%s", b)
		}
	}
}

func TestSpecErrors(t *testing.T) {
	cases := map[string]string{
		"name: x\ninput: int\noutput: int\n":                                  "input must be a mapping",
		"name: x\ninput: {a: integer}\noutput: int\n":                         "unknown type",
		"name: x\ninput: {a: int}\noutput: int\nbogus: 1\n":                   "bogus",
		"name: x\ninput: {a b: int}\noutput: int\n":                           "not a valid identifier",
		"input: {a: int}\noutput: int\n":                                      "name is required",
		"name: x\ninput: {a: int}\noutput: int\noptions: {targets: [java]}\n": "unknown target",
	}
	for src, want := range cases {
		_, err := ParseSpecYAML([]byte(src))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want error containing %q", src, err, want)
		}
	}
}

func TestCheck(t *testing.T) {
	s, _ := ParseSpecYAML([]byte(sampleSpec))
	cases := map[string]string{
		`{"first":"a","last":"b","age":3,"tags":[],"orders":[{"id":1,"total":2}]}`: "",
		`{"first":"a","last":"b","age":3.5,"tags":[],"orders":[]}`:                 "age: expected int, got 3.5",
		`{"first":"a","last":"b","age":3,"tags":[1],"orders":[]}`:                  "tags[0]: expected string",
		`{"first":"a","last":"b","age":3,"tags":[],"orders":[{"id":1}]}`:           `orders[0]: missing field "total"`,
		`{"first":"a","last":"b","age":3,"tags":[],"orders":[],"x":1}`:             `unexpected field "x"`,
		`{"first":null,"last":"b","age":3,"tags":[],"orders":[]}`:                  "first: expected string, got null",
	}
	for src, want := range cases {
		v, err := DecodeValue(strings.NewReader(src))
		if err != nil {
			t.Fatal(err)
		}
		err = s.Input.Check(v, "input")
		if want == "" && err != nil || want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Errorf("%s: got %v, want %q", src, err, want)
		}
	}
}

func TestProjectLifecycle(t *testing.T) {
	dir := t.TempDir()
	s, _ := ParseSpecYAML([]byte(sampleSpec))
	p, err := Create(dir, s)
	if err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "data", "train")
	found, err := Find(sub)
	if err != nil || found != dir {
		t.Fatalf("Find = %q, %v", found, err)
	}
	in := map[string]any{"first": "Ada", "last": "L", "age": 36, "tags": []any{}, "orders": []any{}}
	// ints from Go code are not json.Number; round-trip through JSON first.
	name, _ := p.NextName(Train)
	if name != "001" {
		t.Fatalf("NextName = %s", name)
	}
	inV, _ := DecodeValue(strings.NewReader(`{"first":"Ada","last":"L","age":36,"tags":[],"orders":[]}`))
	outV, _ := DecodeValue(strings.NewReader(`{"fullName":"Ada L","isAdult":true}`))
	_ = in
	if _, err := p.WriteExample(Train, name, inV, outV, false); err != nil {
		t.Fatal(err)
	}
	if _, err := p.WriteExample(Train, name, inV, outV, false); err == nil {
		t.Fatal("expected overwrite refusal")
	}
	if n, _ := p.NextName(Train); n != "002" {
		t.Fatalf("NextName after write = %s", n)
	}
	os.WriteFile(p.ExamplePath(Train, "orphan", Input), []byte(`{}`), 0o644)
	p2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	errs := p2.Validate()
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "no matching output") {
		t.Fatalf("Validate = %v", errs)
	}
	exs, _ := p2.Examples(Train)
	le, err := p2.LoadExample(exs[0])
	if err != nil || le.Input["first"] != "Ada" {
		t.Fatalf("LoadExample = %+v, %v", le, err)
	}
}

func TestEncodeValueOrderAndEscaping(t *testing.T) {
	s, _ := ParseSpecYAML([]byte(sampleSpec))
	v, _ := DecodeValue(strings.NewReader(`{"orders":[{"total":2.50,"id":1}],"tags":["a<b"],"age":3,"last":"&","first":"z"}`))
	b, err := EncodeValue(s.Input, v)
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "first": "z",
  "last": "&",
  "age": 3,
  "tags": ["a<b"],
  "orders": [
    {
      "id": 1,
      "total": 2.50
    }
  ]
}
`
	if string(b) != want {
		t.Fatalf("got\n%s\nwant\n%s", b, want)
	}
	back, err := DecodeValue(strings.NewReader(string(b)))
	if err != nil || s.Input.Check(back, "input") != nil {
		t.Fatalf("encoded value does not round-trip: %v", err)
	}
	c1, _ := CanonicalValue(Scalar(Float), mustDecode(t, "2.50"))
	c2, _ := CanonicalValue(Scalar(Float), mustDecode(t, "2.5"))
	if c1 != c2 {
		t.Fatalf("canonical floats differ: %s vs %s", c1, c2)
	}
}

func TestValidateContradiction(t *testing.T) {
	s, _ := ParseSpecYAML([]byte("name: x\ninput: {a: float}\noutput: int\n"))
	p, err := Create(t.TempDir(), s)
	if err != nil {
		t.Fatal(err)
	}
	p.WriteExample(Train, "a", mustDecode(t, `{"a": 1.0}`), mustDecode(t, "1"), false)
	p.WriteExample(Holdout, "b", mustDecode(t, `{"a": 1}`), mustDecode(t, "2"), false)
	p.WriteExample(Train, "c", mustDecode(t, `{"a": 2}`), mustDecode(t, "2"), false)
	errs := p.Validate()
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "contradiction") {
		t.Fatalf("Validate = %v", errs)
	}
}

func mustDecode(t *testing.T, s string) any {
	t.Helper()
	v, err := DecodeValue(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	return v
}
