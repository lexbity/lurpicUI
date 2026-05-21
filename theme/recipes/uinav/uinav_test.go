package uinav

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/theme"
)

func TestResolveMenuRecipe_reports_dense_variant(t *testing.T) {
	slots, report := ResolveMenuRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, MenuDense)
	if report.Variant != theme.VariantKey("dense") {
		t.Fatalf("variant = %q, want dense", report.Variant)
	}
	if source, ok := report.SlotSource("Surface"); !ok || source != theme.SlotSourceVariantDefault {
		t.Fatalf("surface source = %v, %v", source, ok)
	}
	if slots.Surface.Base.Fills == nil {
		t.Fatal("expected dense menu surface styling")
	}
}

func TestResolveBreadcrumbRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveBreadcrumbRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("standard") {
		t.Fatalf("breadcrumb variant = %q, want standard", report.Variant)
	}
	for _, name := range []string{"Root", "SegmentList", "SegmentLink", "Separator", "CurrentSegment", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected breadcrumb slot source for %s", name)
		}
	}
	if slots.Root.Base.Fills == nil || slots.SegmentLink.Base.Fills == nil || slots.CurrentSegment.Base.Fills == nil {
		t.Fatal("expected breadcrumb slots to be populated")
	}
}

func TestResolveNavDrawerRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveNavDrawerRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("standard") {
		t.Fatalf("nav drawer variant = %q, want standard", report.Variant)
	}
	for _, name := range []string{"Root", "ScrimOptional", "DrawerSurface", "Header", "NavItems", "SectionLabels", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected nav drawer slot source for %s", name)
		}
	}
	if slots.DrawerSurface.Base.Fills == nil || slots.NavItems.Base.Fills == nil || slots.Header.Base.Fills == nil {
		t.Fatal("expected nav drawer slots to be populated")
	}
}

func TestResolveNavRailRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveNavRailRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("standard") {
		t.Fatalf("nav rail variant = %q, want standard", report.Variant)
	}
	for _, name := range []string{"Root", "RailSurface", "NavItems", "ActiveIndicator", "Icon", "Label", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected nav rail slot source for %s", name)
		}
	}
	if slots.RailSurface.Base.Fills == nil || slots.Icon.Base.Fills == nil || slots.Label.Base.Fills == nil {
		t.Fatal("expected nav rail slots to be populated")
	}
}

func TestResolveTreeNavigatorRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveTreeNavigatorRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("standard") {
		t.Fatalf("tree navigator variant = %q, want standard", report.Variant)
	}
	for _, name := range []string{"Root", "Tree", "TreeItem", "Disclosure", "Icon", "Label", "SelectionIndicator", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected tree navigator slot source for %s", name)
		}
	}
	if slots.Tree.Base.Fills == nil || slots.Label.Base.Fills == nil || slots.SelectionIndicator.Base.Fills == nil {
		t.Fatal("expected tree navigator slots to be populated")
	}
}

func TestResolvePaginationRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolvePaginationRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("standard") {
		t.Fatalf("pagination variant = %q, want standard", report.Variant)
	}
	for _, name := range []string{"Root", "Page", "Current", "Nav", "Separator", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected pagination slot source for %s", name)
		}
	}
	if slots.Page.Base.Fills == nil || slots.Current.Base.Fills == nil || slots.Nav.Base.Fills == nil {
		t.Fatal("expected pagination slots to be populated")
	}
}

func TestResolveDrawerAndSpeedDialRecipes_expose_expected_slots(t *testing.T) {
	drawer, drawerReport := ResolveDrawerRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if drawerReport.Variant != theme.VariantKey("standard") {
		t.Fatalf("drawer variant = %q, want standard", drawerReport.Variant)
	}
	if _, ok := drawerReport.SlotSource("Surface"); !ok {
		t.Fatal("expected drawer surface slot source")
	}
	if drawer.Surface.Base.Fills == nil || drawer.Scrim.Base.Fills == nil {
		t.Fatal("expected drawer slots to be populated")
	}

	speedDial, speedDialReport := ResolveSpeedDialRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if speedDialReport.Variant != theme.VariantKey("standard") {
		t.Fatalf("speed dial variant = %q, want standard", speedDialReport.Variant)
	}
	if _, ok := speedDialReport.SlotSource("Fab"); !ok {
		t.Fatal("expected speed dial fab slot source")
	}
	if speedDial.Fab.Base.Fills == nil || speedDial.Backdrop.Base.Fills == nil {
		t.Fatal("expected speed dial slots to be populated")
	}
}
