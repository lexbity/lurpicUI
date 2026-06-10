package diag

import (
	"bytes"
	"encoding/json"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fixedDiagnostics returns a deterministic set of diagnostics used by all
// golden-output tests.  The set contains one diagnostic of each severity,
// with various teaching and related-spans combinations.
func fixedDiagnostics() []*Diagnostic {
	return []*Diagnostic{
		{
			RuleID:   "LL001",
			Severity: SeverityWarn,
			Pos:      token.Position{Filename: "app.go", Line: 8, Column: 5},
			Message:  "raw LayoutRole literal with OnArrange set",
			Teach: Teaching{
				Did:     "populated a LayoutRole struct directly",
				UseThis: "an existing layout container or mark",
			},
		},
		{
			RuleID:   "LL003",
			Severity: SeverityError,
			Pos:      token.Position{Filename: "app.go", Line: 10, Column: 5},
			Message:  "hand-rolled layout container: arrange loop over child facets",
			Teach: Teaching{
				Did:      "wrote a LayoutRole that positions children",
				UseThis:  "structure/panel",
				IndexRef: "marks/structure.Panel",
			},
			Related: []token.Position{
				{Filename: "app.go", Line: 15, Column: 2},
				{Filename: "app.go", Line: 22, Column: 2},
			},
		},
		{
			RuleID:   "LL004",
			Severity: SeverityInfo,
			Pos:      token.Position{Filename: "custom.go", Line: 3, Column: 1},
			Message:  "child-arranging facet matches built-in RowLayout",
			Teach: Teaching{
				Did:      "defined a custom facet that arranges children horizontally",
				UseThis:  "layout.RowLayout",
				IndexRef: "layout/linear.Row",
			},
		},
	}
}

// goldenPath returns the path to a golden file by name.
func goldenPath(tb testing.TB, name string) string {
	tb.Helper()
	return filepath.Join("testdata", "golden", name)
}

func readGolden(tb testing.TB, name string) string {
	tb.Helper()
	data, err := os.ReadFile(goldenPath(tb, name))
	if err != nil {
		tb.Fatal(err)
	}
	return string(data)
}

func TestTextReporter_Golden(t *testing.T) {
	diags := fixedDiagnostics()
	SortDiagnostics(diags)

	var buf bytes.Buffer
	r := &TextReporter{w: &buf}
	if err := r.Report(diags); err != nil {
		t.Fatal(err)
	}

	want := readGolden(t, "text.golden")
	got := buf.String()
	if got != want {
		t.Errorf("text output mismatch\nwant:\n%s\n---\ngot:\n%s", want, got)
	}
}

func TestJSONReporter_Golden(t *testing.T) {
	diags := fixedDiagnostics()
	SortDiagnostics(diags)

	var buf bytes.Buffer
	r := &JSONReporter{w: &buf}
	if err := r.Report(diags); err != nil {
		t.Fatal(err)
	}

	want := readGolden(t, "json.golden")
	got := buf.String()
	if got != want {
		t.Errorf("json output mismatch\nwant:\n%s\n---\ngot:\n%s", want, got)
	}
}

func TestGitHubReporter_Golden(t *testing.T) {
	diags := fixedDiagnostics()
	SortDiagnostics(diags)

	var buf bytes.Buffer
	r := &GitHubReporter{w: &buf}
	if err := r.Report(diags); err != nil {
		t.Fatal(err)
	}

	want := readGolden(t, "github.golden")
	got := buf.String()
	if got != want {
		t.Errorf("github output mismatch\nwant:\n%s\n---\ngot:\n%s", want, got)
	}
}

func TestAllReporters_Deterministic(t *testing.T) {
	// Render the same set twice and assert byte-identical output.
	diags := fixedDiagnostics()
	SortDiagnostics(diags)

	formats := []struct {
		name string
		new  func(*bytes.Buffer) Reporter
	}{
		{"text", func(buf *bytes.Buffer) Reporter { return &TextReporter{w: buf} }},
		{"json", func(buf *bytes.Buffer) Reporter { return &JSONReporter{w: buf} }},
		{"github", func(buf *bytes.Buffer) Reporter { return &GitHubReporter{w: buf} }},
	}

	for _, f := range formats {
		t.Run(f.name, func(t *testing.T) {
			var b1, b2 bytes.Buffer
			r1 := f.new(&b1)
			r2 := f.new(&b2)

			if err := r1.Report(diags); err != nil {
				t.Fatal(err)
			}
			if err := r2.Report(diags); err != nil {
				t.Fatal(err)
			}

			if b1.String() != b2.String() {
				t.Error("non-deterministic output between runs")
			}
		})
	}
}

func TestEmptyDiagnostics(t *testing.T) {
	diags := []*Diagnostic{}

	formats := []struct {
		name string
		new  func(*bytes.Buffer) Reporter
	}{
		{"text", func(buf *bytes.Buffer) Reporter { return &TextReporter{w: buf} }},
		{"json", func(buf *bytes.Buffer) Reporter { return &JSONReporter{w: buf} }},
		{"github", func(buf *bytes.Buffer) Reporter { return &GitHubReporter{w: buf} }},
	}

	for _, f := range formats {
		t.Run(f.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := f.new(&buf)
			if err := r.Report(diags); err != nil {
				t.Fatal(err)
			}
			got := buf.String()
			if f.name == "json" {
				// The JSON reporter always emits a valid envelope.
				if !strings.Contains(got, `"diagnostics": []`) {
					t.Errorf("json output should contain empty diagnostics array, got %q", got)
				}
			} else if got != "" {
				t.Errorf("expected empty output for zero diagnostics, got %q", got)
			}
		})
	}
}

