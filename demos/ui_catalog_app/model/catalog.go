package model

import (
	"fmt"
	"sort"
	"strings"
)

// Family identifies a mark family for catalog organization.
type Family uint8

const (
	FamilyBasic Family = iota
	FamilyStructure
	FamilyAnnotation
	FamilyUIInput
	FamilyUINav
	FamilyUINotification
	FamilyChart
)

// String returns the canonical family name.
func (f Family) String() string {
	switch f {
	case FamilyBasic:
		return "basic"
	case FamilyStructure:
		return "structure"
	case FamilyAnnotation:
		return "annotation"
	case FamilyUIInput:
		return "uiinput"
	case FamilyUINav:
		return "uinav"
	case FamilyUINotification:
		return "uinotification"
	case FamilyChart:
		return "chart"
	default:
		return fmt.Sprintf("Family(%d)", uint8(f))
	}
}

// DisplayName returns a human-readable family name.
func (f Family) DisplayName() string {
	switch f {
	case FamilyBasic:
		return "Basic"
	case FamilyStructure:
		return "Structure"
	case FamilyAnnotation:
		return "Annotation"
	case FamilyUIInput:
		return "UI Input"
	case FamilyUINav:
		return "UI Navigation"
	case FamilyUINotification:
		return "UI Notification"
	case FamilyChart:
		return "Chart"
	default:
		return f.String()
	}
}

// AllFamilies returns all families in display order.
func AllFamilies() []Family {
	return []Family{
		FamilyBasic,
		FamilyStructure,
		FamilyAnnotation,
		FamilyUIInput,
		FamilyUINav,
		FamilyUINotification,
		FamilyChart,
	}
}

// ParseFamily parses a canonical family name.
func ParseFamily(s string) (Family, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "basic":
		return FamilyBasic, true
	case "structure":
		return FamilyStructure, true
	case "annotation":
		return FamilyAnnotation, true
	case "uiinput":
		return FamilyUIInput, true
	case "uinav":
		return FamilyUINav, true
	case "uinotification":
		return FamilyUINotification, true
	case "chart":
		return FamilyChart, true
	default:
		return 0, false
	}
}

// CoverageStatus indicates the implementation state of a catalog entry.
type CoverageStatus uint8

const (
	CoverageImplemented CoverageStatus = iota
	CoveragePartial
	CoveragePlaceholder
	CoverageMissing
	CoverageThemeDependent
	CoverageLayoutDependent
)

// String returns the canonical coverage status name.
func (c CoverageStatus) String() string {
	switch c {
	case CoverageImplemented:
		return "implemented"
	case CoveragePartial:
		return "partial"
	case CoveragePlaceholder:
		return "placeholder"
	case CoverageMissing:
		return "missing"
	case CoverageThemeDependent:
		return "theme-dependent"
	case CoverageLayoutDependent:
		return "layout-dependent"
	default:
		return fmt.Sprintf("CoverageStatus(%d)", uint8(c))
	}
}

// DisplayName returns a human-readable coverage status.
func (c CoverageStatus) DisplayName() string {
	switch c {
	case CoverageImplemented:
		return "Implemented"
	case CoveragePartial:
		return "Partial"
	case CoveragePlaceholder:
		return "Placeholder"
	case CoverageMissing:
		return "Missing"
	case CoverageThemeDependent:
		return "Theme Dependent"
	case CoverageLayoutDependent:
		return "Layout Dependent"
	default:
		return c.String()
	}
}

// IsUsable returns true if the entry can be previewed/rendered.
func (c CoverageStatus) IsUsable() bool {
	switch c {
	case CoverageImplemented, CoveragePartial, CoverageThemeDependent, CoverageLayoutDependent:
		return true
	default:
		return false
	}
}

// IsComplete returns true if the coverage status indicates a complete implementation.
func (c CoverageStatus) IsComplete() bool {
	return c == CoverageImplemented
}

// NeedsAttention returns true if the coverage status requires developer attention.
func (c CoverageStatus) NeedsAttention() bool {
	return c == CoverageMissing || c == CoveragePlaceholder || c == CoveragePartial
}

// ConstructionClass identifies how a mark is authored and built.
type ConstructionClass uint8

const (
	ConstructionPrimitive ConstructionClass = iota
	ConstructionComposed
	ConstructionGenerated
)

// String returns the canonical construction class name.
func (c ConstructionClass) String() string {
	switch c {
	case ConstructionPrimitive:
		return "primitive"
	case ConstructionComposed:
		return "composed"
	case ConstructionGenerated:
		return "generated"
	default:
		return fmt.Sprintf("ConstructionClass(%d)", uint8(c))
	}
}

// Variant describes a supported variant of a mark.
type Variant struct {
	ID            string
	Label         string
	SizeClass     string
	StateClass    string
	ThemeClass    string
	ScreenshotKey string
}

// State describes a supported state of a mark.
type State struct {
	ID            string
	Label         string
	ScreenshotKey string
}

