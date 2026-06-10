// Package diag provides the diagnostic model and output reporters for lurpiclint.
//
// The diagnostic model (Severity, Teaching, Diagnostic) is the shared currency
// between rules, the rule engine, and all output formatters.  Every rule emits
// diagnostics; every reporter consumes them.
package diag

import (
	"fmt"
	"go/token"
	"sort"
)

// Severity classifies a diagnostic.
type Severity int

const (
	SeverityOff   Severity = -1 // not a real severity; used to disable a rule via config
	SeverityInfo  Severity = 0
	SeverityWarn  Severity = 1
	SeverityError Severity = 2
)

// String returns the lowercase label for a severity level.
func (s Severity) String() string {
	switch s {
	case SeverityOff:
		return "off"
	case SeverityInfo:
		return "info"
	case SeverityWarn:
		return "warn"
	case SeverityError:
		return "error"
	default:
		return fmt.Sprintf("severity(%d)", int(s))
	}
}

// SeverityFromString converts a string to a Severity.
// Returns false in the second position if the string is not recognised.
func SeverityFromString(s string) (Severity, bool) {
	switch s {
	case "off":
		return SeverityOff, true
	case "info":
		return SeverityInfo, true
	case "warn", "warning":
		return SeverityWarn, true
	case "error":
		return SeverityError, true
	default:
		return SeverityInfo, false
	}
}

// Teaching is the awareness triple that accompanies every diagnostic.
// It tells the author what they did, what they should use instead, and where
// to find it in the uxauthoring index.
type Teaching struct {
	Did      string // what the author did ("hand-rolled a child-arranging LayoutRole")
	UseThis  string // the existing capability ("structure/screen regions")
	IndexRef string // uxauthoring-index pointer ("marks/structure.Screen")
}

// Diagnostic is an individual finding produced by a rule.
type Diagnostic struct {
	RuleID   string           // "LL003"
	Severity Severity         // severity classification
	Pos      token.Position   // file:line:col of the primary span
	Message  string           // human-readable description
	Teach    Teaching         // awareness triple
	Related  []token.Position // secondary spans (e.g. each AddChild call)
}

// ByPosition implements sort.Interface for []*Diagnostic, ordering by
// (severity descending, file, line, column).  This gives a stable,
// deterministic order: errors first, then warnings, then info.
type ByPosition []*Diagnostic

func (b ByPosition) Len() int      { return len(b) }
func (b ByPosition) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b ByPosition) Less(i, j int) bool {
	di, dj := b[i], b[j]
	if di.Severity != dj.Severity {
		return di.Severity > dj.Severity // errors first
	}
	pi, pj := di.Pos, dj.Pos
	if pi.Filename != pj.Filename {
		return pi.Filename < pj.Filename
	}
	if pi.Line != pj.Line {
		return pi.Line < pj.Line
	}
	return pi.Column < pj.Column
}

// SortDiagnostics sorts a slice of diagnostics in place, deterministically.
func SortDiagnostics(d []*Diagnostic) {
	sort.Sort(ByPosition(d))
}

// MaxSeverity returns the highest severity present in the slice.
// Returns SeverityInfo if the slice is empty.
func MaxSeverity(d []*Diagnostic) Severity {
	max := SeverityInfo
	for _, di := range d {
		if di.Severity > max {
			max = di.Severity
		}
	}
	return max
}
