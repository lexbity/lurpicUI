package diag

import (
	"fmt"
	"io"
	"os"
)

// Reporter writes a set of diagnostics to an output stream in a specific
// format.  Implementations must be safe for concurrent use only if explicitly
// documented.
type Reporter interface {
	// Report emits all diagnostics.  The slice may be empty, in which case
	// the reporter should produce no output (or a summary with zero
	// findings).
	Report([]*Diagnostic) error

	// Name returns a short human-readable label for the format.
	Name() string
}

// NewReporter returns a Reporter for the named format.
//
// Supported formats:
//
//	"text"   — human-readable, optionally colourised on TTY
//	"json"   — versioned JSON schema for LLM consumption
//	"github" — GitHub Actions workflow command annotations
func NewReporter(format string, w io.Writer) (Reporter, error) {
	switch format {
	case "text":
		_, isTTY := w.(*os.File)
		return &TextReporter{w: w, color: isTTY}, nil
	case "json":
		return &JSONReporter{w: w}, nil
	case "github":
		return &GitHubReporter{w: w}, nil
	default:
		return nil, fmt.Errorf("unknown reporter format %q", format)
	}
}
