package dataset

import (
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const expectedHeader = "date,revenue,users,region"

const dateFormat = "2006-01-02"

type Row struct {
	Date    time.Time
	Revenue float64
	Users   float64
	Region  string
}

type ParseError struct {
	Line int
	Err  error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %v", e.Line, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

type HeaderMismatchError struct {
	Expected string
	Actual   string
}

func (e *HeaderMismatchError) Error() string {
	return fmt.Sprintf("expected header %q, got %q", e.Expected, e.Actual)
}

type EmptyInputError struct{}

func (e *EmptyInputError) Error() string {
	return "empty input"
}

func Parse(raw []byte) ([]Row, error) {
	if len(raw) == 0 {
		return nil, &EmptyInputError{}
	}

	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.TrimLeadingSpace = false
	reader.FieldsPerRecord = 4

	records, err := reader.ReadAll()
	if err != nil {
		var csvErr *csv.ParseError
		if errors.As(err, &csvErr) {
			return nil, &ParseError{Line: csvErr.Line, Err: err}
		}
		return nil, &ParseError{Line: 1, Err: err}
	}

	gotHeader := strings.Join(records[0], ",")
	if gotHeader != strings.TrimSpace(expectedHeader) {
		return nil, &ParseError{
			Line: 1,
			Err:  &HeaderMismatchError{Expected: expectedHeader, Actual: gotHeader},
		}
	}

	rows := make([]Row, 0, len(records)-1)

	for i := 1; i < len(records); i++ {
		fields := records[i]
		var row Row

		dateStr := strings.TrimSpace(fields[0])
		t, err := time.Parse(dateFormat, dateStr)
		if err != nil {
			return nil, &ParseError{
				Line: i + 1,
				Err:  fmt.Errorf("bad date %q: %w", dateStr, err),
			}
		}
		row.Date = t

		revenueStr := strings.TrimSpace(fields[1])
		revenue, err := strconv.ParseFloat(revenueStr, 64)
		if err != nil {
			return nil, &ParseError{
				Line: i + 1,
				Err:  fmt.Errorf("bad revenue %q: %w", revenueStr, err),
			}
		}
		row.Revenue = revenue

		usersStr := strings.TrimSpace(fields[2])
		users, err := strconv.ParseFloat(usersStr, 64)
		if err != nil {
			return nil, &ParseError{
				Line: i + 1,
				Err:  fmt.Errorf("bad users %q: %w", usersStr, err),
			}
		}
		row.Users = users

		row.Region = strings.TrimSpace(fields[3])
		if row.Region == "" {
			return nil, &ParseError{
				Line: i + 1,
				Err:  fmt.Errorf("empty region"),
			}
		}

		rows = append(rows, row)
	}

	return rows, nil
}
