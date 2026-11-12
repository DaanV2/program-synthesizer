// Package project handles a synthesis project on disk:
//
//	<project>/
//	  spec.yaml              (or spec.json)
//	  data/
//	    train/
//	      in/<name>.json     input value for example <name>
//	      out/<name>.json    expected output for example <name>
//	    holdout/
//	      in/<name>.json
//	      out/<name>.json
//
// An example is the pair of files with the same <name> under in/ and out/
// of one split. Every input file must conform to spec.input and every
// output file to spec.output.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	SpecYAML = "spec.yaml"
	SpecJSON = "spec.json"
	DataDir  = "data"
	InDir    = "in"
	OutDir   = "out"
	Ext      = ".json"
)

// Split is a named subset of the examples.
type Split string

const (
	Train   Split = "train"
	Holdout Split = "holdout"
)

// Splits lists every split, in order.
var Splits = []Split{Train, Holdout}

// ParseSplit converts a string to a Split.
func ParseSplit(s string) (Split, error) {
	for _, sp := range Splits {
		if string(sp) == s {
			return sp, nil
		}
	}
	return "", fmt.Errorf("unknown split %q (want train or holdout)", s)
}

// Side is one half of an example.
type Side string

const (
	Input  Side = InDir
	Output Side = OutDir
)

// Project is a loaded project directory.
type Project struct {
	Dir      string
	SpecPath string // spec.yaml or spec.json; Save writes back to the same format
	Spec     *Spec
}

// ErrNoProject is returned by Find when no spec file is found.
var ErrNoProject = errors.New("no spec.yaml or spec.json found (run `synth init` to create a project)")

// Find walks up from start until it finds a directory containing a spec file.
func Find(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if specPath(dir) != "" {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNoProject
		}
		dir = parent
	}
}

func specPath(dir string) string {
	for _, name := range []string{SpecYAML, SpecJSON} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Load reads the project in dir. dir must contain the spec file itself;
// use Find first to search parent directories.
func Load(dir string) (*Project, error) {
	p := specPath(dir)
	if p == "" {
		return nil, fmt.Errorf("%s: %w", dir, ErrNoProject)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var s *Spec
	if strings.HasSuffix(p, ".json") {
		s, err = ParseSpecJSON(b)
	} else {
		s, err = ParseSpecYAML(b)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return &Project{Dir: dir, SpecPath: p, Spec: s}, nil
}

// Create makes a new project in dir (which may already exist but must not
// contain a spec file), writing spec.yaml and the data directories.
func Create(dir string, s *Spec) (*Project, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if existing := specPath(dir); existing != "" {
		return nil, fmt.Errorf("%s already exists", existing)
	}
	p := &Project{Dir: dir, SpecPath: filepath.Join(dir, SpecYAML), Spec: s}
	for _, sp := range Splits {
		for _, side := range []Side{Input, Output} {
			if err := os.MkdirAll(p.SideDir(sp, side), 0o755); err != nil {
				return nil, err
			}
		}
	}
	return p, p.Save()
}

// Save validates the spec and writes it back to SpecPath.
func (p *Project) Save() error {
	if err := p.Spec.Validate(); err != nil {
		return err
	}
	var b []byte
	var err error
	if strings.HasSuffix(p.SpecPath, ".json") {
		b, err = p.Spec.EncodeJSON()
	} else {
		b, err = p.Spec.EncodeYAML()
	}
	if err != nil {
		return err
	}
	return os.WriteFile(p.SpecPath, b, 0o644)
}

// SideDir is data/<split>/<in|out>.
func (p *Project) SideDir(sp Split, side Side) string {
	return filepath.Join(p.Dir, DataDir, string(sp), string(side))
}

// Example is one input/output pair.
type Example struct {
	Split      Split
	Name       string
	InputPath  string
	OutputPath string
}

// PairError reports an input without an output or vice versa.
type PairError struct {
	Path    string
	Missing string
}

func (e *PairError) Error() string { return fmt.Sprintf("%s: no matching %s", e.Path, e.Missing) }

// Examples lists the complete pairs in a split, sorted by name. Unpaired
// files are returned as PairErrors alongside the pairs that are complete.
func (p *Project) Examples(sp Split) ([]Example, []error) {
	ins, err1 := listNames(p.SideDir(sp, Input))
	outs, err2 := listNames(p.SideDir(sp, Output))
	if err := errors.Join(err1, err2); err != nil {
		return nil, []error{err}
	}
	var exs []Example
	var errs []error
	for name := range ins {
		if !outs[name] {
			errs = append(errs, &PairError{Path: p.ExamplePath(sp, name, Input), Missing: "output in " + string(Output) + "/"})
			continue
		}
		exs = append(exs, Example{Split: sp, Name: name, InputPath: p.ExamplePath(sp, name, Input), OutputPath: p.ExamplePath(sp, name, Output)})
	}
	for name := range outs {
		if !ins[name] {
			errs = append(errs, &PairError{Path: p.ExamplePath(sp, name, Output), Missing: "input in " + string(Input) + "/"})
		}
	}
	sort.Slice(exs, func(i, j int) bool { return exs[i].Name < exs[j].Name })
	sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
	return exs, errs
}

// ExamplePath is data/<split>/<in|out>/<name>.json.
func (p *Project) ExamplePath(sp Split, name string, side Side) string {
	return filepath.Join(p.SideDir(sp, side), name+Ext)
}

func listNames(dir string) (map[string]bool, error) {
	names := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), Ext) {
			continue
		}
		names[strings.TrimSuffix(e.Name(), Ext)] = true
	}
	return names, nil
}

