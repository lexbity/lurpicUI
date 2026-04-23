// Package model provides coverage enforcement mechanisms for the ui_catalog application.
// This file implements the coverage gate that detects missing entries and tracks coverage drift.
package model

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// CoverageGate validates that all required marks have catalog entries.
type CoverageGate struct {
	// RequiredIDs are the mark IDs that must exist in the catalog.
	RequiredIDs []string
	// CatalogEntries are the entries currently in the catalog.
	CatalogEntries []*CatalogEntry
	// MissingEntries tracks IDs that are required but not in the catalog.
	MissingEntries []string
	// PlaceholderEntries tracks entries with placeholder coverage status.
	PlaceholderEntries []string
	// DriftDetected indicates if new marks were added without catalog entries.
	DriftDetected bool
}

// NewCoverageGate creates a new coverage gate with the given required IDs and catalog entries.
func NewCoverageGate(requiredIDs []string, entries []*CatalogEntry) *CoverageGate {
	return &CoverageGate{
		RequiredIDs:    requiredIDs,
		CatalogEntries: entries,
	}
}

// Validate checks for missing entries and placeholder coverage.
// Returns an error if drift is detected (new marks without catalog entries).
func (g *CoverageGate) Validate() error {
	g.findMissingEntries()
	g.findPlaceholderEntries()

	if len(g.MissingEntries) > 0 {
		g.DriftDetected = true
		return fmt.Errorf("coverage drift detected: %d missing catalog entries: %s",
			len(g.MissingEntries), strings.Join(g.MissingEntries, ", "))
	}

	return nil
}

// ValidateWithWarnings checks for issues but returns warnings instead of errors for placeholders.
func (g *CoverageGate) ValidateWithWarnings() ([]string, error) {
	g.findMissingEntries()
	g.findPlaceholderEntries()

	var warnings []string

	if len(g.PlaceholderEntries) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d entries have placeholder coverage: %s",
			len(g.PlaceholderEntries), strings.Join(g.PlaceholderEntries, ", ")))
	}

	if len(g.MissingEntries) > 0 {
		g.DriftDetected = true
		return warnings, fmt.Errorf("coverage drift detected: %d missing catalog entries: %s",
			len(g.MissingEntries), strings.Join(g.MissingEntries, ", "))
	}

	return warnings, nil
}

// findMissingEntries identifies required IDs not present in the catalog.
func (g *CoverageGate) findMissingEntries() {
	entryMap := make(map[string]bool)
	for _, entry := range g.CatalogEntries {
		entryMap[entry.ID] = true
	}

	g.MissingEntries = nil
	for _, id := range g.RequiredIDs {
		if !entryMap[id] {
			g.MissingEntries = append(g.MissingEntries, id)
		}
	}
	sort.Strings(g.MissingEntries)
}

// findPlaceholderEntries identifies entries with placeholder coverage status.
func (g *CoverageGate) findPlaceholderEntries() {
	g.PlaceholderEntries = nil
	for _, entry := range g.CatalogEntries {
		if entry.Coverage == CoveragePlaceholder {
			g.PlaceholderEntries = append(g.PlaceholderEntries, entry.ID)
		}
	}
	sort.Strings(g.PlaceholderEntries)
}

// CoverageDiff represents the difference between two coverage states.
type CoverageDiff struct {
	Before  CoverageSnapshot `json:"before"`
	After   CoverageSnapshot `json:"after"`
	Changes DiffChanges      `json:"changes"`
}

// CoverageSnapshot captures coverage state at a point in time.
type CoverageSnapshot struct {
	Timestamp   string                    `json:"timestamp"`
	Total       int                       `json:"total"`
	Implemented int                       `json:"implemented"`
	Partial     int                       `json:"partial"`
	Placeholder int                       `json:"placeholder"`
	Missing     int                       `json:"missing"`
	Percent     float64                   `json:"percent"`
	Entries     map[string]CoverageStatus `json:"entries"`
}

// DiffChanges tracks what changed between snapshots.
type DiffChanges struct {
	NewImplemented  []string `json:"newImplemented"`
	NewPartial      []string `json:"newPartial"`
	NewPlaceholders []string `json:"newPlaceholders"`
	NewMissing      []string `json:"newMissing"`
	RemovedEntries  []string `json:"removedEntries"`
}

// NewCoverageSnapshot creates a snapshot from a list of catalog entries.
func NewCoverageSnapshot(entries []*CatalogEntry) CoverageSnapshot {
	snapshot := CoverageSnapshot{
		Timestamp: timeNow(),
		Entries:   make(map[string]CoverageStatus),
	}

	for _, entry := range entries {
		snapshot.Total++
		snapshot.Entries[entry.ID] = entry.Coverage

		switch entry.Coverage {
		case CoverageImplemented:
			snapshot.Implemented++
		case CoveragePartial:
			snapshot.Partial++
		case CoveragePlaceholder:
			snapshot.Placeholder++
		case CoverageMissing:
			snapshot.Missing++
		}
	}

	if snapshot.Total > 0 {
		snapshot.Percent = float64(snapshot.Implemented) / float64(snapshot.Total) * 100
	}

	return snapshot
}

