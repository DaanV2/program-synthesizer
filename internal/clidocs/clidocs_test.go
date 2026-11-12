package clidocs

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCLIReferenceUpToDate fails when docs/cli does not match the current
// command tree. Fix it by running `just docs` and committing the result.
func TestCLIReferenceUpToDate(t *testing.T) {
	diffs, err := Check(filepath.Join("..", "..", "docs", "cli"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) > 0 {
		t.Fatalf("CLI reference is stale; run `just docs` and commit docs/cli\n  %s", strings.Join(diffs, "\n  "))
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	if err := Generate(a); err != nil {
		t.Fatal(err)
	}
	if err := Generate(b); err != nil {
		t.Fatal(err)
	}
	pa, err := readPages(a)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := readPages(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(pa) == 0 {
		t.Fatal("no pages generated")
	}
	for name, s := range pa {
		if pb[name] != s {
			t.Errorf("%s differs between two runs", name)
		}
		if strings.Contains(s, "Auto generated") {
			t.Errorf("%s has a dated auto-gen footer", name)
		}
		if strings.Contains(s, a) {
			t.Errorf("%s contains the output directory path", name)
		}
	}
}

func TestMarkdownText(t *testing.T) {
	in := "Examples live in data/<split>/in/<name>.json, see `a<b>`.\n\n" +
		"Keys:\n" +
		"  name        project name\n" +
		"\n" +
		"  output      'list<int>'\n" +
		"Pairs *.json files.\n"
	want := "Examples live in data/&lt;split&gt;/in/&lt;name&gt;.json, see `a<b>`.\n\n" +
		"Keys:\n\n" +
		"```text\n" +
		"name        project name\n" +
		"\n" +
		"output      'list<int>'\n" +
		"```\n\n" +
		"Pairs \\*.json files."
	if got := markdownText(in); got != want {
		t.Errorf("markdownText:\n got: %q\nwant: %q", got, want)
	}
}
