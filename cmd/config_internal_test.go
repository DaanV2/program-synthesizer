package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DaanV2/program-synthesizer/internal/project"
)

// newTestProject writes a project with one valid training example.
func newTestProject(t *testing.T, spec string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, project.SpecYAML), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"data/train/in/001.json":  `{"first": "Ada", "age": 36}`,
		"data/train/out/001.json": `{"fullName": "Ada", "isAdult": true}`,
	} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

const cliSpec = `# my project
name: demo
input:
  first: string # given name
  age: int
output:
  fullName: string
  isAdult: bool
options:
  targets: [go]
`

func TestConfigListGet(t *testing.T) {
	dir := newTestProject(t, cliSpec)
	out, err := run(t, "-C", dir, "config", "list")
	if err != nil {
		t.Fatal(err)
	}
	want := "name = demo\ninput.first = string\ninput.age = int\noutput.fullName = string\noutput.isAdult = bool\noptions.targets = go\n"
	if out != want {
		t.Fatalf("list:\n%s\nwant\n%s", out, want)
	}
	out, err = run(t, "-C", dir, "config", "get", "input")
	if err != nil || out != "{first: string, age: int}\n" {
		t.Fatalf("get input: %q, %v", out, err)
	}
	if _, err := run(t, "-C", dir, "config", "get", "description"); err == nil || !strings.Contains(err.Error(), "no value set for description") {
		t.Fatalf("get unset: %v", err)
	}
	if _, err := run(t, "-C", dir, "config", "get", "bogus"); err == nil || !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("get bogus: %v", err)
	}
	// Project discovery from a subdirectory.
	sub := filepath.Join(dir, "data", "train")
	if out, err := run(t, "-C", sub, "config", "get", "name"); err != nil || out != "demo\n" {
		t.Fatalf("get from subdir: %q, %v", out, err)
	}
}

func TestConfigSetUnsetMove(t *testing.T) {
	dir := newTestProject(t, cliSpec)
	steps := [][]string{
		{"set", "description", "Join names"},
		{"set", "options.targets", "go,rust"},
		{"set", "options.float_tolerance", "0.01"},
		{"move", "input.age", "--before", "first"},
		{"set", "output.initials", "string"}, // breaks the example (missing field): warns, still saves
		{"unset", "output.initials"},
		{"unset", "options.float_tolerance"},
	}
	for _, st := range steps {
		if _, err := run(t, append([]string{"-C", dir, "config"}, st...)...); err != nil {
			t.Fatalf("%v: %v", st, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, project.SpecYAML))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{"# my project", "first: string # given name", "description: Join names", "targets: [go, rust]"} {
		if !strings.Contains(got, want) {
			t.Errorf("spec.yaml lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "initials") || strings.Contains(got, "float_tolerance") {
		t.Errorf("unset keys still present:\n%s", got)
	}
	if strings.Index(got, "age:") > strings.Index(got, "first:") {
		t.Errorf("move did not reorder:\n%s", got)
	}
}

func TestConfigStrict(t *testing.T) {
	dir := newTestProject(t, cliSpec)
	specPath := filepath.Join(dir, project.SpecYAML)
	before, _ := os.ReadFile(specPath)

	_, err := run(t, "-C", dir, "config", "set", "input.age", "string", "--strict")
	if err == nil || !strings.Contains(err.Error(), "not changed: 1 example(s) would no longer conform") {
		t.Fatalf("strict set: %v", err)
	}
	if after, _ := os.ReadFile(specPath); !bytes.Equal(after, before) {
		t.Fatal("strict refusal changed spec.yaml")
	}
	// A change that keeps the examples valid passes --strict.
	if _, err := run(t, "-C", dir, "config", "set", "input.age", "float", "--strict"); err != nil {
		t.Fatalf("compatible strict change: %v", err)
	}
	// Problems that existed before the change do not block --strict.
	writeFile(t, filepath.Join(dir, "data", "train", "in", "002.json"), `{"first": 1, "age": 2}`)
	writeFile(t, filepath.Join(dir, "data", "train", "out", "002.json"), `{"fullName": "x", "isAdult": true}`)
	if _, err := run(t, "-C", dir, "config", "set", "description", "x", "--strict"); err != nil {
		t.Fatalf("unrelated strict change: %v", err)
	}
	if _, err := run(t, "-C", dir, "config", "unset", "output.isAdult", "--strict"); err == nil || !strings.Contains(err.Error(), "not changed") {
		t.Fatalf("strict unset: %v", err)
	}
}

func TestConfigErrors(t *testing.T) {
	dir := newTestProject(t, cliSpec)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"set", "input", "int"}, "input must be a record"},
		{[]string{"set", "input.age", "integer"}, "unknown type"},
		{[]string{"set", "options.targets", "java"}, "unknown target"},
		{[]string{"set", "foo", "bar"}, "unknown key"},
		{[]string{"unset", "name"}, "cannot unset name"},
		{[]string{"unset", "input.nope"}, `no field "nope"`},
		{[]string{"move", "input.age"}, "exactly one of --before or --after"},
		{[]string{"move", "input.age", "--before", "x", "--after", "y"}, "exactly one of --before or --after"},
		{[]string{"set", "name"}, "accepts 2 arg(s)"},
	}
	for _, tc := range cases {
		if _, err := run(t, append([]string{"-C", dir, "config"}, tc.args...)...); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: got %v, want error containing %q", tc.args, err, tc.want)
		}
	}
	if _, err := run(t, "-C", t.TempDir(), "config", "list"); err == nil || !strings.Contains(err.Error(), "synth init") {
		t.Errorf("no project: %v", err)
	}
}

func TestConfigJSONSpec(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, project.SpecJSON), `{"name": "j", "input": {"a": "int"}, "output": "int"}`)
	if _, err := run(t, "-C", dir, "config", "set", "input.b", "[{id: int}]"); err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := project.FormatType(p.Spec.Input); got != "{a: int, b: [{id: int}]}" {
		t.Fatalf("input = %s", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
