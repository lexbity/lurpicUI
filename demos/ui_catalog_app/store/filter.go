package store

import (
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/ui_catalog/model"
)

// FilterState holds the current filter and sort configuration.
type FilterState struct {
	Query              string
	SelectedFamilies   []model.Family
	InteractiveOnly    bool
	ThemeSensitiveOnly bool
	CoverageFilters    []model.CoverageStatus
	SortMode           model.SortMode
	ShowVariants       bool
	ShowStates         bool
}

// DefaultFilterState returns the initial filter configuration.
func DefaultFilterState() FilterState {
	return FilterState{
		Query:              "",
		SelectedFamilies:   nil, // nil means all families
		InteractiveOnly:    false,
		ThemeSensitiveOnly: false,
		CoverageFilters:    nil, // nil means all coverage statuses
		SortMode:           model.SortByFamilyThenID,
		ShowVariants:       true,
		ShowStates:         true,
	}
}

// Clone creates a deep copy of the filter state.
func (f FilterState) Clone() FilterState {
	out := f
	if f.SelectedFamilies != nil {
		out.SelectedFamilies = make([]model.Family, len(f.SelectedFamilies))
		copy(out.SelectedFamilies, f.SelectedFamilies)
	}
	if f.CoverageFilters != nil {
		out.CoverageFilters = make([]model.CoverageStatus, len(f.CoverageFilters))
		copy(out.CoverageFilters, f.CoverageFilters)
	}
	return out
}

// IsFamilySelected reports whether a family passes the filter.
func (f FilterState) IsFamilySelected(fam model.Family) bool {
	if len(f.SelectedFamilies) == 0 {
		return true
	}
	for _, sf := range f.SelectedFamilies {
		if sf == fam {
			return true
		}
	}
	return false
}

// IsCoverageSelected reports whether a coverage status passes the filter.
func (f FilterState) IsCoverageSelected(coverage model.CoverageStatus) bool {
	if len(f.CoverageFilters) == 0 {
		return true
	}
	for _, c := range f.CoverageFilters {
		if c == coverage {
			return true
		}
	}
	return false
}

// FilterStore is the reactive store for filter state.
var FilterStore = store.NewValueStore[FilterState](DefaultFilterState())

// SetFilterQuery updates the text query filter.
func SetFilterQuery(query string) {
	current := FilterStore.Get()
	current.Query = query
	FilterStore.Set(current)
}

// ToggleFamily adds or removes a family from the selection.
func ToggleFamily(fam model.Family) {
	current := FilterStore.Get()
	found := -1
	for i, f := range current.SelectedFamilies {
		if f == fam {
			found = i
			break
		}
	}
	if found >= 0 {
		// Remove
		current.SelectedFamilies = append(current.SelectedFamilies[:found], current.SelectedFamilies[found+1:]...)
	} else {
		// Add
		current.SelectedFamilies = append(current.SelectedFamilies, fam)
	}
	FilterStore.Set(current)
}

// SetInteractiveOnly updates the interactive-only toggle.
func SetInteractiveOnly(v bool) {
	current := FilterStore.Get()
	current.InteractiveOnly = v
	FilterStore.Set(current)
}

// SetThemeSensitiveOnly updates the theme-sensitive-only toggle.
func SetThemeSensitiveOnly(v bool) {
	current := FilterStore.Get()
	current.ThemeSensitiveOnly = v
	FilterStore.Set(current)
}

// SetSortMode updates the sort mode.
func SetSortMode(mode model.SortMode) {
	current := FilterStore.Get()
	current.SortMode = mode
	FilterStore.Set(current)
}

// ResetFilters restores default filter state.
func ResetFilters() {
	FilterStore.Set(DefaultFilterState())
}

// SelectSingleFamily sets only one family as selected.
func SelectSingleFamily(fam model.Family) {
	current := FilterStore.Get()
	current.SelectedFamilies = []model.Family{fam}
	FilterStore.Set(current)
}

// SelectAllFamilies clears family filtering (show all).
func SelectAllFamilies() {
	current := FilterStore.Get()
	current.SelectedFamilies = nil
	FilterStore.Set(current)
}

// ToggleCoverageFilter adds or removes a coverage status from the filter.
func ToggleCoverageFilter(coverage model.CoverageStatus) {
	current := FilterStore.Get()
	found := -1
	for i, c := range current.CoverageFilters {
		if c == coverage {
			found = i
			break
		}
	}
	if found >= 0 {
		// Remove
		current.CoverageFilters = append(current.CoverageFilters[:found], current.CoverageFilters[found+1:]...)
	} else {
		// Add
		current.CoverageFilters = append(current.CoverageFilters, coverage)
	}
	FilterStore.Set(current)
}

// SelectAllCoverages clears coverage filtering (show all).
func SelectAllCoverages() {
	current := FilterStore.Get()
	current.CoverageFilters = nil
	FilterStore.Set(current)
}
