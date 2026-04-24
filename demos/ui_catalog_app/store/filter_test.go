package store

import (
	"testing"

	"codeburg.org/lexbit/ui_catalog/model"
)

func TestDefaultFilterState_CoverageFiltersAreOpen(t *testing.T) {
	state := DefaultFilterState()
	for _, coverage := range []model.CoverageStatus{
		model.CoverageImplemented,
		model.CoveragePartial,
		model.CoveragePlaceholder,
		model.CoverageMissing,
		model.CoverageThemeDependent,
		model.CoverageLayoutDependent,
	} {
		if !state.IsCoverageSelected(coverage) {
			t.Fatalf("default filter should allow %s", coverage.String())
		}
	}
}

func TestToggleCoverageFilter_AllStatuses(t *testing.T) {
	t.Cleanup(func() {
		ResetFilters()
	})

	ResetFilters()
	for _, coverage := range []model.CoverageStatus{
		model.CoverageImplemented,
		model.CoveragePartial,
		model.CoveragePlaceholder,
		model.CoverageMissing,
		model.CoverageThemeDependent,
		model.CoverageLayoutDependent,
	} {
		ToggleCoverageFilter(coverage)
		state := FilterStore.Get()
		if !containsCoverage(state.CoverageFilters, coverage) {
			t.Fatalf("coverage %s was not added", coverage.String())
		}
		ToggleCoverageFilter(coverage)
		state = FilterStore.Get()
		if containsCoverage(state.CoverageFilters, coverage) {
			t.Fatalf("coverage %s was not removed", coverage.String())
		}
	}
}

func containsCoverage(slice []model.CoverageStatus, val model.CoverageStatus) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