// LoadedExample is an example with its values decoded and type-checked.
type LoadedExample struct {
	Example
	Input  map[string]any // parameter name -> value
	Output any
}

// LoadExample reads both files of e and checks them against the spec.
func (p *Project) LoadExample(e Example) (*LoadedExample, error) {
	in, err := ReadValueFile(e.InputPath)
	if err != nil {
		return nil, err
	}
	if err := p.Spec.Input.Check(in, "input"); err != nil {
		return nil, fmt.Errorf("%s: %w", e.InputPath, err)
	}
	out, err := ReadValueFile(e.OutputPath)
	if err != nil {
		return nil, err
	}
	if err := p.Spec.Output.Check(out, "output"); err != nil {
		return nil, fmt.Errorf("%s: %w", e.OutputPath, err)
	}
	return &LoadedExample{Example: e, Input: in.(map[string]any), Output: out}, nil
}

// Validate loads every example in every split and returns all problems
// found: unpaired files, type errors, and contradictions (the same input
// with different outputs, which no program can fit).
func (p *Project) Validate() []error {
	var errs []error
	type seen struct {
		ex  Example
		out string
	}
	byInput := map[string]seen{}
	for _, sp := range Splits {
		exs, pairErrs := p.Examples(sp)
		errs = append(errs, pairErrs...)
		for _, e := range exs {
			le, err := p.LoadExample(e)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			in, err1 := CanonicalValue(p.Spec.Input, any(le.Input))
			out, err2 := CanonicalValue(p.Spec.Output, le.Output)
			if err1 != nil || err2 != nil {
				continue // already type-checked; cannot happen
			}
			if prev, dup := byInput[in]; dup && prev.out != out {
				errs = append(errs, &ContradictionError{A: prev.ex, B: e})
			} else if !dup {
				byInput[in] = seen{e, out}
			}
		}
	}
	return errs
}

// ContradictionError reports two examples with equal inputs but different outputs.
type ContradictionError struct{ A, B Example }

func (e *ContradictionError) Error() string {
	return fmt.Sprintf("contradiction: %s and %s have the same input but different outputs", e.A.InputPath, e.B.InputPath)
}

// WriteExample type-checks and writes an example's two files. It refuses to
// overwrite existing files unless overwrite is true.
func (p *Project) WriteExample(sp Split, name string, input, output any, overwrite bool) (Example, error) {
	e := Example{Split: sp, Name: name, InputPath: p.ExamplePath(sp, name, Input), OutputPath: p.ExamplePath(sp, name, Output)}
	if !ValidName(name) {
		return e, fmt.Errorf("invalid example name %q (use letters, digits, '-', '_', '.')", name)
	}
	if err := p.Spec.Input.Check(input, "input"); err != nil {
		return e, err
	}
	if err := p.Spec.Output.Check(output, "output"); err != nil {
		return e, err
	}
	if !overwrite {
		for _, path := range []string{e.InputPath, e.OutputPath} {
			if _, err := os.Stat(path); err == nil {
				return e, fmt.Errorf("%s already exists", path)
			}
		}
	}
	for _, side := range []Side{Input, Output} {
		if err := os.MkdirAll(p.SideDir(sp, side), 0o755); err != nil {
			return e, err
		}
	}
	// Each file is written atomically; if the second fails, remove the first
	// so no orphan is left behind (unless it existed before and we overwrote it).
	_, inExisted := os.Stat(e.InputPath)
	if err := WriteValueFile(e.InputPath, p.Spec.Input, input); err != nil {
		return e, err
	}
	if err := WriteValueFile(e.OutputPath, p.Spec.Output, output); err != nil {
		if inExisted != nil {
			os.Remove(e.InputPath)
		}
		return e, err
	}
	return e, nil
}

// ValidName reports whether name is usable as an example file name.
func ValidName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		ok := r == '-' || r == '_' || r == '.' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !ok {
			return false
		}
	}
	return true
}

// NextName returns the next free zero-padded numeric name in a split
// ("001", "002", ...), looking at both in/ and out/.
func (p *Project) NextName(sp Split) (string, error) {
	maxN := 0
	for _, side := range []Side{Input, Output} {
		names, err := listNames(p.SideDir(sp, side))
		if err != nil {
			return "", err
		}
		for n := range names {
			if v, err := strconv.Atoi(n); err == nil && v > 0 {
				maxN = max(maxN, v)
			}
		}
	}
	return fmt.Sprintf("%03d", maxN+1), nil
}
