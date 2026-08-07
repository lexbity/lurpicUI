// Package dataset defines the deterministic seed data model for Lurpic
// Studio. The flagship exhibit (E1) streams rows over time and charts them;
// this package is the pure, framework-free foundation the store topology in
// state/ is built on.
//
// The bundled seed file (assets/metrics.csv, reached via SeedPath) is a
// starting snapshot; the streaming feed reshapes it at runtime and is never a
// realtime source (NG-1).
package dataset

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// SeedPath is the module-relative path of the bundled seed CSV. The demo
// embeds the file with //go:embed and feeds the bytes to Parse.
const SeedPath = "assets/metrics.csv"

// Row is one metric sample in the streaming feed and the chart's input.
//
// ID is the monotonic collection key assigned at insert (never parsed from
// the seed file); Value is the plotted metric; Region groups rows into
// categorical bands (the bar chart's band x-scale); Time is the x-domain
// value. The ID lives on the row because CollectionStore re-derives item ids
// from items after removals, so identity must be deterministic per row and
// stable across edits — a data-derived id would collide on edit (see the
// F-row-id finding in README.md).
type Row struct {
	ID     uint64
	Time   time.Time
	Value  float64
	Region string
}

// ParseError is a typed, per-line seed data error. Line is the 1-based data
// row number (2 = the first row after the header), Field the offending column,
// and Value the raw cell text.
type ParseError struct {
	Line  int
	Field string
	Value string
	Err   error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("dataset: line %d: field %q value %q: %v", e.Line, e.Field, e.Value, e.Err)
}

// Unwrap exposes the underlying parse error for errors.As/Is.
func (e *ParseError) Unwrap() error { return e.Err }

// Column names — declared once so strict header checking and per-line errors
// share the same canonical names.
const (
	colDate    = "date"
	colRevenue = "revenue"
	colUsers   = "users"
	colRegion  = "region"
)

// seedHeader is the exact required header row.
var seedHeader = []string{colDate, colRevenue, colUsers, colRegion}

const dateLayout = "2006-01-02"

// Parse reads a strict, CRLF-tolerant metrics CSV and returns its rows in
// file order. The header must match date,revenue,users,region exactly and
// every column of every row is validated; the users column is validated but
// not carried by Row (it is not part of the flagship model — see the F-users
// finding). The first malformed data row fails with a *ParseError; structural
// failures (empty input, wrong header, inconsistent columns) return plain
// errors.
func Parse(data []byte) ([]Row, error) {
	// FieldsPerRecord is set to -1 so csv.Reader never rejects a record on
	// field count: column strictness lives in equalHeader and parseRow so
	// every branch is reachable and every per-line error is a typed
	// *ParseError. csv.Reader only reports syntax failures (malformed
	// quoting), which carry the physical line.
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err == io.EOF {
		return nil, fmt.Errorf("dataset: empty seed: no header")
	}
	if err != nil {
		return nil, fmt.Errorf("dataset: read header: %w", err)
	}
	if !equalHeader(header) {
		return nil, fmt.Errorf("dataset: header %q, want %q", header, seedHeader)
	}

	var rows []Row
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, &ParseError{Line: recordLine(err, len(rows)+2), Field: "record", Err: err}
		}

		row, perr := parseRow(rec)
		if perr != nil {
			perr.Line = len(rows) + 2
			return nil, perr
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// recordLine extracts the physical line from a record-level csv error,
// falling back to the caller-computed data-row line when the error carries
// no line info. The standard csv.Reader reports only *csv.ParseError today,
// but the fallback keeps Parse's error surface uniformly typed regardless.
func recordLine(err error, fallback int) int {
	var perr *csv.ParseError
	if errors.As(err, &perr) && perr.Line > 0 {
		return perr.Line
	}
	return fallback
}

// equalHeader reports whether the parsed header matches the canonical schema
// exactly (names and order).
func equalHeader(header []string) bool {
	if len(header) != len(seedHeader) {
		return false
	}
	for i, name := range header {
		if name != seedHeader[i] {
			return false
		}
	}
	return true
}

// parseRow validates one 4-column record and maps it to a Row. The returned
// *ParseError carries Field and Value but no Line (the caller assigns it).
func parseRow(rec []string) (Row, *ParseError) {
	if len(rec) != len(seedHeader) {
		return Row{}, &ParseError{Field: "record", Err: fmt.Errorf("got %d fields, want %d", len(rec), len(seedHeader))}
	}

	t, err := time.Parse(dateLayout, rec[0])
	if err != nil {
		return Row{}, &ParseError{Field: colDate, Value: rec[0], Err: err}
	}

	value, err := strconv.ParseFloat(rec[1], 64)
	if err != nil {
		return Row{}, &ParseError{Field: colRevenue, Value: rec[1], Err: err}
	}

	if _, err := strconv.ParseUint(rec[2], 10, 64); err != nil {
		return Row{}, &ParseError{Field: colUsers, Value: rec[2], Err: err}
	}

	region := strings.TrimSpace(rec[3])
	if region == "" {
		return Row{}, &ParseError{Field: colRegion, Value: rec[3], Err: errors.New("region is empty")}
	}

	return Row{Time: t, Value: value, Region: region}, nil
}
