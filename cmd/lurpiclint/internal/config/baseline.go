package config

import (
	"encoding/json"
	"fmt"
	"os"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// BaselineEntry records a single known finding that should be suppressed.
type BaselineEntry struct {
	RuleID  string `json:"rule_id"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// Baseline stores a set of known findings.
type Baseline struct {
	Entries []BaselineEntry `json:"entries"`
}

// LoadBaseline reads a baseline JSON file.
func LoadBaseline(path string) (*Baseline, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path from user config
	if err != nil {
		return nil, fmt.Errorf("reading baseline %s: %w", path, err)
	}
	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parsing baseline %s: %w", path, err)
	}
	return &b, nil
}

// SuppressByBaseline filters diagnostics by removing those that match a
// baseline entry (same rule_id, file, line, message).  Returns the filtered
// slice and any stale baseline entries (entries that had no matching
// diagnostic).
func SuppressByBaseline(diags []*diag.Diagnostic, baseline *Baseline) (filtered []*diag.Diagnostic, stale []BaselineEntry) {
	if baseline == nil {
		return diags, nil
	}

	// Build a set of baseline signatures for O(1) lookup.
	type sig struct {
		ruleID  string
		file    string
		line    int
		message string
	}
	baselineSet := make(map[sig]bool, len(baseline.Entries))
	for _, e := range baseline.Entries {
		baselineSet[sig{
			ruleID:  e.RuleID,
			file:    e.File,
			line:    e.Line,
			message: e.Message,
		}] = true
	}

	// Track which baseline entries were matched.
	matched := make(map[sig]bool)

	for _, d := range diags {
		s := sig{
			ruleID:  d.RuleID,
			file:    d.Pos.Filename,
			line:    d.Pos.Line,
			message: d.Message,
		}
		if baselineSet[s] {
			matched[s] = true
			continue // suppressed
		}
		filtered = append(filtered, d)
	}

	// Find stale baseline entries (never matched).
	for _, e := range baseline.Entries {
		s := sig{
			ruleID:  e.RuleID,
			file:    e.File,
			line:    e.Line,
			message: e.Message,
		}
		if !matched[s] {
			stale = append(stale, e)
		}
	}

	return filtered, stale
}
