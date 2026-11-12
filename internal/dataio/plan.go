package dataio

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"

	"github.com/DaanV2/program-synthesizer/internal/project"
)

// Options control where imported records go.
type Options struct {
	// Split receives every record, except those sent to holdout by
	// HoldoutFraction. Defaults to train.
	Split project.Split
	// HoldoutFraction, if > 0, sends round(fraction*n) randomly chosen
	// records to the holdout split instead of Split.
	HoldoutFraction float64
	// Seed makes the holdout choice reproducible.
	Seed uint64
	// Overwrite allows replacing existing example files.
	Overwrite bool
}

// Planned is a record with its destination decided.
type Planned struct {
	Record
	Split project.Split
	Name  string
}

// Plan is the outcome of checking a batch against a project. If Problems is
// empty, Apply can write Examples.
type Plan struct {
	Examples []Planned
	Problems []error
	Warnings []string
}

// Counts returns the number of planned examples per split.
func (pl *Plan) Counts() map[project.Split]int {
	c := map[project.Split]int{}
	for _, e := range pl.Examples {
		c[e.Split]++
	}

	return c
}

// MakePlan assigns every record a split and a name and checks it against
// the project: both values must match the spec, names must be valid and
// unique within a split, and existing files are only replaced with
// Overwrite. It never writes anything.
//
// Records keep their source Name if it is a valid example name; otherwise
// (and when they have none) they get the next free numeric name in their
// split, continuing from project.NextName.
func MakePlan(p *project.Project, recs []Record, opt Options) (*Plan, error) {
	if opt.Split == "" {
		opt.Split = project.Train
	}
	if opt.HoldoutFraction < 0 || opt.HoldoutFraction > 1 {
		return nil, fmt.Errorf("holdout fraction must be between 0 and 1, got %v", opt.HoldoutFraction)
	}
	if opt.HoldoutFraction > 0 && opt.Split == project.Holdout {
		return nil, errors.New("holdout fraction cannot be combined with split holdout")
	}

	pl := &Plan{Examples: make([]Planned, len(recs))}
	holdout := HoldoutMask(len(recs), opt.HoldoutFraction, opt.Seed)
	if opt.HoldoutFraction > 0 && len(recs) > 0 && countTrue(holdout) == 0 {
		pl.Warnings = append(pl.Warnings, fmt.Sprintf("holdout fraction %v of %d examples rounds to 0; nothing goes to holdout", opt.HoldoutFraction, len(recs)))
	}

	pn := planner{p: p, pl: pl, overwrite: opt.Overwrite, used: map[project.Split]map[string]string{}}
	for _, sp := range project.Splits {
		pn.used[sp] = map[string]string{}
	}
	// Pass 1: split, type check, keep valid source names.
	for i, r := range recs {
		sp := opt.Split
		if holdout[i] {
			sp = project.Holdout
		}
		pl.Examples[i] = pn.place(r, sp)
	}
	// Pass 2: generate names for the rest, skipping anything taken.
	if err := pn.generateNames(); err != nil {
		return nil, err
	}

	return pl, nil
}

// planner holds the state shared by MakePlan's two passes.
type planner struct {
	p         *project.Project
	pl        *Plan
	overwrite bool
	used      map[project.Split]map[string]string // split -> lower-case name -> source that claimed it
}

