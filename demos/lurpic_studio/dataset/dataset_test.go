package dataset

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParse_happyPath_returnsAllRows(t *testing.T) {
	const raw = `date,revenue,users,region
2026-01-01,12450.50,1840,NA
2026-01-02,13110.00,1902,EU
2026-01-03,9875.25,1450,APAC
2026-01-04,5620.80,890,LATAM
2026-01-05,13420.75,2010,NA
2026-01-06,11890.40,1755,EU
2026-01-07,10230.60,1620,APAC
2026-01-08,6110.90,975,LATAM
2026-01-09,14230.00,2120,NA
2026-01-10,12750.30,1885,EU
2026-01-11,10870.45,1700,APAC
2026-01-12,6430.20,1020,LATAM
2026-01-13,15120.80,2210,NA
2026-01-14,13560.50,1995,EU
2026-01-15,11540.70,1780,APAC
2026-01-16,6750.35,1055,LATAM
2026-01-17,12890.90,1910,NA
2026-01-18,14010.25,2075,EU
2026-01-19,9980.55,1540,APAC
2026-01-20,5840.60,925,LATAM
2026-01-21,13850.40,2050,NA
2026-01-22,12130.75,1820,EU
2026-01-23,10460.85,1665,APAC
2026-01-24,6320.90,1005,LATAM
2026-01-25,14670.25,2160,NA
2026-01-26,12980.60,1930,EU
2026-01-27,11230.35,1720,APAC
2026-01-28,6540.45,1035,LATAM
2026-01-29,15340.80,2275,NA
2026-01-30,13760.15,2035,EU
2026-01-31,10780.50,1695,APAC
2026-02-01,6910.25,1085,LATAM
2026-02-02,14890.60,2195,NA
2026-02-03,14220.40,2090,EU
2026-02-04,11750.75,1810,APAC
2026-02-05,7150.30,1120,LATAM
2026-02-06,15670.90,2310,NA
2026-02-07,13480.55,2015,EU
2026-02-08,12120.60,1900,APAC
2026-02-09,6950.70,1095,LATAM
`
	rows, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 40 {
		t.Fatalf("expected 40 rows, got %d", len(rows))
	}

	if rows[0].Date != (time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("row[0] Date: expected 2026-01-01, got %v", rows[0].Date)
	}
	if rows[0].Revenue != 12450.50 {
		t.Fatalf("row[0] Revenue: expected 12450.50, got %f", rows[0].Revenue)
	}
	if rows[0].Users != 1840 {
		t.Fatalf("row[0] Users: expected 1840, got %f", rows[0].Users)
	}
	if rows[0].Region != "NA" {
		t.Fatalf("row[0] Region: expected NA, got %q", rows[0].Region)
	}

	if rows[39].Date != (time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("row[39] Date: expected 2026-02-09, got %v", rows[39].Date)
	}
	if rows[39].Revenue != 6950.70 {
		t.Fatalf("row[39] Revenue: expected 6950.70, got %f", rows[39].Revenue)
	}
	if rows[39].Users != 1095 {
		t.Fatalf("row[39] Users: expected 1095, got %f", rows[39].Users)
	}
	if rows[39].Region != "LATAM" {
		t.Fatalf("row[39] Region: expected LATAM, got %q", rows[39].Region)
	}
}

func TestParse_emptyInput(t *testing.T) {
	_, err := Parse([]byte{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	var emptyErr *EmptyInputError
	if !errors.As(err, &emptyErr) {
		t.Fatalf("expected *EmptyInputError, got %T", err)
	}
	// Explicitly call Error() to ensure the method body is covered.
	if got := emptyErr.Error(); got != "empty input" {
		t.Fatalf("unexpected error string: %q", got)
	}
}

func TestErrorTypes_string(t *testing.T) {
	var emptyErr EmptyInputError
	if got := emptyErr.Error(); got != "empty input" {
		t.Fatalf("EmptyInputError string: %q", got)
	}
	parseErr := &ParseError{Line: 5, Err: fmt.Errorf("test error")}
	if got := parseErr.Error(); got != "line 5: test error" {
		t.Fatalf("ParseError string: %q", got)
	}
	headerErr := &HeaderMismatchError{Expected: "exp", Actual: "act"}
	if got := headerErr.Error(); got != `expected header "exp", got "act"` {
		t.Fatalf("HeaderMismatchError string: %q", got)
	}
}

func TestParse_emptyInput_variousDepths(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "nil", data: nil},
		{name: "empty", data: []byte{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.data)
			if err == nil {
				t.Fatal("expected error for empty input")
			}
			var emptyErr *EmptyInputError
			if !errors.As(err, &emptyErr) {
				t.Fatalf("expected *EmptyInputError, got %T", err)
			}
		})
	}
}

func TestParse_malformedHeader_wrongFieldCount(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{name: "missing_columns", header: "date,revenue,users"},
		{name: "extra_column", header: "a,b,c,d,e"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.header + "\n2026-01-01,100,10,NA\n"
			_, err := Parse([]byte(data))
			if err == nil {
				t.Fatal("expected error")
			}
			var parseErr *ParseError
			if !errors.As(err, &parseErr) {
				t.Fatalf("expected *ParseError, got %T", err)
			}
			if parseErr.Line != 1 {
				t.Fatalf("expected line 1, got %d", parseErr.Line)
			}
		})
	}
}

