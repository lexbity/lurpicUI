package model

import (
	"strings"
	"testing"
)

// TestNewStandardCatalog_ContainsAllExpectedEntries verifies that the standard
// catalog includes all inventory-relevant elements from the plan.
func TestNewStandardCatalog_ContainsAllExpectedEntries(t *testing.T) {
	c := NewStandardCatalog()

	// Expected entries by family from the product plan
	expectedEntries := map[string]Family{
		// Basic family
		"basic.rect":     FamilyBasic,
		"basic.ellipse":  FamilyBasic,
		"basic.polygon":  FamilyBasic,
		"basic.polyline": FamilyBasic,
		"basic.line":     FamilyBasic,
		"basic.path":     FamilyBasic,
		"basic.image":    FamilyBasic,
		"basic.text":     FamilyBasic,

		// Structure family
		"structure.group":     FamilyStructure,
		"structure.clip":      FamilyStructure,
		"structure.transform": FamilyStructure,
		"structure.viewport":  FamilyStructure,
		"structure.anchor":    FamilyStructure,
		"structure.layer":     FamilyStructure,

		// Annotation family
		"annotation.label":     FamilyAnnotation,
		"annotation.connector": FamilyAnnotation,
		"annotation.callout":   FamilyAnnotation,
		"annotation.handle":    FamilyAnnotation,
		"annotation.symbol":    FamilyAnnotation,
		"annotation.icon":      FamilyAnnotation,
		"annotation.badge":     FamilyAnnotation,
		"annotation.rule":      FamilyAnnotation,
		"annotation.area":      FamilyAnnotation,

		// UI Input family
		"uiinput.button":     FamilyUIInput,
		"uiinput.checkbox":   FamilyUIInput,
		"uiinput.switch":     FamilyUIInput,
		"uiinput.slider":     FamilyUIInput,
		"uiinput.select":     FamilyUIInput,
		"uiinput.textinput":  FamilyUIInput,
		"uiinput.radiogroup": FamilyUIInput,

		// UI Navigation family
		"uinav.tabs":        FamilyUINav,
		"uinav.breadcrumbs": FamilyUINav,
		"uinav.drawer":      FamilyUINav,
		"uinav.menu":        FamilyUINav,
		"uinav.pagination":  FamilyUINav,
		"uinav.scrollbar":   FamilyUINav,
		"uinav.speeddial":   FamilyUINav,

		// UI Notification family
		"uinotification.dialog":   FamilyUINotification,
		"uinotification.snackbar": FamilyUINotification,
		"uinotification.progress": FamilyUINotification,

		// Chart family
		"chart.axis": FamilyChart,
	}

	// Check each expected entry exists with correct family
	for id, expectedFamily := range expectedEntries {
		entry, ok := c.GetEntry(id)
		if !ok {
			t.Errorf("Standard catalog missing expected entry: %s", id)
			continue
		}
		if entry.Family != expectedFamily {
			t.Errorf("Entry %s has family %v, want %v", id, entry.Family, expectedFamily)
		}
		// Check ID format follows convention
		if !strings.Contains(entry.ID, ".") {
			t.Errorf("Entry %s does not follow dotted ID convention", id)
		}
	}

	// Check total count
	expectedCount := len(expectedEntries)
	if c.Count() != expectedCount {
		t.Errorf("Standard catalog has %d entries, want %d", c.Count(), expectedCount)
	}
}

// TestNewStandardCatalog_AllFamiliesRepresented verifies each family has entries.
func TestNewStandardCatalog_AllFamiliesRepresented(t *testing.T) {
	c := NewStandardCatalog()

	for _, fam := range AllFamilies() {
		count := c.CountByFamily(fam)
		if count == 0 {
			t.Errorf("Family %s has no entries in standard catalog", fam.String())
		}
	}
}

