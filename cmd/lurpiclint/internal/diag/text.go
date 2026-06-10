package diag

import (
	"fmt"
	"io"
)

// TextReporter emits diagnostics in a human-readable format.
// Each diagnostic is printed as:
//
//	file:line:col: SEVERITY rule-id: message
//	       help: did – use_this (index_ref)
//
// The help line is omitted when the teaching triple is empty.
type TextReporter struct {
	w     io.Writer
	color bool
}

func (r *TextReporter) Name() string { return "text" }

func (r *TextReporter) Report(diags []*Diagnostic) error {
	if len(diags) == 0 {
		return nil
	}

	for _, d := range diags {
		pos := d.Pos
		sev := d.Severity.String()

		_, err := fmt.Fprintf(r.w, "%s:%d:%d: %s %s: %s\n",
			pos.Filename, pos.Line, pos.Column, sev, d.RuleID, d.Message)
		if err != nil {
			return err
		}

		if d.Teach.Did != "" || d.Teach.UseThis != "" || d.Teach.IndexRef != "" {
			help := d.Teach.Did
			if d.Teach.UseThis != "" {
				help += " — use " + d.Teach.UseThis
			}
			if d.Teach.IndexRef != "" {
				help += " (" + d.Teach.IndexRef + ")"
			}
			_, err := fmt.Fprintf(r.w, "       help: %s\n", help)
			if err != nil {
				return err
			}
		}

		// Related spans as secondary lines.
		for _, rel := range d.Related {
			_, err := fmt.Fprintf(r.w, "       see: %s:%d:%d\n",
				rel.Filename, rel.Line, rel.Column)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
