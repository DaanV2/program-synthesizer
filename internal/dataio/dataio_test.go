package dataio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"synth/internal/project"
)

func scalarSpec() *project.Spec {
	return &project.Spec{
		Name: "t",
		Input: project.RecordOf(
			project.F("x", project.Scalar(project.Int)),
			project.F("rate", project.Scalar(project.Float)),
			project.F("ok", project.Scalar(project.Bool)),
			project.F("label", project.Scalar(project.String)),
		),
		Output: project.Scalar(project.Int),
	}
}

func newProject(t *testing.T, s *project.Spec) *project.Project {
	t.Helper()
	p, err := project.Create(t.TempDir(), s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func joinErrs(errs []error) string {
	var s []string
	for _, e := range errs {
		s = append(s, e.Error())
	}
	return strings.Join(s, "\n")
}

func TestReadJSONL(t *testing.T) {
	src := "\ufeff{\"input\": {\"x\": 1}, \"output\": 2.0}\r\n\n   \n" +
		`{"input": {"x": 2}}` + "\n" +
		`{"input": {"x": 3}, "output": 4, "extra": 1}` + "\n" +
		`not json` + "\n" +
		`{"output": [1, 2], "input": {"x": 5}}` // no trailing newline
	b, err := ReadJSONL(strings.NewReader(src), "ex.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Records) != 2 {
		t.Fatalf("got %d records, want 2: %+v", len(b.Records), b.Records)
	}
	if b.Records[0].Source != "ex.jsonl:1" || b.Records[1].Source != "ex.jsonl:7" {
		t.Errorf("sources = %q, %q", b.Records[0].Source, b.Records[1].Source)
	}
	if got := b.Records[0].Output; got != json.Number("2.0") {
		t.Errorf("output = %#v, want json.Number(2.0)", got)
	}
	got := joinErrs(b.Problems)
	for _, want := range []string{`ex.jsonl:4: missing key "output"`, `ex.jsonl:5: unexpected key "extra"`, "ex.jsonl:6: invalid JSON"} {
		if !strings.Contains(got, want) {
			t.Errorf("problems missing %q:\n%s", want, got)
		}
	}
}

func TestReadCSV(t *testing.T) {
	s := scalarSpec()
	src := "\ufefflabel,x,rate,ok,out\r\n" +
		"hello world , 1, 2.50,true,3\r\n" +
		"\"a,b\",-2,.5,0,4\r\n" +
		"x,abc,1,maybe,\r\n"
	b, err := ReadCSV(strings.NewReader(src), "ex.csv", s.Input, s.Output)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Records) != 2 {
		t.Fatalf("got %d records: %+v; problems: %v", len(b.Records), b.Records, b.Problems)
	}
	want := map[string]any{"label": "hello world ", "x": json.Number("1"), "rate": json.Number("2.50"), "ok": true}
	if !reflect.DeepEqual(b.Records[0].Input, want) {
		t.Errorf("input = %#v, want %#v", b.Records[0].Input, want)
	}
	if b.Records[1].Input.(map[string]any)["rate"] != json.Number("0.5") {
		t.Errorf("rate .5 parsed as %#v", b.Records[1].Input.(map[string]any)["rate"])
	}
	got := joinErrs(b.Problems)
	for _, want := range []string{`ex.csv:4: column "x": invalid int "abc"`, `ex.csv:4: column "ok": invalid bool "maybe"`, `ex.csv:4: column "out": empty value`} {
		if !strings.Contains(got, want) {
			t.Errorf("problems missing %q:\n%s", want, got)
		}
	}
	for _, r := range b.Records {
		if err := s.Input.Check(r.Input, "input"); err != nil {
			t.Errorf("%s: %v", r.Source, err)
		}
	}
}

func TestReadCSVRecordOutput(t *testing.T) {
	in := project.RecordOf(project.F("first", project.Scalar(project.String)))
	out := project.RecordOf(project.F("name", project.Scalar(project.String)), project.F("n", project.Scalar(project.Int)))
	b, err := ReadCSV(strings.NewReader("first,out.n,out.name\nAda,3,ADA\n"), "r.csv", in, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Problems) > 0 || len(b.Records) != 1 {
		t.Fatalf("records %v problems %v", b.Records, b.Problems)
	}
	if err := out.Check(b.Records[0].Output, "output"); err != nil {
		t.Fatal(err)
	}
}

func TestReadCSVHeaderErrors(t *testing.T) {
	s := scalarSpec()
	cases := map[string]string{
		"x,rate,ok,out\n":                 "missing column(s) label",
		"x,rate,ok,label,out,bogus\n":     `unknown column "bogus"`,
		"x,x,rate,ok,label,out\n":         `duplicate column "x"`,
		"":                                "empty CSV file",
		"x,rate,ok,label,out\n1,2,true\n": "wrong number of fields",
	}
	for src, want := range cases {
		_, err := ReadCSV(strings.NewReader(src), "h.csv", s.Input, s.Output)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want %q", src, err, want)
		}
	}
}

