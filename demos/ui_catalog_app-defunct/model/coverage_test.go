package model

import (
	"testing"
)

func TestCoverageGate_Validate_NoMissingEntries(t *testing.T) {
	requiredIDs := []string{"mark-1", "mark-2", "mark-3"}
	entries := []*CatalogEntry{
		{ID: "mark-1", Coverage: CoverageImplemented},
		{ID: "mark-2", Coverage: CoveragePartial},
		{ID: "mark-3", Coverage: CoveragePlaceholder},
	}

	gate := NewCoverageGate(requiredIDs, entries)
	err := gate.Validate()

	if err != nil {
		t.Errorf("Validate() should not error when all entries exist, got: %v", err)
	}
	if gate.DriftDetected {
		t.Error("DriftDetected should be false when all entries exist")
	}
	if len(gate.MissingEntries) != 0 {
		t.Errorf("MissingEntries should be empty, got: %v", gate.MissingEntries)
	}
}

func TestCoverageGate_Validate_MissingEntries(t *testing.T) {
	requiredIDs := []string{"mark-1", "mark-2", "mark-3", "mark-4"}
	entries := []*CatalogEntry{
		{ID: "mark-1", Coverage: CoverageImplemented},
		{ID: "mark-2", Coverage: CoveragePartial},
	}

	gate := NewCoverageGate(requiredIDs, entries)
	err := gate.Validate()

	if err == nil {
		t.Error("Validate() should error when entries are missing")
	}
	if !gate.DriftDetected {
		t.Error("DriftDetected should be true when entries are missing")
	}
	if len(gate.MissingEntries) != 2 {
		t.Errorf("MissingEntries should contain 2 entries, got: %v", gate.MissingEntries)
	}
	if !contains(gate.MissingEntries, "mark-3") || !contains(gate.MissingEntries, "mark-4") {
		t.Errorf("MissingEntries should contain mark-3 and mark-4, got: %v", gate.MissingEntries)
	}
}

func TestCoverageGate_ValidateWithWarnings_Placeholders(t *testing.T) {
	requiredIDs := []string{"mark-1", "mark-2"}
	entries := []*CatalogEntry{
		{ID: "mark-1", Coverage: CoverageImplemented},
		{ID: "mark-2", Coverage: CoveragePlaceholder},
	}

	gate := NewCoverageGate(requiredIDs, entries)
	warnings, err := gate.ValidateWithWarnings()

	if err != nil {
		t.Errorf("ValidateWithWarnings() should not error for placeholders, got: %v", err)
	}
	if len(warnings) != 1 {
		t.Errorf("Expected 1 warning for placeholder, got: %v", warnings)
	}
	if len(gate.PlaceholderEntries) != 1 || gate.PlaceholderEntries[0] != "mark-2" {
		t.Errorf("PlaceholderEntries should contain mark-2, got: %v", gate.PlaceholderEntries)
	}
}

func TestCoverageGate_ValidateWithWarnings_MissingAndPlaceholders(t *testing.T) {
	requiredIDs := []string{"mark-1", "mark-2", "mark-3"}
	entries := []*CatalogEntry{
		{ID: "mark-1", Coverage: CoverageImplemented},
		{ID: "mark-2", Coverage: CoveragePlaceholder},
	}

	gate := NewCoverageGate(requiredIDs, entries)
	warnings, err := gate.ValidateWithWarnings()

	if err == nil {
		t.Error("ValidateWithWarnings() should error when entries are missing")
	}
	if len(warnings) != 1 {
		t.Errorf("Expected 1 warning for placeholder, got: %v", warnings)
	}
	if !gate.DriftDetected {
		t.Error("DriftDetected should be true when entries are missing")
	}
}

