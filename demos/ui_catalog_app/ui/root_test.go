package ui

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
	"codeburg.org/lexbit/ui_catalog/store"
)

func TestCatalogRootFacet_Creation(t *testing.T) {
	th := theme.Default()
	shaper := text.NewShaper(nil)
	shaper.SetContentScale(1.0)
	meta := model.DefaultBuildMetadata()

	root := NewCatalogRootFacet(th, shaper, meta)
	if root == nil {
		t.Fatal("NewCatalogRootFacet returned nil")
	}

	if root.HeaderFacet() == nil {
		t.Error("HeaderFacet is nil")
	}
	if root.SidebarFacet() == nil {
		t.Error("SidebarFacet is nil")
	}
	if root.ContentFacet() == nil {
		t.Error("ContentFacet is nil")
	}
	if root.InspectorFacet() == nil {
		t.Error("InspectorFacet is nil")
	}
	if root.FooterFacet() == nil {
		t.Error("FooterFacet is nil")
	}
}

func TestCatalogRootFacet_Base(t *testing.T) {
	th := theme.Default()
	shaper := text.NewShaper(nil)
	shaper.SetContentScale(1.0)
	meta := model.DefaultBuildMetadata()

	root := NewCatalogRootFacet(th, shaper, meta)
	base := root.Base()
	if base == nil {
		t.Fatal("Base() returned nil")
	}
	if base.Impl() != root {
		t.Error("Base().Impl() != root")
	}
}

func TestCatalogRootFacet_Lifecycle(t *testing.T) {
	th := theme.Default()
	shaper := text.NewShaper(nil)
	shaper.SetContentScale(1.0)
	meta := model.DefaultBuildMetadata()

	root := NewCatalogRootFacet(th, shaper, meta)

	// Test attach
	facet.Attach(root, facet.AttachContext{Theme: th})
	defer facet.Dispose(root)

	// After attach, state should be attached
	if root.State() != facet.StateAttached {
		t.Errorf("State() = %v, want StateAttached", root.State())
	}
}

func TestCatalogRootFacet_Measure(t *testing.T) {
	th := theme.Default()
	shaper := text.NewShaper(nil)
	shaper.SetContentScale(1.0)
	meta := model.DefaultBuildMetadata()

	root := NewCatalogRootFacet(th, shaper, meta)

	// Test measure with valid constraints
	cons := facet.Constraints{
		MinSize: gfx.Size{},
		MaxSize: gfx.Size{W: 1000, H: 600},
	}
	size := root.layout.OnMeasure(cons)
	if size.W != 1000 || size.H != 600 {
		t.Errorf("OnMeasure = %v, want {1000, 600}", size)
	}
}

func TestCatalogRootFacet_Arrange(t *testing.T) {
	th := theme.Default()
	shaper := text.NewShaper(nil)
	shaper.SetContentScale(1.0)
	meta := model.DefaultBuildMetadata()

	root := NewCatalogRootFacet(th, shaper, meta)
	facet.Attach(root, facet.AttachContext{Theme: th})
	defer facet.Dispose(root)

	// Test arrange
	bounds := gfx.RectFromXYWH(0, 0, 1000, 600)
	root.layout.Arrange(bounds)

	// Verify shell bounds were computed
	profile := DefaultLayoutProfile()
	shell := CalculateShellBoundsWithProfile(bounds, profile.SidebarWidthDefault, profile.InspectorWidthDefault, profile)
	if shell.Header.IsEmpty() {
		t.Error("Header bounds is empty")
	}
	if shell.Footer.IsEmpty() {
		t.Error("Footer bounds is empty")
	}
	if shell.Sidebar.IsEmpty() {
		t.Error("Sidebar bounds is empty")
	}
	if shell.Content.IsEmpty() {
		t.Error("Content bounds is empty")
	}
	if shell.Inspector.IsEmpty() {
		t.Error("Inspector bounds is empty")
	}
}

func TestCalculateShellBounds(t *testing.T) {
	bounds := gfx.RectFromXYWH(0, 0, 1000, 600)
	profile := LayoutProfileForDensity(store.DensityNormal)
	shell := CalculateShellBoundsWithProfile(bounds, 200, 250, profile)

	// Header should be at top
	if shell.Header.Min.Y != 0 {
		t.Errorf("Header.Min.Y = %v, want 0", shell.Header.Min.Y)
	}
	if shell.Header.Height() != profile.HeaderHeight {
		t.Errorf("Header.Height = %v, want %v", shell.Header.Height(), profile.HeaderHeight)
	}

	// Footer should be at bottom
	if shell.Footer.Max.Y != 600 {
		t.Errorf("Footer.Max.Y = %v, want 600", shell.Footer.Max.Y)
	}
	if shell.Footer.Height() != profile.FooterHeight {
		t.Errorf("Footer.Height = %v, want %v", shell.Footer.Height(), profile.FooterHeight)
	}

	// Sidebar should be on left with requested width
	if shell.Sidebar.Width() != 200 {
		t.Errorf("Sidebar.Width = %v, want 200", shell.Sidebar.Width())
	}

	// Inspector should be on right with requested width
	if shell.Inspector.Width() != 250 {
		t.Errorf("Inspector.Width = %v, want 250", shell.Inspector.Width())
	}

	// Inspector should end at window right
	if shell.Inspector.Max.X != 1000 {
		t.Errorf("Inspector.Max.X = %v, want 1000", shell.Inspector.Max.X)
	}
}

func TestHeaderFacet_WithMetadata(t *testing.T) {
	th := theme.Default()
	shaper := text.NewShaper(nil)
	shaper.SetContentScale(1.0)

	meta := model.BuildMetadata{
		Version:     "1.0.0",
		Commit:      "abc123",
		BuildTime:   "2024-01-01",
		GoVersion:   "go1.23",
		Backend:     "software",
		ThemeEngine: "legacy",
	}

	h := NewHeaderFacet(th, shaper, meta)
	if h == nil {
		t.Fatal("NewHeaderFacet returned nil")
	}

	if h.meta.Version != "1.0.0" {
		t.Errorf("meta.Version = %v, want 1.0.0", h.meta.Version)
	}
}

func TestInset(t *testing.T) {
	bounds := gfx.RectFromXYWH(0, 0, 100, 100)

	// Normal inset
	inset := Inset(bounds, 10)
	if inset.Min.X != 10 || inset.Min.Y != 10 {
		t.Errorf("Inset.Min = %v, want {10, 10}", inset.Min)
	}
	if inset.Max.X != 90 || inset.Max.Y != 90 {
		t.Errorf("Inset.Max = %v, want {90, 90}", inset.Max)
	}

	// Large inset should return empty
	largeInset := Inset(bounds, 60)
	if !largeInset.IsEmpty() {
		t.Error("Large inset should return empty rect")
	}

	// Empty bounds should return empty
	empty := gfx.Rect{}
	result := Inset(empty, 10)
	if !result.IsEmpty() {
		t.Error("Inset of empty rect should return empty")
	}
}