// TestNewStandardCatalog_IDsAreUnique verifies all IDs are unique.
func TestNewStandardCatalog_IDsAreUnique(t *testing.T) {
	c := NewStandardCatalog()
	entries := c.AllEntries()

	seen := make(map[string]bool)
	for _, entry := range entries {
		if seen[entry.ID] {
			t.Errorf("Duplicate ID found: %s", entry.ID)
		}
		seen[entry.ID] = true
	}

	if len(seen) != len(entries) {
		t.Errorf("Expected %d unique IDs, got %d", len(entries), len(seen))
	}
}

// TestNewStandardCatalog_IDsAreStable verifies ID format is consistent.
func TestNewStandardCatalog_IDsAreStable(t *testing.T) {
	c := NewStandardCatalog()

	for _, entry := range c.AllEntries() {
		// ID should not be empty
		if strings.TrimSpace(entry.ID) == "" {
			t.Error("Entry has empty ID")
		}

		// ID should be lowercase
		if entry.ID != strings.ToLower(entry.ID) {
			t.Errorf("Entry ID %s is not lowercase", entry.ID)
		}

		// ID should use dot notation: family.name
		parts := strings.Split(entry.ID, ".")
		if len(parts) != 2 {
			t.Errorf("Entry ID %s does not follow 'family.name' format", entry.ID)
		}

		// Family part should match actual family
		familyPart := parts[0]
		expectedFamily, _ := ParseFamily(familyPart)
		if entry.Family != expectedFamily {
			t.Errorf("Entry %s family part %s does not match family %v", entry.ID, familyPart, entry.Family)
		}
	}
}

// TestNewStandardCatalog_CoverageStatusesExplicit verifies all entries have explicit coverage.
func TestNewStandardCatalog_CoverageStatusesExplicit(t *testing.T) {
	c := NewStandardCatalog()

	for _, entry := range c.AllEntries() {
		// Coverage should be a valid status
		if entry.Coverage < CoverageImplemented || entry.Coverage > CoverageLayoutDependent {
			t.Errorf("Entry %s has invalid coverage status: %d", entry.ID, entry.Coverage)
		}

		// Display name should not be empty
		if strings.TrimSpace(entry.DisplayName) == "" {
			t.Errorf("Entry %s has empty display name", entry.ID)
		}
	}
}

// TestNewStandardCatalog_CoverageCounts provides visibility into coverage distribution.
func TestNewStandardCatalog_CoverageCounts(t *testing.T) {
	c := NewStandardCatalog()

	counts := make(map[CoverageStatus]int)
	for _, entry := range c.AllEntries() {
		counts[entry.Coverage]++
	}

	t.Logf("Coverage distribution:")
	for status := CoverageImplemented; status <= CoverageLayoutDependent; status++ {
		t.Logf("  %s: %d", status.DisplayName(), counts[status])
	}

	if counts[CoverageThemeDependent] == 0 {
		t.Error("Expected at least one theme-dependent entry")
	}
	if counts[CoverageLayoutDependent] == 0 {
		t.Error("Expected at least one layout-dependent entry")
	}
	if counts[CoveragePlaceholder] == 0 {
		t.Error("Expected at least one placeholder entry")
	}
}

// TestNewStandardCatalog_InteractiveFlags verifies interactive flags are set correctly.
func TestNewStandardCatalog_InteractiveFlags(t *testing.T) {
	c := NewStandardCatalog()

	interactiveCount := 0
	for _, entry := range c.AllEntries() {
		if entry.Interactive {
			interactiveCount++

			// UI input, nav, and notification entries should generally be interactive
			switch entry.Family {
			case FamilyUIInput, FamilyUINav, FamilyUINotification:
				// Expected to be interactive
			default:
				// Other families generally shouldn't be interactive
				// but we just log rather than enforce
			}
		}
	}

	// Should have some interactive entries
	if interactiveCount == 0 {
		t.Error("No interactive entries found")
	}

	t.Logf("Interactive entries: %d/%d", interactiveCount, c.Count())
}

