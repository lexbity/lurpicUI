package dataset

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const dateFormat = "2006-01-02"

var expectedHeader = []string{"date", "revenue", "users", "region"}

type Row struct {
	Date    time.Time
	Revenue float64
	Users   float64
	Region  string
}

type ParseError struct {
	Line   int
	Column string
	Value  string
	Err    error
}

func (e *ParseError) Error() string {
	if e.Column != "" {
		return fmt.Sprintf("line %d, column %q: %v", e.Line, e.Column, e.Err)
	}
	return fmt.Sprintf("line %d: %v", e.Line, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

type ParseErrors struct {
	Errors []*ParseError
}

func (e *ParseErrors) Error() string {
	switch len(e.Errors) {
	case 0:
		return "no parse errors"
	case 1:
		return e.Errors[0].Error()
	default:
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%d parse errors", len(e.Errors)))
		for _, pe := range e.Errors {
			b.WriteString("\n  " + pe.Error())
		}
		return b.String()
	}
}

func (e *ParseErrors) Empty() bool {
	return len(e.Errors) == 0
}

type HeaderError struct {
	Expected []string
	Got      []string
}

func (e *HeaderError) Error() string {
	return fmt.Sprintf("expected header %q, got %q", e.Expected, e.Got)
}

func Parse(raw []byte) ([]Row, error) {
	if len(raw) == 0 {
		return nil, &ParseError{Line: 0, Err: fmt.Errorf("file is empty")}
	}

	reader := csv.NewReader(toReader(raw))
	reader.ReuseRecord = true
	reader.FieldsPerRecord = 0

	rawHeader, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, &ParseError{Line: 0, Err: fmt.Errorf("file contains only a header")}
		}
		return nil, &ParseError{Line: 0, Err: fmt.Errorf("reading header: %w", err)}
	}

	if !headersMatch(rawHeader, expectedHeader) {
		return nil, &HeaderError{Expected: expectedHeader, Got: rawHeader}
	}

	var rows []Row
	var errs []*ParseError
	lineNum := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		lineNum++
		if err != nil {
			errs = append(errs, &ParseError{Line: lineNum, Err: fmt.Errorf("reading record: %w", err)})
			continue
		}

		row, parseErrs := parseRow(record, lineNum)
		if len(parseErrs) > 0 {
			errs = append(errs, parseErrs...)
			continue
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 && len(errs) == 0 {
		return nil, &ParseError{Line: 0, Err: fmt.Errorf("file contains no data rows")}
	}

	if len(errs) > 0 {
		return rows, &ParseErrors{Errors: errs}
	}

	return rows, nil
}

func headersMatch(got, expected []string) bool {
	if len(got) != len(expected) {
		return false
	}
	for i := range got {
		if got[i] != expected[i] {
			return false
		}
	}
	return true
}

func parseRow(record []string, lineNum int) (Row, []*ParseError) {
	if len(record) < 4 {
		return Row{}, []*ParseError{
			{Line: lineNum, Err: fmt.Errorf("expected 4 columns, got %d", len(record))},
		}
	}
	if len(record) > 4 {
		return Row{}, []*ParseError{
			{Line: lineNum, Err: fmt.Errorf("expected 4 columns, got %d", len(record))},
		}
	}

	var row Row
	var errs []*ParseError

	dateStr := strings.TrimSpace(record[0])
	date, err := time.Parse(dateFormat, dateStr)
	if err != nil {
		errs = append(errs, &ParseError{Line: lineNum, Column: "date", Value: dateStr, Err: fmt.Errorf("invalid date: %w", err)})
	} else {
		row.Date = date
	}

	revenueStr := strings.TrimSpace(record[1])
	revenue, err := strconv.ParseFloat(revenueStr, 64)
	if err != nil {
		errs = append(errs, &ParseError{Line: lineNum, Column: "revenue", Value: revenueStr, Err: fmt.Errorf("invalid float: %w", err)})
	} else {
		row.Revenue = revenue
	}

	usersStr := strings.TrimSpace(record[2])
	users, err := strconv.ParseFloat(usersStr, 64)
	if err != nil {
		errs = append(errs, &ParseError{Line: lineNum, Column: "users", Value: usersStr, Err: fmt.Errorf("invalid float: %w", err)})
	} else {
		row.Users = users
	}

	region := strings.TrimSpace(record[3])
	if region == "" {
		errs = append(errs, &ParseError{Line: lineNum, Column: "region", Value: region, Err: fmt.Errorf("region is empty")})
	} else {
		row.Region = region
	}

	return row, errs
}

func toReader(raw []byte) io.Reader {
	return strings.NewReader(string(raw))
}
