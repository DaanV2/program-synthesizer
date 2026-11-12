package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"synth/internal/project"
)

func TestParseFieldList(t *testing.T) {
	ok := map[string]string{ // input -> FormatType of the record
		"first=string,age=int":                    "{first: string, age: int}",
		" first = string , age=int, ":             "{first: string, age: int}",
		"tags=list<string>":                       "{tags: list<string>}",
		"pos={x: int, y: int},n=int":              "{pos: {x: int, y: int}, n: int}",
		"orders=[{id: int, total: float}]":        "{orders: [{id: int, total: float}]}",
		"orders=list<{id: int}>":                  "{orders: [{id: int}]}",
		"{first: string, tags: list<string>}":     "{first: string, tags: list<string>}",
		"{a: {b: int}, c: [{d: bool, e: float}]}": "{a: {b: int}, c: [{d: bool, e: float}]}",
	}
	for in, want := range ok {
		fields, err := parseFieldList(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got := project.FormatType(project.RecordOf(fields...)); got != want {
			t.Errorf("%q: got %s, want %s", in, got, want)
		}
	}
	bad := map[string]string{
		"":                 "empty field list",
		",":                "empty field list",
		"first":            "want name=type",
		"first=strin":      "unknown type",
		"1x=int":           "invalid name",
		"a=int,a=bool":     "already used",
		"a={x: int":        "unclosed bracket",
		"a=int}":           "unbalanced",
		"{a: int, a: int}": "",
		"{x: int}extra":    "unexpected",
	}
	for in, want := range bad {
		if _, err := parseFieldList(in); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want error containing %q", in, err, want)
		}
	}
}

func TestParseInputAndOutputFlags(t *testing.T) {
	params, err := parseInputFlags([]string{"first=string", "{age: int, tags: list<string>}"})
	if err != nil {
		t.Fatal(err)
	}
	if got := project.FormatType(project.RecordOf(params...)); got != "{first: string, age: int, tags: list<string>}" {
		t.Fatalf("got %s", got)
	}
	if _, err := parseInputFlags([]string{"a=int", "a=bool"}); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("duplicate across flags: %v", err)
	}

	outs := map[string]string{
		"int":                          "int",
		"list<string>":                 "list<string>",
		"[{id: int}]":                  "[{id: int}]",
		"{fullName: string}":           "{fullName: string}",
		"fullName=string,isAdult=bool": "{fullName: string, isAdult: bool}",
		"pos={x: int, y: int}":         "{pos: {x: int, y: int}}",
	}
	for in, want := range outs {
		typ, err := parseOutputFlag(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got := project.FormatType(typ); got != want {
			t.Errorf("%q: got %s, want %s", in, got, want)
		}
	}
	for _, in := range []string{"", "integer", "a=int,b", "{x: int"} {
		if _, err := parseOutputFlag(in); err == nil || !strings.HasPrefix(err.Error(), "invalid --output") {
			t.Errorf("%q: got %v, want --output error", in, err)
		}
	}
}

func TestDraftBuild(t *testing.T) {
	d := specDraft{
		Name:    " demo ",
		Params:  []project.Field{project.F("x", project.Scalar(project.Int))},
		Output:  project.Scalar(project.Int),
		Targets: []string{"go"},
	}
	s, err := d.Build()
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "demo" || s.Input.Kind != project.Record || len(s.Input.Fields) != 1 {
		t.Fatalf("bad spec %+v", s)
	}
	bad := []struct {
		mut  func(*specDraft)
		want string
	}{
		{func(d *specDraft) { d.Name = " " }, "name is required"},
		{func(d *specDraft) { d.Params = nil }, "at least one input parameter"},
		{func(d *specDraft) { d.Params = append(d.Params, d.Params[0]) }, "duplicate input parameter"},
		{func(d *specDraft) { d.Params = []project.Field{project.F("a b", project.Scalar(project.Int))} }, "invalid name"},
		{func(d *specDraft) { d.Output = nil }, "output type is required"},
		{func(d *specDraft) { d.Targets = []string{"java"} }, "unknown target"},
		{func(d *specDraft) { d.FloatTolerance = -1 }, "float_tolerance"},
	}
	for _, tc := range bad {
		dd := d
		dd.Params = append([]project.Field(nil), d.Params...)
		tc.mut(&dd)
		if _, err := dd.Build(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("got %v, want error containing %q", err, tc.want)
		}
	}
}

