package model

import (
	"testing"
)

func TestAllFamilies_ContainsAllSeven(t *testing.T) {
	families := AllFamilies()
	if len(families) != 7 {
		t.Errorf("AllFamilies() returned %d families, want 7", len(families))
	}

	expected := []Family{
		FamilyBasic,
		FamilyStructure,
		FamilyAnnotation,
		FamilyUIInput,
		FamilyUINav,
		FamilyUINotification,
		FamilyChart,
	}

	for i, fam := range expected {
		if families[i] != fam {
			t.Errorf("AllFamilies()[%d] = %v, want %v", i, families[i], fam)
		}
	}
}

func TestFamily_String(t *testing.T) {
	tests := []struct {
		family Family
		want   string
	}{
		{FamilyBasic, "basic"},
		{FamilyStructure, "structure"},
		{FamilyAnnotation, "annotation"},
		{FamilyUIInput, "uiinput"},
		{FamilyUINav, "uinav"},
		{FamilyUINotification, "uinotification"},
		{FamilyChart, "chart"},
		{Family(255), "Family(255)"},
	}

	for _, tt := range tests {
		got := tt.family.String()
		if got != tt.want {
			t.Errorf("Family(%d).String() = %q, want %q", tt.family, got, tt.want)
		}
	}
}

func TestFamily_DisplayName(t *testing.T) {
	tests := []struct {
		family Family
		want   string
	}{
		{FamilyBasic, "Basic"},
		{FamilyStructure, "Structure"},
		{FamilyAnnotation, "Annotation"},
		{FamilyUIInput, "UI Input"},
		{FamilyUINav, "UI Navigation"},
		{FamilyUINotification, "UI Notification"},
		{FamilyChart, "Chart"},
	}

	for _, tt := range tests {
		got := tt.family.DisplayName()
		if got != tt.want {
			t.Errorf("Family(%d).DisplayName() = %q, want %q", tt.family, got, tt.want)
		}
	}
}

