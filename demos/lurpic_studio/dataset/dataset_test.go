package dataset

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseHappyPath(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,12450.50,1840,NA\n2026-06-15,9876.54,2500,EU\n"
	rows, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	checkRow(t, rows[0], time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 12450.50, 1840, "NA")
	checkRow(t, rows[1], time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), 9876.54, 2500, "EU")
}

func TestParseBundledCSV(t *testing.T) {
	data, err := os.ReadFile(filepath.FromSlash("../assets/metrics.csv"))
	if err != nil {
		t.Fatalf("reading bundled csv: %v", err)
	}
	rows, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 40 {
		t.Fatalf("expected 40 rows, got %d", len(rows))
	}
	first := rows[0]
	checkRow(t, first, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 12450.50, 1840, "NA")
	last := rows[39]
	checkRow(t, last, time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC), 13540.80, 2103, "LATAM")

	regionCounts := map[string]int{}
	for _, r := range rows {
		regionCounts[r.Region]++
	}
	if regionCounts["NA"] != 10 {
		t.Errorf("expected 10 NA rows, got %d", regionCounts["NA"])
	}
	if regionCounts["EU"] != 10 {
		t.Errorf("expected 10 EU rows, got %d", regionCounts["EU"])
	}
	if regionCounts["APAC"] != 10 {
		t.Errorf("expected 10 APAC rows, got %d", regionCounts["APAC"])
	}
	if regionCounts["LATAM"] != 10 {
		t.Errorf("expected 10 LATAM rows, got %d", regionCounts["LATAM"])
	}
}

func TestParseEmptyFile(t *testing.T) {
	_, err := Parse([]byte{})
	if err == nil {
		t.Fatal("expected error for empty file")
	}
	var pe *ParseError
	if !as(err, &pe) {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Line != 0 {
		t.Errorf("expected line 0, got %d", pe.Line)
	}
}

func TestParseHeaderOnly(t *testing.T) {
	_, err := Parse([]byte("date,revenue,users,region\n"))
	if err == nil {
		t.Fatal("expected error for header-only file")
	}
	var pe *ParseError
	if !as(err, &pe) {
		t.Fatalf("expected *ParseError for header only, got %T", err)
	}
}

func TestParseHeaderOnlyNoNewline(t *testing.T) {
	_, err := Parse([]byte("date,revenue,users,region"))
	if err == nil {
		t.Fatal("expected error for header-only file without newline")
	}
}

func TestParseMalformedHeader(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"wrong order", "date,users,revenue,region\n2026-01-01,1840,12450.50,NA\n"},
		{"extra column", "date,revenue,users,region,extra\n2026-01-01,12450.50,1840,NA,boom\n"},
		{"missing column", "date,revenue,users\n2026-01-01,12450.50,1840\n"},
		{"typo", "date,revenue,user,region\n2026-01-01,12450.50,1840,NA\n"},
		{"lowercase header", "DATE,REVENUE,USERS,REGION\n2026-01-01,12450.50,1840,NA\n"},
		{"empty header", "\n2026-01-01,12450.50,1840,NA\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.data))
			if err == nil {
				t.Fatal("expected error for malformed header")
			}
			var he *HeaderError
			if !as(err, &he) {
				t.Fatalf("expected *HeaderError, got %T", err)
			}
		})
	}
}

func TestParseBadDate(t *testing.T) {
	data := "date,revenue,users,region\nnot-a-date,12450.50,1840,NA\n2026-13-01,9999.99,1000,EU\n"
	_, err := Parse([]byte(data))
	if err == nil {
		t.Fatal("expected error for bad dates")
	}
	var pes *ParseErrors
	if !as(err, &pes) {
		t.Fatalf("expected *ParseErrors, got %T", err)
	}
	if len(pes.Errors) != 2 {
		t.Fatalf("expected 2 parse errors, got %d", len(pes.Errors))
	}
	if pes.Errors[0].Line != 2 {
		t.Errorf("expected line 2 for first bad date, got %d", pes.Errors[0].Line)
	}
	if pes.Errors[1].Line != 3 {
		t.Errorf("expected line 3 for second bad date, got %d", pes.Errors[1].Line)
	}
}

