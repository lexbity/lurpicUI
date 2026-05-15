package store

import (
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/ui_catalog/model"
)

// CatalogInstance holds the global catalog instance.
// This is initialized at startup and remains immutable.
var CatalogInstance = model.NewStandardCatalog()

// FilteredEntries is a derived store that provides filtered and sorted entries.
var FilteredEntries = store.NewDerived(
	computeFilteredEntries,
	FilterStore,
)

func computeFilteredEntries() []*model.CatalogEntry {
	filter := FilterStore.Get()
	entries := CatalogInstance.Filtered(
		filter.Query,
		filter.SelectedFamilies,
		filter.InteractiveOnly,
		filter.ThemeSensitiveOnly,
		filter.CoverageFilters,
	)
	return model.Sorted(entries, filter.SortMode)
}

// Counts holds aggregate counts for the footer.
type Counts struct {
	Total               int
	Filtered            int
	Selected            int
	WithVariants        int
	WithStates          int
	WithMissingVariants int
	WithMissingStates   int
	ByFamily            map[model.Family]int
	ByCoverage          map[model.CoverageStatus]int
}

// GetCounts returns current catalog counts.
func GetCounts() Counts {
	c := Counts{
		Total:      CatalogInstance.Count(),
		Filtered:   len(FilteredEntries.Get()),
		Selected:   0,
		ByFamily:   make(map[model.Family]int),
		ByCoverage: make(map[model.CoverageStatus]int),
	}

	if SelectionStore.Get() != "" {
		c.Selected = 1
	}

	for _, f := range model.AllFamilies() {
		c.ByFamily[f] = CatalogInstance.CountByFamily(f)
	}

	for status := model.CoverageImplemented; status <= model.CoverageLayoutDependent; status++ {
		c.ByCoverage[status] = CatalogInstance.CountByCoverage(status)
	}

	for _, entry := range CatalogInstance.AllEntries() {
		if len(entry.Variants) > 0 {
			c.WithVariants++
		}
		if len(entry.States) > 0 {
			c.WithStates++
		}
		if len(entry.MissingVariants) > 0 {
			c.WithMissingVariants++
		}
		if len(entry.MissingStates) > 0 {
			c.WithMissingStates++
		}
	}

	return c
}
