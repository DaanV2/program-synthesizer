// Package clidocs generates the Markdown CLI reference in docs/cli from the
// cobra command tree built by cli.NewRootCmd, so the reference cannot drift
// from the real CLI. Run it with `just docs` (cmd/gendocs); the test in this
// package fails when the committed pages are stale.
package clidocs

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"synth/internal/cli"
)

// header is written at the top of every generated page. It is an HTML
// comment, so GitHub does not render it.
const header = "<!-- Generated from the cobra command tree by `just docs`. Do not edit; change the help text in internal/cli instead. -->\n\n"

// Generate writes one Markdown page per command into dir, replacing any
// *.md files already there (so pages of removed commands disappear). dir
// is created if missing.
//
// The output is deterministic: no dates, commands in name order, and links
// between pages are bare file names (relative, so they work on GitHub).
// fang adds --version and the man/completion commands only at Execute
// time, so they are not part of the tree documented here.
func Generate(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	old, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return err
	}
	for _, f := range old {
		if err := os.Remove(f); err != nil {
			return err
		}
	}

	root := cli.NewRootCmd()
	root.DisableAutoGenTag = true
	prepare(root)
	return doc.GenMarkdownTreeCustom(root, dir,
		func(string) string { return header },
		func(link string) string { return link },
	)
}

// prepare rewrites the help text of c and its subcommands so it reads well
// as Markdown: see markdownText. It only touches this private tree.
func prepare(c *cobra.Command) {
	c.DisableAutoGenTag = true
	c.Short = escapeProse(c.Short)
	long := c.Long
	if len(c.Aliases) > 0 {
		names := make([]string, len(c.Aliases))
		for i, a := range c.Aliases {
			names[i] = "`" + c.Parent().CommandPath() + " " + a + "`"
		}
		long += "\n\nAlias: " + strings.Join(names, ", ") + "."
	}
	c.Long = markdownText(long)
	for _, sub := range c.Commands() {
		prepare(sub)
	}
}

// markdownText turns terminal help text into Markdown that renders the
// same way on GitHub:
//
//   - runs of indented lines (tables of keys, flag forms) become fenced
//     code blocks, dedented, so their alignment survives;
//   - in the remaining prose, <, > and * outside `code spans` are escaped,
//     so placeholders such as data/<split>/in/<name>.json are not taken for
//     HTML tags or emphasis.
//
// Unindented lines of one paragraph are left as they are; Markdown joins
// them, which is what the terminal reader sees as a paragraph too.
func markdownText(s string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	var out []string
	blankBefore := func() {
		if len(out) > 0 && out[len(out)-1] != "" {
			out = append(out, "")
		}
	}
	for i := 0; i < len(lines); {
		if !indented(lines[i]) {
			out = append(out, escapeProse(lines[i]))
			i++
			continue
		}
		// The block runs to the last indented line before the next
		// unindented, non-blank one; blank lines inside it are kept.
		end := i
		for j := i; j < len(lines) && (indented(lines[j]) || strings.TrimSpace(lines[j]) == ""); j++ {
			if indented(lines[j]) {
				end = j
			}
		}
		block := dedent(lines[i : end+1])
		blankBefore()
		out = append(out, "```text")
		out = append(out, block...)
		out = append(out, "```")
		if end+1 < len(lines) && strings.TrimSpace(lines[end+1]) != "" {
			out = append(out, "")
		}
		i = end + 1
	}
	return strings.Join(out, "\n")
}

func indented(line string) bool {
	return strings.TrimSpace(line) != "" && (line[0] == ' ' || line[0] == '\t')
}

// dedent removes the indentation common to all non-blank lines.
func dedent(lines []string) []string {
	indent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if len(l) >= indent {
			out[i] = strings.TrimRight(l[indent:], " \t")
		}
	}
	return out
}

// escapeProse escapes Markdown/HTML-significant characters outside
// backtick code spans.
func escapeProse(s string) string {
	var b strings.Builder
	inCode := false
	for _, r := range s {
		switch {
		case r == '`':
			inCode = !inCode
			b.WriteRune(r)
		case inCode:
			b.WriteRune(r)
		case r == '<':
			b.WriteString("&lt;")
		case r == '>':
			b.WriteString("&gt;")
		case r == '*':
			b.WriteString(`\*`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Check regenerates the reference into a temporary directory and compares
// it with dir. It returns a list of differences, empty when dir is up to
// date. Line endings are normalised, so a CRLF checkout still passes.
func Check(dir string) ([]string, error) {
	tmp, err := os.MkdirTemp("", "synth-cli-docs-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := Generate(tmp); err != nil {
		return nil, err
	}
	want, err := readPages(tmp)
	if err != nil {
		return nil, err
	}
	have, err := readPages(dir)
	if err != nil {
		return nil, err
	}
	var diffs []string
	for name, w := range want {
		h, ok := have[name]
		switch {
		case !ok:
			diffs = append(diffs, name+": missing")
		case h != w:
			diffs = append(diffs, name+": differs"+firstDiff(h, w))
		}
	}
	for name := range have {
		if _, ok := want[name]; !ok {
			diffs = append(diffs, name+": no longer generated (command removed or renamed?)")
		}
	}
	slices.Sort(diffs)
	return diffs, nil
}

func readPages(dir string) (map[string]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}
	pages := map[string]string{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		pages[filepath.Base(f)] = strings.ReplaceAll(string(b), "\r\n", "\n")
	}
	return pages, nil
}

// firstDiff describes the first line where have and want differ.
func firstDiff(have, want string) string {
	h, w := strings.Split(have, "\n"), strings.Split(want, "\n")
	for i := range max(len(h), len(w)) {
		var hl, wl string
		if i < len(h) {
			hl = h[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if hl != wl {
			return fmt.Sprintf(" at line %d:\n    committed: %q\n    generated: %q", i+1, hl, wl)
		}
	}
	return ""
}