// place type-checks r and keeps its source name when that is valid and free.
func (pn *planner) place(r Record, sp project.Split) Planned {
	e := Planned{Record: r, Split: sp}
	if err := pn.p.Spec.Input.Check(r.Input, "input"); err != nil {
		pn.pl.Problems = append(pn.pl.Problems, &sourceError{Source: r.Source, Err: err})
	}
	if err := pn.p.Spec.Output.Check(r.Output, "output"); err != nil {
		pn.pl.Problems = append(pn.pl.Problems, &sourceError{Source: r.Source, Err: err})
	}
	switch {
	case r.Name == "":
		return e
	case !project.ValidName(r.Name):
		pn.pl.Warnings = append(pn.pl.Warnings, fmt.Sprintf("%s: %q is not a valid example name, generating one", r.Source, r.Name))

		return e
	}
	// Compare case-insensitively: Foo.json and foo.json are the same file
	// on Windows and macOS.
	key := strings.ToLower(r.Name)
	if prev, dup := pn.used[sp][key]; dup {
		pn.pl.Problems = append(pn.pl.Problems, errAt(r.Source, "example name %q in %s is also used by %s", r.Name, sp, prev))

		return e
	}
	pn.used[sp][key] = r.Source
	e.Name = r.Name
	if !pn.overwrite && exampleExists(pn.p, sp, e.Name) {
		pn.pl.Problems = append(pn.pl.Problems, errAt(r.Source, "example %s/%s already exists in the project (use --overwrite to replace it)", sp, e.Name))
	}

	return e
}

// generateNames gives every unnamed example the next free numeric name in
// its split, continuing from project.NextName.
func (pn *planner) generateNames() error {
	next := map[project.Split]int{}
	for i := range pn.pl.Examples {
		e := &pn.pl.Examples[i]
		if e.Name != "" {
			continue
		}
		if _, ok := next[e.Split]; !ok {
			first, err := pn.p.NextName(e.Split)
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(first)
			if err != nil {
				return fmt.Errorf("unexpected generated name %q: %w", first, err)
			}
			next[e.Split] = n
		}
		for {
			name := fmt.Sprintf("%03d", next[e.Split])
			next[e.Split]++
			if _, taken := pn.used[e.Split][name]; !taken && !exampleExists(pn.p, e.Split, name) {
				pn.used[e.Split][name] = e.Source
				e.Name = name

				break
			}
		}
	}

	return nil
}

// Apply writes every planned example. It refuses to run if the plan has
// problems. A write error (disk full, permissions) can still leave earlier
// examples written; the error says how many.
func Apply(p *project.Project, pl *Plan, overwrite bool) error {
	if len(pl.Problems) > 0 {
		return fmt.Errorf("plan has %d problem(s); nothing written", len(pl.Problems))
	}
	for i, e := range pl.Examples {
		if _, err := p.WriteExample(e.Split, e.Name, e.Input, e.Output, overwrite); err != nil {
			return fmt.Errorf("writing %s/%s (from %s): %w (%d of %d examples were written before this)", e.Split, e.Name, e.Source, err, i, len(pl.Examples))
		}
	}

	return nil
}

func exampleExists(p *project.Project, sp project.Split, name string) bool {
	for _, side := range []project.Side{project.Input, project.Output} {
		if _, err := os.Stat(p.ExamplePath(sp, name, side)); err == nil {
			return true
		}
	}

	return false
}

// HoldoutMask marks round(fraction*n) of n positions, chosen uniformly at
// random from seed. The result depends only on (n, fraction, seed): it uses
// a hand-written Fisher-Yates shuffle over PCG, whose output is specified,
// so it does not change between Go releases.
func HoldoutMask(n int, fraction float64, seed uint64) []bool {
	mask := make([]bool, n)
	k := int(math.Round(fraction * float64(n)))
	if k <= 0 {
		return mask
	}
	k = min(k, n)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	src := rand.NewPCG(seed, 0x9e3779b97f4a7c15)
	for i := n - 1; i > 0; i-- {
		j := int(boundedRand(src, uint64(i+1))) //nolint:gosec // result < i+1, which is an int
		idx[i], idx[j] = idx[j], idx[i]
	}
	for _, i := range idx[:k] {
		mask[i] = true
	}

	return mask
}

// boundedRand returns a uniform value in [0, n) by rejection sampling.
func boundedRand(src *rand.PCG, n uint64) uint64 {
	limit := math.MaxUint64 - math.MaxUint64%n
	for {
		if v := src.Uint64(); v < limit {
			return v % n
		}
	}
}

func countTrue(bs []bool) int {
	n := 0
	for _, b := range bs {
		if b {
			n++
		}
	}

	return n
}
