package dataset

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mustParse(t *testing.T, data string) []Row {
	t.Helper()
	rows, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse(%q): %v", data, err)
	}
	return rows
}

func TestParse_happyPath(t *testing.T) {
	csv := `date,revenue,users,region
2026-01-01,100,10,north
2026-01-02,250,20,south
2026-01-03,12.5,30,east
2026-01-04,0.5,40,west
`
	rows := mustParse(t, csv)
	if len(rows) != 4 {
		t.Fatalf("len(rows) = %d, want 4", len(rows))
	}

	want := []Row{
		{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Value: 100, Region: "north"},
		{Time: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Value: 250, Region: "south"},
		{Time: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), Value: 12.5, Region: "east"},
		{Time: time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC), Value: 0.5, Region: "west"},
	}
	for i, w := range want {
		if rows[i] != w {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], w)
		}
		if rows[i].ID != 0 {
			t.Errorf("row %d: ID = %d, want 0 (Parse never assigns ids)", i, rows[i].ID)
		}
	}
}

func TestParse_trailingNewline(t *testing.T) {
	csv := "date,revenue,users,region\n2026-01-01,100,10,north\n\n"
	rows := mustParse(t, csv)
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1 (trailing blank line ignored)", len(rows))
	}
}

func TestParse_crlf(t *testing.T) {
	csv := "date,revenue,users,region\r\n2026-01-01,100,10,north\r\n2026-01-02,200,20,south\r\n"
	rows := mustParse(t, csv)
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2 (CRLF tolerated)", len(rows))
	}
	if rows[1].Value != 200 || rows[1].Region != "south" {
		t.Fatalf("CRLF row = %+v, want Value=200 Region=south", rows[1])
	}
}

func TestParse_emptyFile(t *testing.T) {
	_, err := Parse(nil)
	if err == nil {
		t.Fatal("Parse(nil) = nil error, want empty-seed error")
	}
	if !strings.Contains(err.Error(), "empty seed") {
		t.Fatalf("empty error = %v, want mention of empty seed", err)
	}
}

func TestParse_headerOnly(t *testing.T) {
	rows := mustParse(t, "date,revenue,users,region\n")
	if len(rows) != 0 {
		t.Fatalf("header-only file parsed to %d rows, want 0", len(rows))
	}
}

func TestParse_badHeader(t *testing.T) {
	cases := []string{
		"date,revenue,users\n2026-01-01,100,10,north\n",                // missing region column
		"date,revenue,users,region,extra\n2026-01-01,100,10,north,x\n", // extra column
		"DATE,REVENUE,USERS,REGION\n2026-01-01,100,10,north\n",         // wrong case
		"users,region,date,revenue\n2026-01-01,100,10,north\n",         // wrong order
	}
	for _, c := range cases {
		if _, err := Parse([]byte(c)); err == nil {
			t.Errorf("Parse(%q): nil error, want header error", c)
		}
	}
}

func TestParse_badDate(t *testing.T) {
	csv := "date,revenue,users,region\nnot-a-date,100,10,north\n"
	_, err := Parse([]byte(csv))
	var perr *ParseError
	if !errors.As(err, &perr) {
		t.Fatalf("err = %T %v, want *ParseError", err, err)
	}
	if perr.Line != 2 || perr.Field != "date" || perr.Value != "not-a-date" {
		t.Fatalf("ParseError = %+v, want Line=2 Field=date Value=not-a-date", perr)
	}
}

func TestParse_badRevenue(t *testing.T) {
	csv := "date,revenue,users,region\n2026-01-01,not-a-number,10,north\n"
	_, err := Parse([]byte(csv))
	var perr *ParseError
	if !errors.As(err, &perr) {
		t.Fatalf("err = %T %v, want *ParseError", err, err)
	}
	if perr.Line != 2 || perr.Field != "revenue" || perr.Value != "not-a-number" {
		t.Fatalf("ParseError = %+v, want Line=2 Field=revenue", perr)
	}
}

func TestParse_badUsers(t *testing.T) {
	csv := "date,revenue,users,region\n2026-01-01,100,ten,north\n"
	_, err := Parse([]byte(csv))
	var perr *ParseError
	if !errors.As(err, &perr) {
		t.Fatalf("err = %T %v, want *ParseError", err, err)
	}
	if perr.Line != 2 || perr.Field != "users" || perr.Value != "ten" {
		t.Fatalf("ParseError = %+v, want Line=2 Field=users Value=ten", perr)
	}
}

func TestParse_negativeUsers(t *testing.T) {
	csv := "date,revenue,users,region\n2026-01-01,100,-10,north\n"
	if _, err := Parse([]byte(csv)); err == nil {
		t.Fatal("Parse accepted negative users")
	}
}

func TestParse_emptyRegion(t *testing.T) {
	csv := "date,revenue,users,region\n2026-01-01,100,10,\n"
	_, err := Parse([]byte(csv))
	var perr *ParseError
	if !errors.As(err, &perr) {
		t.Fatalf("err = %T %v, want *ParseError", err, err)
	}
	if perr.Field != "region" {
		t.Fatalf("ParseError.Field = %q, want region", perr.Field)
	}
}