func TestSortDiagnostics_Stability(t *testing.T) {
	// Create diagnostics with known severity and position, then verify order.
	diags := []*Diagnostic{
		{RuleID: "D", Severity: SeverityWarn, Pos: token.Position{Filename: "a.go", Line: 5, Column: 1}},
		{RuleID: "A", Severity: SeverityError, Pos: token.Position{Filename: "a.go", Line: 1, Column: 1}},
		{RuleID: "C", Severity: SeverityInfo, Pos: token.Position{Filename: "b.go", Line: 1, Column: 1}},
		{RuleID: "B", Severity: SeverityError, Pos: token.Position{Filename: "a.go", Line: 10, Column: 1}},
	}

	SortDiagnostics(diags)

	// Expected order: errors first (by file/line), then warnings, then info.
	// A (error, a.go:1), B (error, a.go:10), D (warn, a.go:5), C (info, b.go:1)
	expected := []string{"A", "B", "D", "C"}
	for i, d := range diags {
		if d.RuleID != expected[i] {
			t.Errorf("position %d: got %s, want %s", i, d.RuleID, expected[i])
		}
	}
}

func TestSortDiagnostics_SamePositionDifferentSeverity(t *testing.T) {
	// When positions are identical, errors sort before warnings before info.
	pos := token.Position{Filename: "x.go", Line: 1, Column: 1}
	diags := []*Diagnostic{
		{RuleID: "info", Severity: SeverityInfo, Pos: pos},
		{RuleID: "warn", Severity: SeverityWarn, Pos: pos},
		{RuleID: "error", Severity: SeverityError, Pos: pos},
	}

	SortDiagnostics(diags)

	expected := []string{"error", "warn", "info"}
	for i, d := range diags {
		if d.RuleID != expected[i] {
			t.Errorf("position %d: got %s, want %s", i, d.RuleID, expected[i])
		}
	}
}

func TestSortDiagnostics_Deterministic(t *testing.T) {
	diags := fixedDiagnostics()

	SortDiagnostics(diags)
	sorted1 := diagnosticIDs(diags)

	// Sort again — must be stable (no change).
	SortDiagnostics(diags)
	sorted2 := diagnosticIDs(diags)

	if len(sorted1) != len(sorted2) {
		t.Fatal("length changed after re-sorting")
	}
	for i := range sorted1 {
		if sorted1[i] != sorted2[i] {
			t.Errorf("position %d: first=%s second=%s", i, sorted1[i], sorted2[i])
		}
	}
}

