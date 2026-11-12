// Package dataio reads example data from outside a project (pairs of
// directories, JSONL files and CSV files) and plans how to write it into a
// project's data/ directory.
//
// Importing happens in two phases so that it is all-or-nothing:
//
//  1. A reader (ReadDirs, ReadJSONL, ReadCSV) turns a source into Records,
//     collecting per-record problems instead of stopping at the first one.
//  2. Plan assigns each record a split and a name and checks it against the
//     project (types, names, existing files). Only if no record has a
//     problem does Apply write anything.
package dataio

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Record is one example read from a source, not yet placed in a project.
type Record struct {
	// Source says where the record came from, for error messages:
	// "file.jsonl:3", "file.csv:7" or the input file path for directories.
	Source string
	// Name is the example name suggested by the source (the file basename
	// for directory imports); empty means "generate one".
	Name   string
	Input  any
	Output any
}

// Batch is the result of reading a source.
type Batch struct {
	Records []Record
	// Problems are per-record errors (bad JSON, bad CSV cell, ...). Any
	// problem makes the import fail, but reading continues so that all of
	// them can be reported at once.
	Problems []error
	// Warnings are reported but do not fail the import (e.g. an input file
	// with no matching output file).
	Warnings []string
}

// sourceError prefixes an error with its source location.
type sourceError struct {
	Source string
	Err    error
}

func (e *sourceError) Error() string { return e.Source + ": " + e.Err.Error() }
func (e *sourceError) Unwrap() error { return e.Err }

func errAt(source string, format string, a ...any) error {
	return &sourceError{Source: source, Err: fmt.Errorf(format, a...)}
}

// trimBOM removes a leading UTF-8 byte order mark, which Windows tools
// (Notepad, Excel's "CSV UTF-8") like to add.
func trimBOM(s string) string { return strings.TrimPrefix(s, "\ufeff") }

// SortProblems orders problems by source file and then line number, so
// that problems found while reading and while planning interleave in file
// order. Problems without a source location keep their order, first.
func SortProblems(errs []error) {
	slices.SortStableFunc(errs, func(a, b error) int {
		fa, la := sourceKey(a)
		fb, lb := sourceKey(b)
		if c := strings.Compare(fa, fb); c != 0 {
			return c
		}
		return la - lb
	})
}

// CountSources returns the number of distinct sources (examples) that the
// problems refer to; problems without a source count as one each.
func CountSources(errs []error) int {
	seen := map[string]bool{}
	n := 0
	for _, e := range errs {
		var se *sourceError
		if !errors.As(e, &se) {
			n++
			continue
		}
		if !seen[se.Source] {
			seen[se.Source] = true
			n++
		}
	}
	return n
}

func sourceKey(err error) (string, int) {
	var se *sourceError
	if !errors.As(err, &se) {
		return "", 0
	}
	i := strings.LastIndexByte(se.Source, ':')
	if i < 0 {
		return se.Source, 0
	}
	line, convErr := strconv.Atoi(se.Source[i+1:])
	if convErr != nil {
		return se.Source, 0
	}
	return se.Source[:i], line
}