func TestParse_columnCountMismatch(t *testing.T) {
	cases := []string{
		"date,revenue,users,region\n2026-01-01,100,10\n",             // too few columns
		"date,revenue,users,region\n2026-01-01,100,10,north,extra\n", // too many columns
	}
	for _, csv := range cases {
		_, err := Parse([]byte(csv))
		if err == nil {
			t.Errorf("Parse(%q): nil error, want field-count error", csv)
		}
		var perr *ParseError
		if !errors.As(err, &perr) {
			t.Fatalf("err = %T %v, want *ParseError for field-count violation", err, err)
		}
		if perr.Field != "record" {
			t.Errorf("ParseError.Field = %q, want record", perr.Field)
		}
	}
}

func TestParse_malformedHeader(t *testing.T) {
	// A bare quote in the first line fails the header read itself.
	csv := "\"date,revenue,users,region\n2026-01-01,100,10,north\n"
	_, err := Parse([]byte(csv))
	if err == nil {
		t.Fatal("Parse accepted a header with a malformed quote")
	}
	if !strings.Contains(err.Error(), "read header") {
		t.Fatalf("header error = %v, want mention of reading the header", err)
	}
}

func TestRecordLine(t *testing.T) {
	csvErr := &csv.ParseError{Line: 42}
	if got := recordLine(csvErr, 9); got != 42 {
		t.Errorf("recordLine(*csv.ParseError{Line:42}) = %d, want 42", got)
	}
	if got := recordLine(&csv.ParseError{Line: 0}, 9); got != 9 {
		t.Errorf("recordLine(*csv.ParseError{Line:0}) = %d, want fallback 9", got)
	}
	if got := recordLine(errors.New("boom"), 9); got != 9 {
		t.Errorf("recordLine(non-csv error) = %d, want fallback 9", got)
	}
}

func TestParse_malformedQuote(t *testing.T) {
	// A bare quote makes csv.Reader fail on the record itself.
	csv := "date,revenue,users,region\n2026-01-01,100,10,\"unclosed\n"
	_, err := Parse([]byte(csv))
	if err == nil {
		t.Fatal("Parse accepted a record with a malformed quote")
	}
	var perr *ParseError
	if !errors.As(err, &perr) {
		t.Fatalf("err = %T %v, want *ParseError for csv parse failure", err, err)
	}
	if perr.Field != "record" {
		t.Fatalf("ParseError.Field = %q, want record", perr.Field)
	}
}

func TestParse_errorCarriesLineNumberForLaterRows(t *testing.T) {
	// The failing row is the third data row, so its line is 4.
	csv := "date,revenue,users,region\n2026-01-01,100,10,north\n2026-01-02,200,20,south\n2026-01-03,oops,30,east\n"
	_, err := Parse([]byte(csv))
	var perr *ParseError
	if !errors.As(err, &perr) {
		t.Fatalf("err = %T %v, want *ParseError", err, err)
	}
	if perr.Line != 4 || perr.Field != "revenue" {
		t.Fatalf("ParseError = %+v, want Line=4 Field=revenue", perr)
	}
}

// TestParse_seedFile pins the committed seed asset against the schema: 40
// rows, 4 regions, strict header, and exact integer values (the generated
// seed uses integer revenues so every value is exactly representable). This
// guards the seed file against drifting from the Parse contract.
// TestParseError_messageAndUnwrap covers the typed error's Error() and
// Unwrap() methods so the per-line error surface is fully pinned.
func TestParseError_messageAndUnwrap(t *testing.T) {
	perr := &ParseError{Line: 7, Field: "revenue", Value: "abc", Err: errors.New("bad float")}
	msg := perr.Error()
	for _, want := range []string{"line 7", `field "revenue"`, `value "abc"`, "bad float"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, want it to contain %q", msg, want)
		}
	}

	// errors.Is/As must traverse the chain through Unwrap.
	wrapped := fmt.Errorf("outer: %w", perr)
	if !errors.Is(wrapped, perr.Err) {
		t.Error("errors.Is through wrapped ParseError failed")
	}
}

func TestParse_seedFile(t *testing.T) {
	path := filepath.Join("..", "assets", "metrics.csv")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seed file %s: %v", path, err)
	}
	rows, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse seed file: %v", err)
	}
	if len(rows) != 40 {
		t.Fatalf("seed rows = %d, want 40", len(rows))
	}

	regions := map[string]int{}
	for _, r := range rows {
		regions[r.Region]++
		if r.ID != 0 {
			t.Fatalf("seed row %s has pre-assigned ID %d", r.Time, r.ID)
		}
	}
	if len(regions) != 4 {
		t.Fatalf("seed regions = %v, want 4 distinct", regions)
	}
	for region, count := range regions {
		if count != 10 {
			t.Errorf("region %q count = %d, want 10", region, count)
		}
	}

	first := rows[0]
	if first.Time.Format(dateLayout) != "2026-01-01" || first.Value != 820 || first.Region != "north" {
		t.Fatalf("first seed row = %+v, want 2026-01-01/820/north", first)
	}
	last := rows[len(rows)-1]
	if last.Time.Format(dateLayout) != "2026-02-09" {
		t.Fatalf("last seed row time = %s, want 2026-02-09", last.Time.Format(dateLayout))
	}
}