// CatalogEntry describes one mark type in the catalog.
type CatalogEntry struct {
	ID                string
	DisplayName       string
	Family            Family
	Subcategory       string
	ConstructionClass ConstructionClass
	Interactive       bool
	ThemeSensitive    bool
	LayoutSensitive   bool
	Coverage          CoverageStatus
	Notes             string
	Variants          []Variant
	States            []State
}

// IsComplete returns true if the entry is fully implemented.
func (e *CatalogEntry) IsComplete() bool {
	if e == nil {
		return false
	}
	return e.Coverage == CoverageImplemented
}

// Catalog provides access to all inventory entries.
type Catalog struct {
	entries map[string]*CatalogEntry
	order   []string
}

// NewCatalog creates an empty catalog.
func NewCatalog() *Catalog {
	return &Catalog{
		entries: make(map[string]*CatalogEntry),
		order:   nil,
	}
}

// AddEntry adds an entry to the catalog.
func (c *Catalog) AddEntry(e *CatalogEntry) error {
	if c == nil {
		return fmt.Errorf("catalog is nil")
	}
	if e == nil {
		return fmt.Errorf("entry is nil")
	}
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("entry ID is required")
	}
	if _, exists := c.entries[e.ID]; exists {
		return fmt.Errorf("entry %q already exists", e.ID)
	}
	c.entries[e.ID] = e
	c.order = append(c.order, e.ID)
	return nil
}

// GetEntry returns an entry by ID.
func (c *Catalog) GetEntry(id string) (*CatalogEntry, bool) {
	if c == nil {
		return nil, false
	}
	e, ok := c.entries[id]
	return e, ok
}

// AllEntries returns all entries in stable order.
func (c *Catalog) AllEntries() []*CatalogEntry {
	if c == nil {
		return nil
	}
	out := make([]*CatalogEntry, 0, len(c.order))
	for _, id := range c.order {
		if e, ok := c.entries[id]; ok {
			out = append(out, e)
		}
	}
	return out
}

// EntriesByFamily returns entries filtered by family.
func (c *Catalog) EntriesByFamily(f Family) []*CatalogEntry {
	if c == nil {
		return nil
	}
	var out []*CatalogEntry
	for _, id := range c.order {
		if e, ok := c.entries[id]; ok && e.Family == f {
			out = append(out, e)
		}
	}
	return out
}

// Count returns the total number of entries.
func (c *Catalog) Count() int {
	if c == nil {
		return 0
	}
	return len(c.entries)
}

// CountByFamily returns the count of entries in a family.
func (c *Catalog) CountByFamily(f Family) int {
	return len(c.EntriesByFamily(f))
}

// CountByCoverage returns the count of entries with a given coverage status.
func (c *Catalog) CountByCoverage(status CoverageStatus) int {
	if c == nil {
		return 0
	}
	count := 0
	for _, e := range c.entries {
		if e.Coverage == status {
			count++
		}
	}
	return count
}

// Filtered returns entries matching the filter criteria.
func (c *Catalog) Filtered(query string, families []Family, interactiveOnly, themeSensitiveOnly bool, coverage []CoverageStatus) []*CatalogEntry {
	if c == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))

	var out []*CatalogEntry
	for _, id := range c.order {
		e, ok := c.entries[id]
		if !ok {
			continue
		}

		// Text query filter
		if query != "" {
			if !strings.Contains(strings.ToLower(e.ID), query) &&
				!strings.Contains(strings.ToLower(e.DisplayName), query) {
				continue
			}
		}

		// Family filter
		if len(families) > 0 {
			found := false
			for _, f := range families {
				if e.Family == f {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Interactive only filter
		if interactiveOnly && !e.Interactive {
			continue
		}

		// Theme sensitive only filter
		if themeSensitiveOnly && !e.ThemeSensitive {
			continue
		}

		// Coverage filter
		if len(coverage) > 0 {
			found := false
			for _, c := range coverage {
				if e.Coverage == c {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		out = append(out, e)
	}
	return out
}

// SortMode defines how to sort catalog entries.
type SortMode uint8

const (
	SortByID SortMode = iota
	SortByDisplayName
	SortByFamilyThenID
	SortByCoverage
)

// Sorted returns entries sorted by the given mode.
func Sorted(entries []*CatalogEntry, mode SortMode) []*CatalogEntry {
	out := make([]*CatalogEntry, len(entries))
	copy(out, entries)

	switch mode {
	case SortByID:
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].ID < out[j].ID
		})
	case SortByDisplayName:
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].DisplayName != out[j].DisplayName {
				return out[i].DisplayName < out[j].DisplayName
			}
			return out[i].ID < out[j].ID
		})
	case SortByFamilyThenID:
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Family != out[j].Family {
				return out[i].Family < out[j].Family
			}
			return out[i].ID < out[j].ID
		})
	case SortByCoverage:
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Coverage != out[j].Coverage {
				return out[i].Coverage < out[j].Coverage
			}
			return out[i].ID < out[j].ID
		})
	}

	return out
}