func TestParseBadFloat(t *testing.T) {
	cases := []struct {
		name   string
		data   string
		column string
		line   int
	}{
		{"revenue non-numeric", "date,revenue,users,region\n2026-01-01,abc,1840,NA\n", "revenue", 2},
		{"revenue empty", "date,revenue,users,region\n2026-01-01,,1840,NA\n", "revenue", 2},
		{"users non-numeric", "date,revenue,users,region\n2026-01-01,12450.50,abc,NA\n", "users", 2},
		{"users empty", "date,revenue,users,region\n2026-01-01,12450.50,,NA\n", "users", 2},
		{"both bad", "date,revenue,users,region\n2026-01-01,xyz,bad,NA\n", "revenue", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.data))
			if err == nil {
				t.Fatal("expected error for bad float")
			}
			var pes *ParseErrors
			if !as(err, &pes) {
				t.Fatalf("expected *ParseErrors, got %T", err)
			}
			found := false
			for _, pe := range pes.Errors {
				if pe.Column == tc.column && pe.Line == tc.line {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error at line %d column %q, got errors: %v", tc.line, tc.column, pes.Errors)
			}
		})
	}
}

func TestParseEmptyRegion(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,12450.50,1840,\n"
	_, err := Parse([]byte(data))
	if err == nil {
		t.Fatal("expected error for empty region")
	}
	var pes *ParseErrors
	if !as(err, &pes) {
		t.Fatalf("expected *ParseErrors, got %T", err)
	}
	if pes.Errors[0].Column != "region" {
		t.Errorf("expected column 'region', got %q", pes.Errors[0].Column)
	}
}

func TestParseTooFewColumns(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,12450.50,1840\n"
	_, err := Parse([]byte(data))
	if err == nil {
		t.Fatal("expected error for too few columns")
	}
}

func TestParseTooManyColumns(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,12450.50,1840,NA,extra\n"
	_, err := Parse([]byte(data))
	if err == nil {
		t.Fatal("expected error for too many columns")
	}
}

func TestParseMultipleLinesWithErrors(t *testing.T) {
	data := "date,revenue,users,region\n" +
		"2026-01-01,12450.50,1840,NA\n" +
		"bad-date,9999.99,1000,EU\n" +
		"2026-01-03,not-a-number,2000,APAC\n" +
		"2026-01-04,10000.00,,LATAM\n" +
		"2026-01-05,15000.00,2500,\n" +
		"2026-01-06,20000.00,3000,EU\n"
	rows, err := Parse([]byte(data))
	if err == nil {
		t.Fatal("expected error for multiple bad lines")
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 valid rows, got %d", len(rows))
	}
	checkRow(t, rows[0], time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 12450.50, 1840, "NA")
	checkRow(t, rows[1], time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC), 20000.00, 3000, "EU")

	var pes *ParseErrors
	if !as(err, &pes) {
		t.Fatalf("expected *ParseErrors, got %T", err)
	}
	if len(pes.Errors) != 4 {
		t.Fatalf("expected 4 parse errors, got %d", len(pes.Errors))
	}
}

func TestParseTrailingNewline(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,12450.50,1840,NA\n\n"
	rows, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
}

func TestParseCRLF(t *testing.T) {
	data := "date,revenue,users,region\r\n2026-01-01,12450.50,1840,NA\r\n2026-01-02,13110.00,1902,EU\r\n"
	rows, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	checkRow(t, rows[0], time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 12450.50, 1840, "NA")
	checkRow(t, rows[1], time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), 13110.00, 1902, "EU")
}

func TestParseMixedCRLFAndLF(t *testing.T) {
	data := "date,revenue,users,region\r\n2026-01-01,12450.50,1840,NA\n2026-01-02,13110.00,1902,EU\r\n"
	rows, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
}

func TestParseEmptyFieldsInMiddleColumns(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,,,NA\n"
	_, err := Parse([]byte(data))
	if err == nil {
		t.Fatal("expected error for empty numeric fields")
	}
	var pes *ParseErrors
	if !as(err, &pes) {
		t.Fatalf("expected *ParseErrors, got %T", err)
	}
	if len(pes.Errors) != 2 {
		t.Fatalf("expected 2 errors (revenue + users), got %d", len(pes.Errors))
	}
}

