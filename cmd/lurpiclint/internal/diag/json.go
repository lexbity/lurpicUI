package diag

import (
	"encoding/json"
	"io"
)

// schemaVersion is the version of the JSON output schema.
// It is incremented when the schema changes in a backward-incompatible way.
const schemaVersion = 1

// jsonOutput is the top-level JSON envelope.
type jsonOutput struct {
	SchemaVersion int              `json:"schema_version"`
	Diagnostics   []jsonDiagnostic `json:"diagnostics"`
}

// jsonDiagnostic is the JSON-serializable representation of a Diagnostic.
type jsonDiagnostic struct {
	RuleID   string        `json:"rule_id"`
	Severity string        `json:"severity"`
	Pos      jsonPos       `json:"pos"`
	Message  string        `json:"message"`
	Teaching jsonTeaching  `json:"teaching,omitempty"`
	Related  []jsonPos     `json:"related,omitempty"`
}

type jsonPos struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type jsonTeaching struct {
	Did      string `json:"did,omitempty"`
	UseThis  string `json:"use_this,omitempty"`
	IndexRef string `json:"index_ref,omitempty"`
}

// JSONReporter emits diagnostics as a versioned JSON document.
type JSONReporter struct {
	w io.Writer
}

func (r *JSONReporter) Name() string { return "json" }

func (r *JSONReporter) Report(diags []*Diagnostic) error {
	out := jsonOutput{
		SchemaVersion: schemaVersion,
		Diagnostics:   make([]jsonDiagnostic, 0, len(diags)),
	}

	for _, d := range diags {
		jd := jsonDiagnostic{
			RuleID:   d.RuleID,
			Severity: d.Severity.String(),
			Pos: jsonPos{
				File:   d.Pos.Filename,
				Line:   d.Pos.Line,
				Column: d.Pos.Column,
			},
			Message: d.Message,
		}

		if d.Teach.Did != "" || d.Teach.UseThis != "" || d.Teach.IndexRef != "" {
			jd.Teaching = jsonTeaching{
				Did:      d.Teach.Did,
				UseThis:  d.Teach.UseThis,
				IndexRef: d.Teach.IndexRef,
			}
		}

		if len(d.Related) > 0 {
			jd.Related = make([]jsonPos, 0, len(d.Related))
			for _, rel := range d.Related {
				jd.Related = append(jd.Related, jsonPos{
					File:   rel.Filename,
					Line:   rel.Line,
					Column: rel.Column,
				})
			}
		}

		out.Diagnostics = append(out.Diagnostics, jd)
	}

	enc := json.NewEncoder(r.w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
