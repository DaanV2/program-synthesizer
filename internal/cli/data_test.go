package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"synth/internal/project"
)

func newDataTestProject(t *testing.T) *project.Project {
	t.Helper()
	s := &project.Spec{
		Name:   "sum",
		Input:  project.RecordOf(project.F("a", project.Scalar(project.Int)), project.F("b", project.Scalar(project.Int))),
		Output: project.Scalar(project.Int),
	}
	p, err := project.Create(t.TempDir(), s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// runCLI executes the root command against project dir and returns stdout.
func runCLI(t *testing.T, dir, stdin string, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(append([]string{"--log-level", "error", "-C", dir}, args...))
	err := cmd.Execute()
	return out.String(), err
}

func TestDataAddListValidate(t *testing.T) {
	p := newDataTestProject(t)
	if _, err := runCLI(t, p.Dir, "", "data", "add", "--in", `{"a": 1, "b": 2}`, "--out", "3"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, p.Dir, `{"a": 5, "b": 5}`, "data", "add", "--input", "-", "--out", "10", "--split", "holdout"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, p.Dir, "", "data", "add", "--in", `{"a": 1.5, "b": 2}`, "--out", "3"); err == nil || !strings.Contains(err.Error(), "input.a: expected int") {
		t.Errorf("bad type: err = %v", err)
	}
	if _, err := runCLI(t, p.Dir, "", "data", "add", "--in", `{"a": 1, "b": 2}`, "--out", "3", "--name", "001"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("existing name: err = %v", err)
	}

	out, err := runCLI(t, p.Dir, "", "data", "list")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`{"a":1,"b":2}`, "holdout", "10"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q:\n%s", want, out)
		}
	}

	out, err = runCLI(t, p.Dir, "", "data", "validate")
	if err != nil || !strings.Contains(out, "2 example(s): 1 train, 1 holdout") {
		t.Errorf("validate: err = %v, out = %s", err, out)
	}
	if err := os.WriteFile(p.ExamplePath(project.Train, "001", project.Output), []byte(`"three"`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = runCLI(t, p.Dir, "", "data", "validate")
	if err == nil || !strings.Contains(out, "output: expected int, got string") {
		t.Errorf("validate broken: err = %v, out = %s", err, out)
	}
}

func TestDataImportAllOrNothing(t *testing.T) {
	p := newDataTestProject(t)
	src := filepath.Join(t.TempDir(), "ex.csv")
	if err := os.WriteFile(src, []byte("a,b,out\n1,2,3\n4,5,x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, p.Dir, "", "data", "import", src); err == nil || !strings.Contains(err.Error(), "nothing was written") {
		t.Fatalf("bad csv: err = %v", err)
	}
	if exs, _ := p.Examples(project.Train); len(exs) != 0 {
		t.Fatalf("failed import wrote %d examples", len(exs))
	}

	jsonl := "{\"input\": {\"a\": 1, \"b\": 2}, \"output\": 3}\n{\"input\": {\"a\": 2, \"b\": 2}, \"output\": 4}\n"
	out, err := runCLI(t, p.Dir, jsonl, "data", "import", "-", "--format", "jsonl", "--dry-run")
	if err != nil || !strings.Contains(out, "would import 2 example(s)") {
		t.Fatalf("dry run: err = %v, out = %s", err, out)
	}
	if exs, _ := p.Examples(project.Train); len(exs) != 0 {
		t.Fatal("dry run wrote files")
	}
	out, err = runCLI(t, p.Dir, jsonl, "data", "import", "-", "--format", "jsonl", "--holdout", "0.5")
	if err != nil || !strings.Contains(out, "imported 2 example(s): 1 train, 1 holdout") {
		t.Fatalf("import: err = %v, out = %s", err, out)
	}

	for _, args := range [][]string{
		{"data", "import"},
		{"data", "import", src, "--inputs", "x", "--outputs", "y"},
		{"data", "import", "--inputs", "x"},
		{"data", "import", src, "--holdout", "0.2", "--split", "holdout"},
		{"data", "import", src, "--seed", "3"},
		{"data", "import", "-"},
	} {
		if _, err := runCLI(t, p.Dir, "", args...); err == nil {
			t.Errorf("%v: want error", args)
		}
	}
}