func TestCSVColumnsRejectsNonScalar(t *testing.T) {
	in := project.RecordOf(project.F("xs", project.ListOf(project.Scalar(project.Int))))
	if _, err := CSVColumns(in, project.Scalar(project.Int)); err == nil || !strings.Contains(err.Error(), `"xs" is list<int>`) {
		t.Errorf("list input: err = %v", err)
	}
	in = project.RecordOf(project.F("x", project.Scalar(project.Int)))
	if _, err := CSVColumns(in, project.ListOf(project.Scalar(project.Int))); err == nil {
		t.Error("list output: want error")
	}
	nested := project.RecordOf(project.F("r", project.RecordOf(project.F("a", project.Scalar(project.Int)))))
	if _, err := CSVColumns(in, nested); err == nil {
		t.Error("nested record output: want error")
	}
}

func TestReadDirs(t *testing.T) {
	dir := t.TempDir()
	ins, outs := filepath.Join(dir, "in"), filepath.Join(dir, "out")
	writeFile(t, filepath.Join(ins, "a.json"), `{"x": 1}`)
	writeFile(t, filepath.Join(outs, "a.json"), `2`)
	writeFile(t, filepath.Join(ins, "B.JSON"), `{"x": 2}`)
	writeFile(t, filepath.Join(outs, "B.json"), `3`)
	writeFile(t, filepath.Join(ins, "lonely.json"), `{"x": 3}`)
	writeFile(t, filepath.Join(outs, "orphan.json"), `4`)
	writeFile(t, filepath.Join(ins, "bad.json"), `{"x": `)
	writeFile(t, filepath.Join(outs, "bad.json"), `1`)
	writeFile(t, filepath.Join(ins, "notes.txt"), `ignored`)

	b, err := ReadDirs(ins, outs)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range b.Records {
		names = append(names, r.Name)
	}
	if !reflect.DeepEqual(names, []string{"B", "a"}) {
		t.Errorf("names = %v", names)
	}
	if len(b.Warnings) != 2 || !strings.Contains(b.Warnings[0], "lonely.json") || !strings.Contains(b.Warnings[1], "orphan.json") {
		t.Errorf("warnings = %v", b.Warnings)
	}
	if len(b.Problems) != 1 || !strings.Contains(b.Problems[0].Error(), "bad.json") {
		t.Errorf("problems = %v", b.Problems)
	}
	if _, err := ReadDirs(ins, ins); err == nil {
		t.Error("same dir: want error")
	}
}

func TestHoldoutMaskDeterministic(t *testing.T) {
	a := HoldoutMask(20, 0.25, 7)
	b := HoldoutMask(20, 0.25, 7)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed gave different masks")
	}
	if n := countTrue(a); n != 5 {
		t.Fatalf("got %d holdout, want 5", n)
	}
	if reflect.DeepEqual(a, HoldoutMask(20, 0.25, 8)) {
		t.Error("different seeds gave the same mask (possible but very unlikely)")
	}
	if countTrue(HoldoutMask(3, 0, 1)) != 0 || countTrue(HoldoutMask(3, 1, 1)) != 3 || len(HoldoutMask(0, 0.5, 1)) != 0 {
		t.Error("edge cases wrong")
	}
	// Pin the exact output so an accidental algorithm change is caught:
	// users rely on --seed reproducing the same split.
	want := []int{}
	for i, h := range HoldoutMask(10, 0.3, 42) {
		if h {
			want = append(want, i)
		}
	}
	if !reflect.DeepEqual(want, pinnedMask) {
		t.Errorf("HoldoutMask(10, 0.3, 42) = %v, want %v", want, pinnedMask)
	}
}

var pinnedMask = []int{3, 4, 8}

func rec(src, name string, x any, out any) Record {
	return Record{Source: src, Name: name, Input: map[string]any{"x": x}, Output: out}
}

