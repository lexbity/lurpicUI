package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// BaselineEntry records a single known finding that should be suppressed.
// File is stored repo-relative with forward slashes so the baseline is
// portable across checkout locations; LoadBaseline relativizes absolute
// entries that live under the module root.
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

// ModuleRoot walks up from start (the working directory when start is empty)
// to the nearest directory containing go.mod.  The result is symlink-resolved
// so comparisons against loader-reported absolute paths are stable.
// Returns "" when no module root is found.
func ModuleRoot(start string) string {
	dir := start
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return ""
		}
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// RelPath normalizes path against the module root: absolute paths under root
// become repo-relative with forward slashes; relative paths keep their form
// with forward slashes; anything else (foreign absolute paths, empty root) is
// returned with forward slashes unchanged, so it simply never matches and is
// reported stale instead of silently suppressing.
func RelPath(root, path string) string {
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) || root == "" {
		return filepath.ToSlash(path)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolvedPath = path
	}
	if resolvedPath == resolvedRoot {
		return "."
	}
	prefix := resolvedRoot + string(os.PathSeparator)
	if strings.HasPrefix(resolvedPath, prefix) {
		return filepath.ToSlash(resolvedPath[len(prefix):])
	}
	return filepath.ToSlash(path)
}

// LoadBaseline reads a baseline JSON file and relativizes every entry's File
// against root, so a baseline recorded with absolute paths on one machine
// still matches when the repo is checked out somewhere else.
func LoadBaseline(path, root string) (*Baseline, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path from user config
	if err != nil {
		return nil, fmt.Errorf("reading baseline %s: %w", path, err)
	}
	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parsing baseline %s: %w", path, err)
	}
	for i := range b.Entries {
		b.Entries[i].File = RelPath(root, b.Entries[i].File)
	}
	return &b, nil
}

// SuppressByBaseline filters diagnostics by removing those that match a
// baseline entry (same rule_id, file, line, message).  File paths on both
// sides are normalized repo-relative to root before comparison, making the
// match independent of where the module is checked out.  Returns the
// filtered slice and any stale baseline entries (entries that had no matching
// diagnostic).
func SuppressByBaseline(root string, diags []*diag.Diagnostic, baseline *Baseline) (filtered []*diag.Diagnostic, stale []BaselineEntry) {
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
			file:    RelPath(root, e.File),
			line:    e.Line,
			message: e.Message,
		}] = true
	}

	// Track which baseline entries were matched.
	matched := make(map[sig]bool)

	for _, d := range diags {
		s := sig{
			ruleID:  d.RuleID,
			file:    RelPath(root, d.Pos.Filename),
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
			file:    RelPath(root, e.File),
			line:    e.Line,
			message: e.Message,
		}
		if !matched[s] {
			stale = append(stale, e)
		}
	}

	return filtered, stale
}
