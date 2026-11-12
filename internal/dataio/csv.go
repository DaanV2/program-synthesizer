package dataio

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/DaanV2/program-synthesizer/internal/project"
)

// CSV column names for the output: "out" for a scalar output, "out.<field>"
// for each field of a record output.
const (
	csvOut       = "out"
	csvOutPrefix = "out."
)

// CSVColumns returns the columns a CSV file must have for the given spec
// types, or an error explaining why the spec cannot be imported from CSV.
// CSV import needs every input parameter to be a scalar, and the output to
// be a scalar or a record of scalars.
func CSVColumns(input, output *project.Type) ([]string, error) {
	var cols []string
	for _, f := range input.Fields {
		if !isScalar(f.Type) {
			return nil, fmt.Errorf("CSV import needs scalar input parameters, but %q is %s (use JSONL or directories instead)", f.Name, f.Type)
		}
		if f.Name == csvOut {
			return nil, fmt.Errorf("CSV import cannot be used: input parameter %q clashes with the output column name", f.Name)
		}
		cols = append(cols, f.Name)
	}
	switch {
	case isScalar(output):
		cols = append(cols, csvOut)
	case output.Kind == project.Record:
		for _, f := range output.Fields {
			if !isScalar(f.Type) {
				return nil, fmt.Errorf("CSV import needs a scalar output or a record of scalars, but output field %q is %s (use JSONL or directories instead)", f.Name, f.Type)
			}
			cols = append(cols, csvOutPrefix+f.Name)
		}
	default:
		return nil, fmt.Errorf("CSV import needs a scalar output or a record of scalars, but output is %s (use JSONL or directories instead)", output)
	}

	return cols, nil
}

func isScalar(t *project.Type) bool {
	switch t.Kind {
	case project.Int, project.Float, project.Bool, project.String:
		return true
	default: // list, record
	}

	return false
}

// csvColumn says where one CSV column's value goes.
type csvColumn struct {
	name   string        // header text
	typ    *project.Type // scalar type to parse the cell as
	field  string        // parameter or output field name; "" for a scalar output
	output bool
}

// ReadCSV reads examples from a CSV file with a header row. The header must
// contain exactly the columns returned by CSVColumns, in any order. Each
// cell is parsed according to its declared type: int, float, bool
// (true/false/1/0, any case) or string (taken verbatim, not trimmed).
func ReadCSV(r io.Reader, source string, input, output *project.Type) (*Batch, error) {
	want, err := CSVColumns(input, output)
	if err != nil {
		return nil, err
	}
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("empty CSV file %s (expected a header row: %s)", source, strings.Join(want, ","))
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", source, err)
	}
	if len(header) > 0 {
		header[0] = trimBOM(header[0])
	}
	cols, err := mapHeader(header, want, input, output)
	if err != nil {
		return nil, fmt.Errorf("invalid CSV header in %s: %w", source, err)
	}

	b := &Batch{}
	for {
		row, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return b, nil
		}
		if err != nil {
			// csv.ParseError already names the line; a broken quote can
			// throw off everything after it, so stop here.
			return nil, fmt.Errorf("reading %s: %w", source, err)
		}
		line, _ := cr.FieldPos(0)
		loc := fmt.Sprintf("%s:%d", source, line)
		rec, problems := csvRecord(row, cols, loc)
		if len(problems) > 0 {
			b.Problems = append(b.Problems, problems...)

			continue
		}
		b.Records = append(b.Records, rec)
	}
}

func mapHeader(header, want []string, input, output *project.Type) ([]csvColumn, error) {
	var cols []csvColumn
	seen := map[string]bool{}
	for _, h := range header {
		name := strings.TrimSpace(h)
		if seen[name] {
			return nil, fmt.Errorf("duplicate column %q", name)
		}
		seen[name] = true
		if !slices.Contains(want, name) {
			return nil, fmt.Errorf("unknown column %q (want columns %s)", name, strings.Join(want, ", "))
		}
		c := csvColumn{name: name}
		switch {
		case name == csvOut:
			c.output, c.typ = true, output
		case strings.HasPrefix(name, csvOutPrefix):
			f, _ := output.Field(strings.TrimPrefix(name, csvOutPrefix))
			c.output, c.typ, c.field = true, f.Type, f.Name
		default:
			f, _ := input.Field(name)
			c.typ, c.field = f.Type, f.Name
		}
		cols = append(cols, c)
	}
	var missing []string
	for _, w := range want {
		if !seen[w] {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing column(s) %s", strings.Join(missing, ", "))
	}

	return cols, nil
}

func csvRecord(row []string, cols []csvColumn, loc string) (Record, []error) {
	if len(row) != len(cols) {
		// encoding/csv already enforces a constant field count; this is a
		// safety net.
		return Record{}, []error{errAt(loc, "expected %d columns, got %d", len(cols), len(row))}
	}
	rec := Record{Source: loc}
	in := map[string]any{}
	var outRec map[string]any
	var problems []error
	for i, c := range cols {
		v, err := parseCell(row[i], c.typ.Kind)
		if err != nil {
			problems = append(problems, errAt(loc, "column %q: %v", c.name, err))

			continue
		}
		switch {
		case !c.output:
			in[c.field] = v
		case c.field == "":
			rec.Output = v
		default:
			if outRec == nil {
				outRec = map[string]any{}
			}
			outRec[c.field] = v
		}
	}
	rec.Input = in
	if outRec != nil {
		rec.Output = outRec
	}

	return rec, problems
}

// jsonNumberRe matches the JSON number grammar.
var jsonNumberRe = regexp.MustCompile(`^-?(0|[1-9]\d*)(\.\d+)?([eE][+-]?\d+)?$`)

// parseCell converts one CSV cell to the value DecodeValue would produce
// for the same JSON (json.Number for numbers).
func parseCell(cell string, k project.Kind) (any, error) {
	if k == project.String {
		return cell, nil
	}
	s := strings.TrimSpace(cell)
	if s == "" {
		return nil, fmt.Errorf("empty value, expected %s", k)
	}
	switch k {
	case project.Int:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid int %q", cell)
		}

		return json.Number(strconv.FormatInt(n, 10)), nil
	case project.Float:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, fmt.Errorf("invalid float %q", cell)
		}
		if jsonNumberRe.MatchString(s) {
			return json.Number(s), nil // keep "2.50" as written
		}

		return json.Number(strconv.FormatFloat(f, 'g', -1, 64)), nil
	case project.Bool:
		v, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("invalid bool %q (want true/false or 1/0)", cell)
		}

		return v, nil
	default:
		return nil, fmt.Errorf("unsupported type %s", k)
	}
}