func TestDefaultProjectName(t *testing.T) {
	if got := defaultProjectName(filepath.Join(t.TempDir(), "my-proj")); got != "my-proj" {
		t.Fatalf("got %q", got)
	}
	if got := defaultProjectName(string(filepath.Separator)); got != "" {
		t.Fatalf("root dir: got %q", got)
	}
}

// TestWizardAccessible drives the real huh forms in accessible mode, which
// reads answers line by line instead of needing a terminal.
func TestWizardAccessible(t *testing.T) {
	script := strings.Join([]string{
		"",                   // project name: keep default
		"Adds things",        // description
		"x",                  // parameter 1
		"1",                  // int
		"x",                  // duplicate name: rejected, asked again
		"pos",                // parameter 2
		"8",                  // custom…
		"{a: int",            // invalid: asked again
		"{a: int, b: float}", // record type
		"",                   // done with parameters
		"2",                  // output: record
		"sum",                // output field 1
		"2",                  // float
		"",                   // done with output fields
		"1e-6",               // float tolerance (output has a float)
		"2",                  // toggle rust
		"0",                  // finish target selection
		"y",                  // create
	}, "\n") + "\n"
	var out bytes.Buffer
	w := &initWizard{ctx: context.Background(), in: iotest.OneByteReader(strings.NewReader(script)), out: &out, accessible: true}
	d := &specDraft{Name: "demo"}
	ok, err := w.fill(d, "/tmp/demo")
	if err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if !ok {
		t.Fatalf("not confirmed\n%s", out.String())
	}
	s, err := d.Build()
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		"name":                    "demo",
		"description":             "Adds things",
		"input":                   "{x: int, pos: {a: int, b: float}}",
		"output":                  "{sum: float}",
		"options.float_tolerance": "1e-06",
		"options.targets":         "go,rust",
	}
	for key, want := range checks {
		if got, err := s.Get(key); err != nil || got != want {
			t.Errorf("%s = %q, %v; want %q\n%s", key, got, err, want, out.String())
		}
	}
	for _, want := range []string{`"x" is already used`, "spec.yaml"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("wizard output lacks %q", want)
		}
	}
}

func TestWizardAccessibleDecline(t *testing.T) {
	script := "\n\nn\n1\n\n1\n1\n0\nn\n" // name, desc, param n:int, done, single output int, targets, decline
	w := &initWizard{ctx: context.Background(), in: iotest.OneByteReader(strings.NewReader(script)), out: &bytes.Buffer{}, accessible: true}
	d := &specDraft{Name: "demo"}
	ok, err := w.fill(d, "x")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected the user to decline")
	}
	if got := project.FormatType(d.Output); got != "int" {
		t.Fatalf("output = %s", got)
	}
}

// run executes the root command with args and returns stdout.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader("")) // not a terminal
	cmd.SetArgs(append([]string{"--log-level", "error"}, args...))
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

func TestInitNonInteractive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "full-name")
	out, err := run(t, "init", dir, "--input", "first=string,last=string", "--input", "age=int",
		"--output", "fullName=string,isAdult=bool", "--targets", "go,rust", "--description", "Join")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `Created project "full-name"`) || !strings.Contains(out, "synth data add") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	p, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := project.FormatType(p.Spec.Input); got != "{first: string, last: string, age: int}" {
		t.Fatalf("input = %s", got)
	}
	if strings.Join(p.Spec.Options.Targets, ",") != "go,rust" || p.Spec.Description != "Join" {
		t.Fatalf("options = %+v", p.Spec)
	}
	for _, sp := range project.Splits {
		if _, err := os.Stat(p.SideDir(sp, project.Output)); err != nil {
			t.Fatal(err)
		}
	}

	// Second init in the same place is refused.
	if _, err := run(t, "init", dir, "-y", "--input", "a=int", "--output", "int"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("re-init: %v", err)
	}
}