func TestCoverageGate_findPlaceholderEntries(t *testing.T) {
	entries := []*CatalogEntry{
		{ID: "mark-1", Coverage: CoverageImplemented},
		{ID: "mark-2", Coverage: CoveragePlaceholder},
		{ID: "mark-3", Coverage: CoveragePlaceholder},
		{ID: "mark-4", Coverage: CoveragePartial},
	}

	gate := NewCoverageGate(nil, entries)
	gate.findPlaceholderEntries()

	if len(gate.PlaceholderEntries) != 2 {
		t.Errorf("Expected 2 placeholder entries, got: %v", gate.PlaceholderEntries)
	}
	if !contains(gate.PlaceholderEntries, "mark-2") || !contains(gate.PlaceholderEntries, "mark-3") {
		t.Errorf("PlaceholderEntries should contain mark-2 and mark-3, got: %v", gate.PlaceholderEntries)
	}
}

func TestCoverageStatus_IsComplete(t *testing.T) {
	tests := []struct {
		status CoverageStatus
		want   bool
	}{
		{CoverageImplemented, true},
		{CoveragePartial, false},
		{CoveragePlaceholder, false},
		{CoverageMissing, false},
		{CoverageThemeDependent, false},
		{CoverageLayoutDependent, false},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			if got := tt.status.IsComplete(); got != tt.want {
				t.Errorf("IsComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCoverageStatus_NeedsAttention(t *testing.T) {
	tests := []struct {
		status CoverageStatus
		want   bool
	}{
		{CoverageMissing, true},
		{CoveragePlaceholder, true},
		{CoveragePartial, true},
		{CoverageImplemented, false},
		{CoverageThemeDependent, false},
		{CoverageLayoutDependent, false},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			if got := tt.status.NeedsAttention(); got != tt.want {
				t.Errorf("NeedsAttention() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewCoverageSnapshot(t *testing.T) {
	entries := []*CatalogEntry{
		{ID: "mark-1", Coverage: CoverageImplemented},
		{ID: "mark-2", Coverage: CoveragePartial},
		{ID: "mark-3", Coverage: CoveragePlaceholder},
		{ID: "mark-4", Coverage: CoverageMissing},
	}

	snapshot := NewCoverageSnapshot(entries)

	if snapshot.Total != 4 {
		t.Errorf("Total = %d, want 4", snapshot.Total)
	}
	if snapshot.Implemented != 1 {
		t.Errorf("Implemented = %d, want 1", snapshot.Implemented)
	}
	if snapshot.Partial != 1 {
		t.Errorf("Partial = %d, want 1", snapshot.Partial)
	}
	if snapshot.Placeholder != 1 {
		t.Errorf("Placeholder = %d, want 1", snapshot.Placeholder)
	}
	if snapshot.Missing != 1 {
		t.Errorf("Missing = %d, want 1", snapshot.Missing)
	}

	expectedPercent := 25.0 // 1/4 * 100
	if snapshot.Percent != expectedPercent {
		t.Errorf("Percent = %v, want %v", snapshot.Percent, expectedPercent)
	}

	if len(snapshot.Entries) != 4 {
		t.Errorf("Entries map has %d entries, want 4", len(snapshot.Entries))
	}
	if snapshot.Entries["mark-1"] != CoverageImplemented {
		t.Error("Entries[mark-1] should be CoverageImplemented")
	}
}

func TestNewCoverageSnapshot_Empty(t *testing.T) {
	entries := []*CatalogEntry{}

	snapshot := NewCoverageSnapshot(entries)

	if snapshot.Total != 0 {
		t.Errorf("Total = %d, want 0", snapshot.Total)
	}
	if snapshot.Percent != 0 {
		t.Errorf("Percent = %v, want 0", snapshot.Percent)
	}
}

func TestComputeDiff(t *testing.T) {
	before := CoverageSnapshot{
		Entries: map[string]CoverageStatus{
			"mark-1": CoverageImplemented,
			"mark-2": CoveragePlaceholder,
			"mark-3": CoveragePartial,
		},
	}

	after := CoverageSnapshot{
		Entries: map[string]CoverageStatus{
			"mark-1": CoverageImplemented, // unchanged
			"mark-2": CoverageImplemented, // changed
			"mark-4": CoveragePartial,     // new
		},
	}

	diff := ComputeDiff(before, after)

	if len(diff.Changes.NewImplemented) != 1 || !contains(diff.Changes.NewImplemented, "mark-2") {
		t.Errorf("Expected mark-2 in NewImplemented, got: %v", diff.Changes.NewImplemented)
	}
	if len(diff.Changes.NewPartial) != 1 || !contains(diff.Changes.NewPartial, "mark-4") {
		t.Errorf("Expected mark-4 in NewPartial, got: %v", diff.Changes.NewPartial)
	}
	if len(diff.Changes.RemovedEntries) != 1 || !contains(diff.Changes.RemovedEntries, "mark-3") {
		t.Errorf("Expected mark-3 in RemovedEntries, got: %v", diff.Changes.RemovedEntries)
	}
}

func TestComputeDiff_NoChanges(t *testing.T) {
	before := CoverageSnapshot{
		Entries: map[string]CoverageStatus{
			"mark-1": CoverageImplemented,
			"mark-2": CoveragePlaceholder,
		},
	}

	after := CoverageSnapshot{
		Entries: map[string]CoverageStatus{
			"mark-1": CoverageImplemented,
			"mark-2": CoveragePlaceholder,
		},
	}

	diff := ComputeDiff(before, after)

	if len(diff.Changes.NewImplemented) != 0 {
		t.Errorf("NewImplemented should be empty, got: %v", diff.Changes.NewImplemented)
	}
	if len(diff.Changes.NewPartial) != 0 {
		t.Errorf("NewPartial should be empty, got: %v", diff.Changes.NewPartial)
	}
	if len(diff.Changes.NewPlaceholders) != 0 {
		t.Errorf("NewPlaceholders should be empty, got: %v", diff.Changes.NewPlaceholders)
	}
	if len(diff.Changes.NewMissing) != 0 {
		t.Errorf("NewMissing should be empty, got: %v", diff.Changes.NewMissing)
	}
	if len(diff.Changes.RemovedEntries) != 0 {
		t.Errorf("RemovedEntries should be empty, got: %v", diff.Changes.RemovedEntries)
	}
}

func TestGenerateCoverageReport(t *testing.T) {
	requiredIDs := []string{"mark-1", "mark-2", "mark-3"}
	entries := []*CatalogEntry{
		{ID: "mark-1", Coverage: CoverageImplemented, Family: FamilyBasic},
		{ID: "mark-2", Coverage: CoverageThemeDependent, Family: FamilyBasic},
		{ID: "mark-3", Coverage: CoverageLayoutDependent, Family: FamilyChart},
	}

	report := GenerateCoverageReport(entries, requiredIDs)

	if report.Summary.Total != 3 {
		t.Errorf("Summary.Total = %d, want 3", report.Summary.Total)
	}
	if report.Summary.Implemented != 1 {
		t.Errorf("Summary.Implemented = %d, want 1", report.Summary.Implemented)
	}
	if report.Summary.ThemeDependent != 1 {
		t.Errorf("Summary.ThemeDependent = %d, want 1", report.Summary.ThemeDependent)
	}
	if report.Summary.LayoutDependent != 1 {
		t.Errorf("Summary.LayoutDependent = %d, want 1", report.Summary.LayoutDependent)
	}
	if report.Summary.Partial != 0 {
		t.Errorf("Summary.Partial = %d, want 0", report.Summary.Partial)
	}
	if report.Summary.Placeholder != 0 {
		t.Errorf("Summary.Placeholder = %d, want 0", report.Summary.Placeholder)
	}

	if len(report.ByFamily) != 2 {
		t.Errorf("ByFamily has %d families, want 2", len(report.ByFamily))
	}
}

func TestGenerateCoverageReport_WithMissing(t *testing.T) {
	requiredIDs := []string{"mark-1", "mark-2", "mark-3", "mark-4"}
	entries := []*CatalogEntry{
		{ID: "mark-1", Coverage: CoverageImplemented},
	}

	report := GenerateCoverageReport(entries, requiredIDs)

	if !report.DriftDetected {
		t.Error("DriftDetected should be true when entries are missing")
	}
	if len(report.Missing) != 3 {
		t.Errorf("Missing should have 3 entries, got: %v", report.Missing)
	}
}

// contains checks if a string slice contains a specific value.
func contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
