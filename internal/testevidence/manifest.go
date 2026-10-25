// Package testevidence is the repository's single test-evidence engine. It
// selects required tests from a committed manifest, executes them with
// `go test -json`, and refuses to report success unless every required root
// and selected subtest actually ran and passed under the declared build mode,
// repetitions, budgets and artifacts.
//
// It is intentionally stdlib-only. Two executable consumers share it:
// scripts/drive-gate (the LDP drive manifest) and scripts/runtime-gate (the
// runtime performance/contract manifest).
package testevidence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Gate is one required evidence entry. All names are exact; a regular
// expression that happens to match tests is not an entry.
type Gate struct {
	ID               string   `json:"id"`
	Phase            string   `json:"phase"`
	Package          string   `json:"package"`
	Tests            []string `json:"tests"`
	Mode             string   `json:"mode"`
	Tags             []string `json:"tags"`
	Backend          string   `json:"backend"`
	Required         bool     `json:"required"`
	Repetitions      int      `json:"repetitions"`
	TestBudgetMS     int64    `json:"test_budget_ms"`
	PackageTimeoutMS int64    `json:"package_timeout_ms"`
	Fixture          string   `json:"fixture"`
	RequiredCaseIDs  []string `json:"required_case_ids"`
	Artifacts        []string `json:"artifacts"`
}

// Manifest is a committed evidence document.
type Manifest struct {
	// Version identifies the manifest grammar. Only version 1 is supported.
	Version int    `json:"version"`
	Gates   []Gate `json:"gates"`
	// FormatExcludes names package import paths whose parse errors are
	// intentional (malformed parser fixtures). They are excluded only from
	// the source-format gate, never from test execution.
	FormatExcludes []string `json:"format_excludes,omitempty"`
}

// Evidence modes.
const (
	ModeHost = "host"
	ModeRace = "race"
)

// LoadManifest decodes a manifest strictly: unknown fields are rejected so a
// misspelled requirement can never silently disappear from the gate.
func LoadManifest(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // committed manifest path
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("manifest %s: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("manifest %s: %w", path, err)
	}
	return &m, nil
}

// Validate rejects incomplete or ambiguous entries before any process runs.
func (m *Manifest) Validate() error {
	if m == nil {
		return errors.New("nil manifest")
	}
	if m.Version != 1 {
		return fmt.Errorf("unsupported manifest version %d", m.Version)
	}
	if len(m.Gates) == 0 {
		return errors.New("manifest declares no gates — an empty selection cannot pass")
	}
	seen := make(map[string]bool, len(m.Gates))
	for i := range m.Gates {
		g := &m.Gates[i]
		if g.ID == "" {
			return fmt.Errorf("gate %d: id is required", i)
		}
		if seen[g.ID] {
			return fmt.Errorf("gate %q: duplicate id", g.ID)
		}
		seen[g.ID] = true
		if g.Phase == "" {
			return fmt.Errorf("gate %q: phase is required", g.ID)
		}
		if g.Package == "" {
			return fmt.Errorf("gate %q: package is required", g.ID)
		}
		if len(g.Tests) == 0 {
			return fmt.Errorf("gate %q: at least one exact test name is required", g.ID)
		}
		for _, t := range g.Tests {
			if t == "" || strings.ContainsAny(t, " \t\n") {
				return fmt.Errorf("gate %q: invalid test name %q", g.ID, t)
			}
		}
		tests := make(map[string]bool, len(g.Tests))
		for _, t := range g.Tests {
			if tests[t] {
				return fmt.Errorf("gate %q: duplicate exact test %q", g.ID, t)
			}
			tests[t] = true
		}
		if g.Mode != ModeHost && g.Mode != ModeRace {
			return fmt.Errorf("gate %q: mode must be %q or %q, got %q", g.ID, ModeHost, ModeRace, g.Mode)
		}
		if g.Repetitions < 1 {
			return fmt.Errorf("gate %q: repetitions must be >= 1", g.ID)
		}
		if g.PackageTimeoutMS <= 0 {
			return fmt.Errorf("gate %q: package_timeout_ms must be positive", g.ID)
		}
		if g.TestBudgetMS < 0 {
			return fmt.Errorf("gate %q: test_budget_ms must not be negative", g.ID)
		}
		if g.Required && g.TestBudgetMS == 0 {
			return fmt.Errorf("gate %q: required gates need an explicit test budget", g.ID)
		}
		tags := make(map[string]bool, len(g.Tags))
		for _, tag := range g.Tags {
			if tag == "" || strings.ContainsAny(tag, ", \t\n") || tags[tag] {
				return fmt.Errorf("gate %q: invalid or duplicate build tag %q", g.ID, tag)
			}
			tags[tag] = true
		}
		artifacts := make(map[string]bool, len(g.Artifacts))
		for _, artifact := range g.Artifacts {
			clean := filepath.Clean(artifact)
			if artifact == "" || filepath.IsAbs(artifact) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return fmt.Errorf("gate %q: artifact path must stay within its evidence directory: %q", g.ID, artifact)
			}
			if artifacts[clean] {
				return fmt.Errorf("gate %q: duplicate artifact %q", g.ID, artifact)
			}
			artifacts[clean] = true
		}
		cases := make(map[string]bool, len(g.RequiredCaseIDs))
		for _, id := range g.RequiredCaseIDs {
			if id == "" || cases[id] {
				return fmt.Errorf("gate %q: empty or duplicate required case id %q", g.ID, id)
			}
			cases[id] = true
		}
	}
	return nil
}

// Select returns gates for a phase ("" means every phase) and mode ("" means
// both). The order is the manifest order, which keeps evidence stable.
func (m *Manifest) Select(phase, mode string) []Gate {
	out := make([]Gate, 0, len(m.Gates))
	for _, g := range m.Gates {
		if phase != "" && g.Phase != phase {
			continue
		}
		if mode != "" && g.Mode != mode {
			continue
		}
		out = append(out, g)
	}
	return out
}

// Phases lists the distinct manifest phases in declaration order.
func (m *Manifest) Phases() []string {
	seen := make(map[string]bool, len(m.Gates))
	out := make([]string, 0, len(m.Gates))
	for _, g := range m.Gates {
		if !seen[g.Phase] {
			seen[g.Phase] = true
			out = append(out, g.Phase)
		}
	}
	return out
}

// SortedPackages returns the distinct packages of a gate list.
func SortedPackages(gates []Gate) []string {
	seen := make(map[string]bool, len(gates))
	out := make([]string, 0, len(gates))
	for _, g := range gates {
		if !seen[g.Package] {
			seen[g.Package] = true
			out = append(out, g.Package)
		}
	}
	sort.Strings(out)
	return out
}

// ArtifactPath is the per-gate directory tests may write declared artifacts
// into. The runner requires every declared artifact to exist there.
func ArtifactPath(dir, gateID string) string {
	return filepath.Join(dir, gateID)
}