func TestInitDefaultsAndErrors(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	if _, err := run(t, "-C", dir, "init", "--yes", "--input", "{x: int}", "--output", "int", "--float-tolerance", "0.5"); err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Spec.Name != "proj" || strings.Join(p.Spec.Options.Targets, ",") != "go" || p.Spec.Options.FloatTolerance != 0.5 {
		t.Fatalf("defaults not applied: %+v", p.Spec)
	}

	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--output", "int"}, "missing --input: required when stdin is not a terminal"},
		{[]string{"-y", "--input", "a=int"}, "missing --output: required with --yes"},
		{[]string{"-y"}, "missing --input and --output: required with --yes"},
		{[]string{"-y", "--input", "a=nope", "--output", "int"}, "unknown type"},
		{[]string{"-y", "--input", "a=int", "--output", "int", "--targets", "java"}, "--targets: unknown target"},
		{[]string{"-y", "--input", "a=int", "--output", "int", "--float-tolerance", "-1"}, ">= 0"},
	}
	for _, tc := range cases {
		args := append([]string{"init", filepath.Join(t.TempDir(), "x")}, tc.args...)
		if _, err := run(t, args...); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: got %v, want error containing %q", tc.args, err, tc.want)
		}
	}
}

func TestWizardState(t *testing.T) {
	st := newWizardState(&specDraft{Name: "demo"})
	if len(st.Inputs) != maxSlots || st.Targets[0] != "go" || !st.Confirm {
		t.Fatalf("bad initial state: %+v", st)
	}
	st.Inputs[0] = typeSlot{Name: "x", Choice: "int"}
	st.Inputs[1] = typeSlot{Name: "pos", Choice: customType, Custom: "{a: int, b: float}"}
	st.Inputs[3] = typeSlot{Name: "ignored", Choice: "int"} // after an empty slot
	if !slotVisible(st.Inputs, 2) || slotVisible(st.Inputs, 3) {
		t.Fatal("slot visibility wrong")
	}
	st.OutputKind = outputRecord
	st.OutputFields[0] = typeSlot{Name: "sum", Choice: "float"}
	st.Output = typeSlot{Choice: "bool"} // ignored for a record output
	st.Tolerance = "0.5"
	st.Targets = []string{"rust"}

	d := &specDraft{}
	if err := st.apply(d); err != nil {
		t.Fatal(err)
	}
	s, err := d.Build()
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"input":                   "{x: int, pos: {a: int, b: float}}",
		"output":                  "{sum: float}",
		"options.float_tolerance": "0.5",
		"options.targets":         "rust",
	} {
		if got, _ := s.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if p := st.preview(specDraft{}); !strings.Contains(p, "sum: float") {
		t.Errorf("preview:\n%s", p)
	}

	// Tolerance is dropped when the (single) output has no float.
	st.OutputKind = outputSingle
	d = &specDraft{}
	if err := st.apply(d); err != nil || d.FloatTolerance != 0 || project.FormatType(d.Output) != "bool" {
		t.Fatalf("single output: %v %+v", err, d)
	}

	// Flag values win over the wizard's slots.
	d = &specDraft{Params: []project.Field{project.F("q", project.Scalar(project.String))}, Output: project.Scalar(project.Int)}
	if err := st.apply(d); err != nil || project.FormatType(project.RecordOf(d.Params...)) != "{q: string}" || project.FormatType(d.Output) != "int" {
		t.Fatalf("flags overridden: %v %+v", err, d)
	}

	// Errors surface in apply and in the preview.
	st.Inputs[1].Custom = "{a: nope}"
	if err := st.apply(&specDraft{}); err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("bad custom type: %v", err)
	}
	if p := st.preview(specDraft{}); !strings.HasPrefix(p, "Cannot build the spec yet") {
		t.Fatalf("preview with error: %s", p)
	}
	st.Inputs[1] = typeSlot{Name: "x", Choice: "int"}
	if err := st.apply(&specDraft{}); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("duplicate: %v", err)
	}
	st.OutputKind = outputRecord
	st.OutputFields[0].Name = ""
	st.Inputs[1].Name = "y"
	if err := st.apply(&specDraft{}); err == nil || !strings.Contains(err.Error(), "at least one field") {
		t.Fatalf("empty record output: %v", err)
	}
}