func TestParseFamily(t *testing.T) {
	tests := []struct {
		input string
		want  Family
		ok    bool
	}{
		{"basic", FamilyBasic, true},
		{"BASIC", FamilyBasic, true},
		{"Basic", FamilyBasic, true},
		{"structure", FamilyStructure, true},
		{"annotation", FamilyAnnotation, true},
		{"uiinput", FamilyUIInput, true},
		{"uinav", FamilyUINav, true},
		{"uinotification", FamilyUINotification, true},
		{"chart", FamilyChart, true},
		{"unknown", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		got, ok := ParseFamily(tt.input)
		if ok != tt.ok {
			t.Errorf("ParseFamily(%q) ok = %v, want %v", tt.input, ok, tt.ok)
		}
		if ok && got != tt.want {
			t.Errorf("ParseFamily(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestCoverageStatus_String(t *testing.T) {
	tests := []struct {
		status CoverageStatus
		want   string
	}{
		{CoverageImplemented, "implemented"},
		{CoveragePartial, "partial"},
		{CoveragePlaceholder, "placeholder"},
		{CoverageMissing, "missing"},
		{CoverageThemeDependent, "theme-dependent"},
		{CoverageLayoutDependent, "layout-dependent"},
		{CoverageStatus(255), "CoverageStatus(255)"},
	}

	for _, tt := range tests {
		got := tt.status.String()
		if got != tt.want {
			t.Errorf("CoverageStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestCoverageStatus_DisplayName(t *testing.T) {
	tests := []struct {
		status CoverageStatus
		want   string
	}{
		{CoverageImplemented, "Implemented"},
		{CoveragePartial, "Partial"},
		{CoveragePlaceholder, "Placeholder"},
		{CoverageMissing, "Missing"},
		{CoverageThemeDependent, "Theme Dependent"},
		{CoverageLayoutDependent, "Layout Dependent"},
	}

	for _, tt := range tests {
		got := tt.status.DisplayName()
		if got != tt.want {
			t.Errorf("CoverageStatus(%d).DisplayName() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestCoverageStatus_IsUsable(t *testing.T) {
	tests := []struct {
		status CoverageStatus
		want   bool
	}{
		{CoverageImplemented, true},
		{CoveragePartial, true},
		{CoverageThemeDependent, true},
		{CoverageLayoutDependent, true},
		{CoveragePlaceholder, false},
		{CoverageMissing, false},
	}

	for _, tt := range tests {
		got := tt.status.IsUsable()
		if got != tt.want {
			t.Errorf("CoverageStatus(%d).IsUsable() = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestCatalog_AddEntry(t *testing.T) {
	c := NewCatalog()

	entry := &CatalogEntry{
		ID:          "test.entry",
		DisplayName: "Test Entry",
		Family:      FamilyBasic,
		Coverage:    CoverageImplemented,
	}

	if err := c.AddEntry(entry); err != nil {
		t.Errorf("AddEntry failed: %v", err)
	}

	// Duplicate ID should fail
	if err := c.AddEntry(entry); err == nil {
		t.Error("AddEntry with duplicate ID should fail")
	}

	// Empty ID should fail
	emptyEntry := &CatalogEntry{ID: "   "}
	if err := c.AddEntry(emptyEntry); err == nil {
		t.Error("AddEntry with empty ID should fail")
	}

	// Nil entry should fail
	if err := c.AddEntry(nil); err == nil {
		t.Error("AddEntry with nil entry should fail")
	}
}

func TestCatalog_GetEntry(t *testing.T) {
	c := NewCatalog()
	entry := &CatalogEntry{
		ID:       "test.entry",
		Coverage: CoverageImplemented,
	}
	c.AddEntry(entry)

	got, ok := c.GetEntry("test.entry")
	if !ok {
		t.Error("GetEntry should find existing entry")
	}
	if got.ID != "test.entry" {
		t.Errorf("GetEntry returned wrong entry: %v", got.ID)
	}

	_, ok = c.GetEntry("nonexistent")
	if ok {
		t.Error("GetEntry should not find nonexistent entry")
	}
}

func TestCatalog_AllEntries(t *testing.T) {
	c := NewCatalog()

	entries := c.AllEntries()
	if len(entries) != 0 {
		t.Errorf("Empty catalog should have 0 entries, got %d", len(entries))
	}

	c.AddEntry(&CatalogEntry{ID: "a", Family: FamilyBasic})
	c.AddEntry(&CatalogEntry{ID: "b", Family: FamilyStructure})
	c.AddEntry(&CatalogEntry{ID: "c", Family: FamilyBasic})

	entries = c.AllEntries()
	if len(entries) != 3 {
		t.Errorf("Catalog should have 3 entries, got %d", len(entries))
	}

	// Check order is preserved (insertion order)
	if entries[0].ID != "a" || entries[1].ID != "b" || entries[2].ID != "c" {
		t.Error("AllEntries did not preserve insertion order")
	}
}

func TestCatalog_EntriesByFamily(t *testing.T) {
	c := NewCatalog()
	c.AddEntry(&CatalogEntry{ID: "a", Family: FamilyBasic})
	c.AddEntry(&CatalogEntry{ID: "b", Family: FamilyStructure})
	c.AddEntry(&CatalogEntry{ID: "c", Family: FamilyBasic})
	c.AddEntry(&CatalogEntry{ID: "d", Family: FamilyUIInput})

	basicEntries := c.EntriesByFamily(FamilyBasic)
	if len(basicEntries) != 2 {
		t.Errorf("Expected 2 basic entries, got %d", len(basicEntries))
	}

	structureEntries := c.EntriesByFamily(FamilyStructure)
	if len(structureEntries) != 1 {
		t.Errorf("Expected 1 structure entry, got %d", len(structureEntries))
	}

	chartEntries := c.EntriesByFamily(FamilyChart)
	if len(chartEntries) != 0 {
		t.Errorf("Expected 0 chart entries, got %d", len(chartEntries))
	}
}

func TestCatalog_Filtered(t *testing.T) {
	c := NewCatalog()
	c.AddEntry(&CatalogEntry{ID: "basic.rect", Family: FamilyBasic, Interactive: false, ThemeSensitive: true})
	c.AddEntry(&CatalogEntry{ID: "uiinput.button", Family: FamilyUIInput, Interactive: true, ThemeSensitive: true})
	c.AddEntry(&CatalogEntry{ID: "structure.group", Family: FamilyStructure, Interactive: false, ThemeSensitive: false})
	c.AddEntry(&CatalogEntry{ID: "annotation.label", Family: FamilyAnnotation, Interactive: false, ThemeSensitive: true})

	// No filters - should return all
	all := c.Filtered("", nil, false, false, nil)
	if len(all) != 4 {
		t.Errorf("No filters should return all 4 entries, got %d", len(all))
	}

	// Text query
	filtered := c.Filtered("button", nil, false, false, nil)
	if len(filtered) != 1 || filtered[0].ID != "uiinput.button" {
		t.Errorf("Query 'button' should return 1 entry, got %d", len(filtered))
	}

	// Family filter
	filtered = c.Filtered("", []Family{FamilyBasic}, false, false, nil)
	if len(filtered) != 1 || filtered[0].ID != "basic.rect" {
		t.Errorf("Family filter should return 1 basic entry, got %d", len(filtered))
	}

	// Interactive only
	filtered = c.Filtered("", nil, true, false, nil)
	if len(filtered) != 1 || filtered[0].ID != "uiinput.button" {
		t.Errorf("Interactive filter should return 1 entry, got %d", len(filtered))
	}

	// Theme sensitive only
	filtered = c.Filtered("", nil, false, true, nil)
	if len(filtered) != 3 {
		t.Errorf("Theme sensitive filter should return 3 entries, got %d", len(filtered))
	}
}

func TestCatalog_Count(t *testing.T) {
	c := NewCatalog()
	if c.Count() != 0 {
		t.Errorf("Empty catalog count should be 0, got %d", c.Count())
	}

	c.AddEntry(&CatalogEntry{ID: "a"})
	c.AddEntry(&CatalogEntry{ID: "b"})
	if c.Count() != 2 {
		t.Errorf("Catalog count should be 2, got %d", c.Count())
	}
}

func TestSorted(t *testing.T) {
	entries := []*CatalogEntry{
		{ID: "z", DisplayName: "Zebra", Family: FamilyAnnotation},
		{ID: "a", DisplayName: "Alpha", Family: FamilyBasic},
		{ID: "m", DisplayName: "Mike", Family: FamilyUIInput},
	}

	// Sort by ID
	sorted := Sorted(entries, SortByID)
	if sorted[0].ID != "a" || sorted[1].ID != "m" || sorted[2].ID != "z" {
		t.Error("SortByID did not sort correctly")
	}

	// Sort by display name
	sorted = Sorted(entries, SortByDisplayName)
	if sorted[0].DisplayName != "Alpha" || sorted[1].DisplayName != "Mike" || sorted[2].DisplayName != "Zebra" {
		t.Error("SortByDisplayName did not sort correctly")
	}

	// Sort by family then ID
	sorted = Sorted(entries, SortByFamilyThenID)
	if sorted[0].Family != FamilyBasic || sorted[1].Family != FamilyAnnotation || sorted[2].Family != FamilyUIInput {
		t.Error("SortByFamilyThenID did not sort correctly")
	}
}