func TestMaxSeverity(t *testing.T) {
	tests := []struct {
		name  string
		diags []*Diagnostic
		want  Severity
	}{
		{"empty", nil, SeverityInfo},
		{"all info", []*Diagnostic{{Severity: SeverityInfo}, {Severity: SeverityInfo}}, SeverityInfo},
		{"mix", []*Diagnostic{{Severity: SeverityInfo}, {Severity: SeverityWarn}}, SeverityWarn},
		{"error present", []*Diagnostic{{Severity: SeverityInfo}, {Severity: SeverityError}}, SeverityError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaxSeverity(tt.diags)
			if got != tt.want {
				t.Errorf("MaxSeverity = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSeverityFromString(t *testing.T) {
	tests := []struct {
		input string
		want  Severity
		ok    bool
	}{
		{"off", SeverityOff, true},
		{"info", SeverityInfo, true},
		{"warn", SeverityWarn, true},
		{"warning", SeverityWarn, true},
		{"error", SeverityError, true},
		{"unknown", SeverityInfo, false},
		{"", SeverityInfo, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := SeverityFromString(tt.input)
			if got != tt.want || ok != tt.ok {
				t.Errorf("SeverityFromString(%q) = %d, %v; want %d, %v",
					tt.input, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		s    Severity
		want string
	}{
		{SeverityOff, "off"},
		{SeverityInfo, "info"},
		{SeverityWarn, "warn"},
		{SeverityError, "error"},
		{Severity(99), "severity(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if tt.s.String() != tt.want {
				t.Errorf("Severity(%d).String() = %q, want %q", tt.s, tt.s.String(), tt.want)
			}
		})
	}
}

func TestJSONReporter_RoundTrip(t *testing.T) {
	// Marshal diagnostics to JSON, unmarshal back, verify schema.
	diags := fixedDiagnostics()
	SortDiagnostics(diags)

	var buf bytes.Buffer
	r := &JSONReporter{w: &buf}
	if err := r.Report(diags); err != nil {
		t.Fatal(err)
	}

	// Unmarshal into envelope.
	var out jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if out.SchemaVersion != schemaVersion {
		t.Errorf("schema_version = %d, want %d", out.SchemaVersion, schemaVersion)
	}

	if len(out.Diagnostics) != len(diags) {
		t.Fatalf("diagnostic count: got %d, want %d", len(out.Diagnostics), len(diags))
	}

	// Verify round-trip preserves key fields.
	for i, jd := range out.Diagnostics {
		d := diags[i]
		if jd.RuleID != d.RuleID {
			t.Errorf("diagnostic %d: rule_id = %q, want %q", i, jd.RuleID, d.RuleID)
		}
		if jd.Severity != d.Severity.String() {
			t.Errorf("diagnostic %d: severity = %q, want %q", i, jd.Severity, d.Severity.String())
		}
		if jd.Pos.File != d.Pos.Filename {
			t.Errorf("diagnostic %d: file = %q, want %q", i, jd.Pos.File, d.Pos.Filename)
		}
		if jd.Pos.Line != d.Pos.Line {
			t.Errorf("diagnostic %d: line = %d, want %d", i, jd.Pos.Line, d.Pos.Line)
		}
		if jd.Pos.Column != d.Pos.Column {
			t.Errorf("diagnostic %d: column = %d, want %d", i, jd.Pos.Column, d.Pos.Column)
		}
	}
}

func TestNewReporter(t *testing.T) {
	var buf bytes.Buffer

	formats := []string{"text", "json", "github"}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			r, err := NewReporter(format, &buf)
			if err != nil {
				t.Fatalf("NewReporter(%q) error: %v", format, err)
			}
			if r.Name() != format {
				t.Errorf("Name() = %q, want %q", r.Name(), format)
			}
		})
	}

	// Unknown format.
	_, err := NewReporter("bogus", &buf)
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

// diagnosticIDs extracts RuleID values from a slice, preserving order.
func diagnosticIDs(diags []*Diagnostic) []string {
	ids := make([]string, len(diags))
	for i, d := range diags {
		ids[i] = d.RuleID
	}
	return ids
}

// Ensure sort.Interface is satisfied at compile time.
var _ sort.Interface = ByPosition{}