func TestParse_malformedHeader_content(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{name: "case_mismatch", header: "Date,Revenue,Users,Region"},
		{name: "reversed_order", header: "region,users,revenue,date"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.header + "\n2026-01-01,100,10,NA\n"
			_, err := Parse([]byte(data))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "expected header") {
				t.Fatalf("error %q does not contain 'expected header'", err.Error())
			}
			var parseErr *ParseError
			if !errors.As(err, &parseErr) {
				t.Fatalf("expected *ParseError, got %T", err)
			}
			if parseErr.Line != 1 {
				t.Fatalf("expected line 1, got %d", parseErr.Line)
			}
			var headerErr *HeaderMismatchError
			if !errors.As(err, &headerErr) {
				t.Fatalf("expected *HeaderMismatchError inside ParseError, got %T", err)
			}
		})
	}
}

func TestParse_badDate(t *testing.T) {
	data := "date,revenue,users,region\nnot-a-date,100,10,NA\n"
	_, err := Parse([]byte(data))
	if err == nil {
		t.Fatal("expected error for bad date")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if parseErr.Line != 2 {
		t.Fatalf("expected line 2, got %d", parseErr.Line)
	}
	if !strings.Contains(err.Error(), "bad date") {
		t.Fatalf("error %q does not contain 'bad date'", err.Error())
	}
}

func TestParse_badRevenue(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,abc,10,NA\n"
	_, err := Parse([]byte(data))
	if err == nil {
		t.Fatal("expected error for bad revenue")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if parseErr.Line != 2 {
		t.Fatalf("expected line 2, got %d", parseErr.Line)
	}
	if !strings.Contains(err.Error(), "bad revenue") {
		t.Fatalf("error %q does not contain 'bad revenue'", err.Error())
	}
}

func TestParse_badUsers(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,100,xyz,NA\n"
	_, err := Parse([]byte(data))
	if err == nil {
		t.Fatal("expected error for bad users")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if parseErr.Line != 2 {
		t.Fatalf("expected line 2, got %d", parseErr.Line)
	}
	if !strings.Contains(err.Error(), "bad users") {
		t.Fatalf("error %q does not contain 'bad users'", err.Error())
	}
}

func TestParse_emptyRegion(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,100,10,\n"
	_, err := Parse([]byte(data))
	if err == nil {
		t.Fatal("expected error for empty region")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if parseErr.Line != 2 {
		t.Fatalf("expected line 2, got %d", parseErr.Line)
	}
	if !strings.Contains(err.Error(), "empty region") {
		t.Fatalf("error %q does not contain 'empty region'", err.Error())
	}
}

func TestParse_trailingNewline(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,12450.50,1840,NA\n\n"
	rows, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Region != "NA" {
		t.Fatalf("expected NA, got %q", rows[0].Region)
	}
}

func TestParse_crlfLineEndings(t *testing.T) {
	data := "date,revenue,users,region\r\n2026-01-01,12450.50,1840,NA\r\n2026-01-02,13110.00,1902,EU\r\n"
	rows, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Region != "NA" {
		t.Fatalf("row[0] expected NA, got %q", rows[0].Region)
	}
	if rows[1].Region != "EU" {
		t.Fatalf("row[1] expected EU, got %q", rows[1].Region)
	}
}

func TestParse_headerOnly(t *testing.T) {
	data := "date,revenue,users,region\n"
	rows, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(rows))
	}
}

func TestParse_wrongFieldCount(t *testing.T) {
	data := "date,revenue,users,region\n2026-01-01,100,10,NA,extra\n"
	_, err := Parse([]byte(data))
	if err == nil {
		t.Fatal("expected error for wrong field count")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected *ParseError, got %T", err)
	}
}

func TestParse_errorTypeAssertions(t *testing.T) {
	t.Run("ParseError", func(t *testing.T) {
		_, err := Parse([]byte("date,revenue,users,region\nbad-date,1,2,3\n"))
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Fatal("expected *ParseError")
		}
		if pe.Line != 2 {
			t.Fatalf("expected line 2, got %d", pe.Line)
		}
	})

	t.Run("HeaderMismatchError_via_As", func(t *testing.T) {
		_, err := Parse([]byte("a,b,c,d\n1,2,3,4\n"))
		var he *HeaderMismatchError
		if !errors.As(err, &he) {
			t.Fatal("expected *HeaderMismatchError")
		}
		if he.Expected != "date,revenue,users,region" {
			t.Fatalf("expected expected header to be %q, got %q", expectedHeader, he.Expected)
		}
	})

	t.Run("HeaderMismatchError_actual_header", func(t *testing.T) {
		_, err := Parse([]byte("a,b,c,d\n1,2,3,4\n"))
		var he *HeaderMismatchError
		if !errors.As(err, &he) {
			t.Fatal("expected *HeaderMismatchError")
		}
		if he.Actual != "a,b,c,d" {
			t.Fatalf("expected actual header 'a,b,c,d', got %q", he.Actual)
		}
	})
}