func TestMakePlanNamingAndConflicts(t *testing.T) {
	p := newProject(t, &project.Spec{Name: "t", Input: project.RecordOf(project.F("x", project.Scalar(project.Int))), Output: project.Scalar(project.Int)})
	if _, err := p.WriteExample(project.Train, "001", map[string]any{"x": json.Number("1")}, json.Number("1"), false); err != nil {
		t.Fatal(err)
	}
	if _, err := p.WriteExample(project.Train, "taken", map[string]any{"x": json.Number("1")}, json.Number("1"), false); err != nil {
		t.Fatal(err)
	}
	recs := []Record{
		rec("s1", "", json.Number("1"), json.Number("2")),
		rec("s2", "003", json.Number("1"), json.Number("2")), // kept; generator must skip it
		rec("s3", "", json.Number("1"), json.Number("2")),
		rec("s4", "bad name!", json.Number("1"), json.Number("2")),
	}
	pl, err := MakePlan(p, recs, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Problems) > 0 {
		t.Fatal(pl.Problems)
	}
	var names []string
	for _, e := range pl.Examples {
		names = append(names, string(e.Split)+"/"+e.Name)
	}
	if want := []string{"train/002", "train/003", "train/004", "train/005"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
	if len(pl.Warnings) != 1 {
		t.Errorf("warnings = %v", pl.Warnings)
	}

	// Type errors, duplicates and existing files are all reported together.
	recs = []Record{
		rec("t1", "dup", json.Number("1"), json.Number("2")),
		rec("t2", "DUP", json.Number("1"), json.Number("2")),
		rec("t3", "", "one", json.Number("2")),
		rec("t4", "", json.Number("1"), json.Number("2.5")),
		rec("t5", "taken", json.Number("1"), json.Number("2")),
	}
	pl, err = MakePlan(p, recs, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := joinErrs(pl.Problems)
	for _, want := range []string{`t2: example name "DUP" in train is also used by t1`, "t3: input.x: expected int", "t4: output: expected int", "t5: example train/taken already exists"} {
		if !strings.Contains(got, want) {
			t.Errorf("problems missing %q:\n%s", want, got)
		}
	}
	if err := Apply(p, pl, false); err == nil {
		t.Error("Apply with problems: want error")
	}
	if exs, _ := p.Examples(project.Train); len(exs) != 2 {
		t.Errorf("Apply with problems wrote files: %v", exs)
	}

	// --overwrite allows replacing existing examples.
	pl, _ = MakePlan(p, []Record{rec("u1", "taken", json.Number("9"), json.Number("9"))}, Options{Overwrite: true})
	if len(pl.Problems) > 0 {
		t.Fatal(pl.Problems)
	}
}

func TestMakePlanHoldoutAndApply(t *testing.T) {
	p := newProject(t, &project.Spec{Name: "t", Input: project.RecordOf(project.F("x", project.Scalar(project.Int))), Output: project.Scalar(project.Int)})
	var recs []Record
	for i := range 10 {
		recs = append(recs, rec("r", "", json.Number(string(rune('0'+i))), json.Number("0")))
	}
	pl, err := MakePlan(p, recs, Options{HoldoutFraction: 0.3, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	c := pl.Counts()
	if c[project.Train] != 7 || c[project.Holdout] != 3 {
		t.Fatalf("counts = %v", c)
	}
	if err := Apply(p, pl, false); err != nil {
		t.Fatal(err)
	}
	ho, _ := p.Examples(project.Holdout)
	if len(ho) != 3 || ho[0].Name != "001" {
		t.Errorf("holdout examples = %v", ho)
	}
	if errs := p.Validate(); len(errs) > 0 {
		t.Error(errs)
	}
	if _, err := MakePlan(p, recs, Options{Split: project.Holdout, HoldoutFraction: 0.5}); err == nil {
		t.Error("holdout fraction with split holdout: want error")
	}
	if _, err := MakePlan(p, recs, Options{HoldoutFraction: 1.5}); err == nil {
		t.Error("fraction > 1: want error")
	}
}

func TestPreview(t *testing.T) {
	typ := project.RecordOf(project.F("b", project.Scalar(project.String)), project.F("a", project.ListOf(project.Scalar(project.Int))))
	v := map[string]any{"a": []any{json.Number("1"), json.Number("2")}, "b": "<x>", "zz": true}
	if got, want := Preview(v, typ, 0), `{"b":"<x>","a":[1,2],"zz":true}`; got != want {
		t.Errorf("Preview = %s, want %s", got, want)
	}
	if got := Preview("héllo wörld", project.Scalar(project.String), 6); got != `"héll…` {
		t.Errorf("truncated = %q", got)
	}
}

func TestSortProblems(t *testing.T) {
	errs := []error{
		errAt("b.jsonl:10", "x"),
		errAt("b.jsonl:2", "y"),
		errAt("a.csv:3", "z"),
		errAt("b.jsonl:2", "w"),
	}
	SortProblems(errs)
	if got, want := joinErrs(errs), "a.csv:3: z\nb.jsonl:2: y\nb.jsonl:2: w\nb.jsonl:10: x"; got != want {
		t.Errorf("sorted =\n%s\nwant\n%s", got, want)
	}
	if n := CountSources(errs); n != 3 {
		t.Errorf("CountSources = %d, want 3", n)
	}
}