func TestParseWhitespaceInHeader(t *testing.T) {
	data := "date, revenue,users,region\n2026-01-01,12450.50,1840,NA\n"
	_, err := Parse([]byte(data))
	if err == nil {
		t.Fatal("expected error for header with space")
	}
	var he *HeaderError
	if !as(err, &he) {
		t.Fatalf("expected *HeaderError, got %T", err)
	}
}

func TestParseRowLeadingWhitespaceInNumericFields(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01, 12450.50,1840,NA\n"
	rows, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Revenue != 12450.50 {
		t.Errorf("expected revenue 12450.50, got %f", rows[0].Revenue)
	}
}

func TestParsePrecisionValues(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,0.01,1,NA\n"
	rows, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Revenue != 0.01 {
		t.Errorf("expected revenue 0.01, got %f", rows[0].Revenue)
	}
	if rows[0].Users != 1 {
		t.Errorf("expected users 1, got %f", rows[0].Users)
	}
}

func TestParseLargeValues(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,99999999.99,100000,NA\n"
	rows, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Revenue != 99999999.99 {
		t.Errorf("expected revenue 99999999.99, got %f", rows[0].Revenue)
	}
	if rows[0].Users != 100000 {
		t.Errorf("expected users 100000, got %f", rows[0].Users)
	}
}

func TestParseNegativeValues(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,-1000.50,-50,NA\n"
	rows, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Revenue != -1000.50 {
		t.Errorf("expected revenue -1000.50, got %f", rows[0].Revenue)
	}
	if rows[0].Users != -50 {
		t.Errorf("expected users -50, got %f", rows[0].Users)
	}
}

func TestParseRegionTrim(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,12450.50,1840, NA \n"
	rows, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows[0].Region != "NA" {
		t.Errorf("expected region 'NA', got %q", rows[0].Region)
	}
}

func TestParseErrorImplementsError(t *testing.T) {
	pe := &ParseError{Line: 5, Column: "revenue", Value: "abc", Err: nil}
	if pe.Error() == "" {
		t.Fatal("ParseError.Error() should not be empty")
	}
}

func TestParseErrorsImplementsError(t *testing.T) {
	pes := &ParseErrors{}
	if pes.Error() != "no parse errors" {
		t.Errorf("expected 'no parse errors', got %q", pes.Error())
	}
	pes = &ParseErrors{Errors: []*ParseError{{Line: 2, Err: nil}}}
	if pes.Error() == "" {
		t.Fatal("ParseErrors.Error() should not be empty")
	}
	pes = &ParseErrors{Errors: []*ParseError{
		{Line: 2, Err: nil},
		{Line: 3, Err: nil},
	}}
	if pes.Error() == "" {
		t.Fatal("ParseErrors.Error() with multiple errors should not be empty")
	}
}

func TestHeaderErrorImplementsError(t *testing.T) {
	he := &HeaderError{Expected: []string{"a", "b"}, Got: []string{"a", "c"}}
	if he.Error() == "" {
		t.Fatal("HeaderError.Error() should not be empty")
	}
}

func checkRow(t *testing.T, r Row, date time.Time, revenue, users float64, region string) {
	t.Helper()
	if !r.Date.Equal(date) {
		t.Errorf("expected date %v, got %v", date, r.Date)
	}
	if r.Revenue != revenue {
		t.Errorf("expected revenue %f, got %f", revenue, r.Revenue)
	}
	if r.Users != users {
		t.Errorf("expected users %f, got %f", users, r.Users)
	}
	if r.Region != region {
		t.Errorf("expected region %q, got %q", region, r.Region)
	}
}

func as(err error, target interface{}) bool {
	if err == nil {
		return false
	}
	switch t := target.(type) {
	case **ParseError:
		*t, _ = err.(*ParseError)
		return *t != nil
	case **ParseErrors:
		*t, _ = err.(*ParseErrors)
		return *t != nil
	case **HeaderError:
		*t, _ = err.(*HeaderError)
		return *t != nil
	}
	return false
}