// ComputeDiff calculates the difference between two coverage snapshots.
func ComputeDiff(before, after CoverageSnapshot) CoverageDiff {
	diff := CoverageDiff{
		Before: before,
		After:  after,
		Changes: DiffChanges{
			NewImplemented:  []string{},
			NewPartial:      []string{},
			NewPlaceholders: []string{},
			NewMissing:      []string{},
			RemovedEntries:  []string{},
		},
	}

	// Find new/changed entries
	for id, afterStatus := range after.Entries {
		beforeStatus, existed := before.Entries[id]
		if !existed {
			// New entry
			switch afterStatus {
			case CoverageImplemented:
				diff.Changes.NewImplemented = append(diff.Changes.NewImplemented, id)
			case CoveragePartial:
				diff.Changes.NewPartial = append(diff.Changes.NewPartial, id)
			case CoveragePlaceholder:
				diff.Changes.NewPlaceholders = append(diff.Changes.NewPlaceholders, id)
			case CoverageMissing:
				diff.Changes.NewMissing = append(diff.Changes.NewMissing, id)
			}
		} else if beforeStatus != afterStatus {
			// Status changed
			switch afterStatus {
			case CoverageImplemented:
				diff.Changes.NewImplemented = append(diff.Changes.NewImplemented, id)
			case CoveragePartial:
				diff.Changes.NewPartial = append(diff.Changes.NewPartial, id)
			case CoveragePlaceholder:
				diff.Changes.NewPlaceholders = append(diff.Changes.NewPlaceholders, id)
			case CoverageMissing:
				diff.Changes.NewMissing = append(diff.Changes.NewMissing, id)
			}
		}
	}

	// Find removed entries
	for id := range before.Entries {
		if _, exists := after.Entries[id]; !exists {
			diff.Changes.RemovedEntries = append(diff.Changes.RemovedEntries, id)
		}
	}

	// Sort all change lists for deterministic output
	sort.Strings(diff.Changes.NewImplemented)
	sort.Strings(diff.Changes.NewPartial)
	sort.Strings(diff.Changes.NewPlaceholders)
	sort.Strings(diff.Changes.NewMissing)
	sort.Strings(diff.Changes.RemovedEntries)

	return diff
}

// timeNow returns current timestamp in RFC3339 format.
func timeNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// CoverageReport provides detailed coverage information.
type CoverageReport struct {
	Summary       CoverageSummary         `json:"summary"`
	ByFamily      map[string]FamilyReport `json:"byFamily"`
	Missing       []string                `json:"missing"`
	Placeholders  []string                `json:"placeholders"`
	Partial       []string                `json:"partial"`
	DriftDetected bool                    `json:"driftDetected"`
}

// FamilyReport contains coverage stats for a specific family.
type FamilyReport struct {
	Total       int     `json:"total"`
	Implemented int     `json:"implemented"`
	Percent     float64 `json:"percent"`
}

// GenerateCoverageReport creates a comprehensive coverage report.
func GenerateCoverageReport(entries []*CatalogEntry, requiredIDs []string) CoverageReport {
	report := CoverageReport{
		ByFamily:     make(map[string]FamilyReport),
		Missing:      []string{},
		Placeholders: []string{},
		Partial:      []string{},
	}

	gate := NewCoverageGate(requiredIDs, entries)
	gate.Validate() // This populates MissingEntries and PlaceholderEntries

	report.Missing = gate.MissingEntries
	report.Placeholders = gate.PlaceholderEntries
	report.DriftDetected = gate.DriftDetected

	// Calculate summary stats
	var total, implemented, partial, placeholder, missing int
	for _, entry := range entries {
		total++

		// Update family stats
		familyStr := entry.Family.String()
		familyReport := report.ByFamily[familyStr]
		familyReport.Total++

		switch entry.Coverage {
		case CoverageImplemented:
			implemented++
			familyReport.Implemented++
		case CoveragePartial:
			partial++
			report.Partial = append(report.Partial, entry.ID)
		case CoveragePlaceholder:
			placeholder++
		case CoverageMissing:
			missing++
		}

		report.ByFamily[familyStr] = familyReport
	}

	// Calculate percentages for families
	for family, stats := range report.ByFamily {
		if stats.Total > 0 {
			stats.Percent = float64(stats.Implemented) / float64(stats.Total) * 100
			report.ByFamily[family] = stats
		}
	}

	report.Summary = CoverageSummary{
		Total:       total,
		Implemented: implemented,
		Partial:     partial,
		Placeholder: placeholder,
		Missing:     missing,
		Percent:     0,
	}

	if total > 0 {
		report.Summary.Percent = float64(implemented) / float64(total) * 100
	}

	sort.Strings(report.Partial)

	return report
}

// CoverageSummary provides high-level coverage statistics.
type CoverageSummary struct {
	Total       int     `json:"total"`
	Implemented int     `json:"implemented"`
	Partial     int     `json:"partial"`
	Placeholder int     `json:"placeholder"`
	Missing     int     `json:"missing"`
	Percent     float64 `json:"percent"`
}