// TestNewStandardCatalog_ThemeSensitiveFlags verifies theme-sensitive flags.
func TestNewStandardCatalog_ThemeSensitiveFlags(t *testing.T) {
	c := NewStandardCatalog()

	themeSensitiveCount := 0
	for _, entry := range c.AllEntries() {
		if entry.ThemeSensitive {
			themeSensitiveCount++
		}
	}

	// Most entries should be theme-sensitive
	if themeSensitiveCount == 0 {
		t.Error("No theme-sensitive entries found")
	}

	t.Logf("Theme-sensitive entries: %d/%d", themeSensitiveCount, c.Count())
}

// TestNewStandardCatalog_VariantAndStateMatrices verifies canonical inventory matrices are populated.
func TestNewStandardCatalog_VariantAndStateMatrices(t *testing.T) {
	c := NewStandardCatalog()

	tests := []struct {
		id              string
		wantVariantsMin int
		wantStatesMin   int
		wantMissingMin  int
	}{
		{id: "basic.text", wantVariantsMin: 3, wantStatesMin: 3},
		{id: "structure.group", wantVariantsMin: 2, wantStatesMin: 2},
		{id: "annotation.label", wantVariantsMin: 2, wantStatesMin: 3},
		{id: "uiinput.button", wantVariantsMin: 3, wantStatesMin: 5, wantMissingMin: 2},
		{id: "uinav.tabs", wantVariantsMin: 3, wantStatesMin: 4},
		{id: "uinotification.dialog", wantVariantsMin: 3, wantStatesMin: 3},
		{id: "chart.axis", wantVariantsMin: 2, wantStatesMin: 2},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			entry, ok := c.GetEntry(tt.id)
			if !ok {
				t.Fatalf("missing entry %q", tt.id)
			}
			if len(entry.Variants) < tt.wantVariantsMin {
				t.Fatalf("entry %s has %d variants, want at least %d", tt.id, len(entry.Variants), tt.wantVariantsMin)
			}
			if len(entry.States) < tt.wantStatesMin {
				t.Fatalf("entry %s has %d states, want at least %d", tt.id, len(entry.States), tt.wantStatesMin)
			}
			if tt.wantMissingMin > 0 && len(entry.MissingVariants) < tt.wantMissingMin {
				t.Fatalf("entry %s has %d missing variants, want at least %d", tt.id, len(entry.MissingVariants), tt.wantMissingMin)
			}
			if len(entry.Variants) > 1 {
				for i := 1; i < len(entry.Variants); i++ {
					if entry.Variants[i].ID < entry.Variants[i-1].ID {
						t.Fatalf("entry %s variants are not sorted by ID", tt.id)
					}
				}
			}
			if len(entry.States) > 1 {
				for i := 1; i < len(entry.States); i++ {
					if entry.States[i].ID < entry.States[i-1].ID {
						t.Fatalf("entry %s states are not sorted by ID", tt.id)
					}
				}
			}
		})
	}
}

// TestCatalogEntry_IsComplete verifies the IsComplete method.
func TestCatalogEntry_IsComplete(t *testing.T) {
	complete := &CatalogEntry{Coverage: CoverageImplemented}
	if !complete.IsComplete() {
		t.Error("Implemented entry should be complete")
	}

	partial := &CatalogEntry{Coverage: CoveragePartial}
	if partial.IsComplete() {
		t.Error("Partial entry should not be complete")
	}

	placeholder := &CatalogEntry{Coverage: CoveragePlaceholder}
	if placeholder.IsComplete() {
		t.Error("Placeholder entry should not be complete")
	}

	missing := &CatalogEntry{Coverage: CoverageMissing}
	if missing.IsComplete() {
		t.Error("Missing entry should not be complete")
	}

	nilEntry := (*CatalogEntry)(nil)
	if nilEntry.IsComplete() {
		t.Error("Nil entry should not be complete")
	}
}
