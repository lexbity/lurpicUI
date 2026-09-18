package runtime

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/theme"
)

// TestRuntimeResolveLayerRecipeRefModalIsOneByOne pins the standard "modal"
// recipe (RX-1 Q4): a single cell that fills the parent so the child centers
// itself within it. It replaces the old hardcoded "modal" check in
// resolveAttachedLayers.
func TestRuntimeResolveLayerRecipeRefModalIsOneByOne(t *testing.T) {
	rt := mustRuntime(t)
	recipe, err := rt.resolveLayerRecipeRef(facet.LayerRecipeRef{Name: "modal"}, gfx.Rect{})
	if err != nil {
		t.Fatalf("resolve modal recipe: %v", err)
	}
	if recipe.PolicyKind != layout.LayerLayoutGrid {
		t.Fatalf("modal PolicyKind = %v, want LayerLayoutGrid", recipe.PolicyKind)
	}
	if recipe.Grid.Columns != 1 || recipe.Grid.Rows != 1 {
		t.Fatalf("modal grid = %dx%d, want 1x1", recipe.Grid.Columns, recipe.Grid.Rows)
	}
}

// TestRuntimeResolveLayerRecipeRefThemeRegistered pins the Family-qualified
// resolution path: an attachment recipe ref that names a theme-registered
// recipe resolves through the theme resolver.
func TestRuntimeResolveLayerRecipeRefThemeRegistered(t *testing.T) {
	rt := mustRuntime(t)
	ref := layout.LayerLayoutRecipeRef{Family: "rt-test", Name: "canvas"}
	resolver := theme.NewThemeResolver()
	if err := resolver.RegisterLayerLayoutRecipe(ref, func(theme.ResolvedContext) layout.ResolvedLayerLayoutRecipe {
		return layout.ResolvedLayerLayoutRecipe{PolicyKind: layout.LayerLayoutFree}
	}); err != nil {
		t.Fatalf("register recipe: %v", err)
	}
	rt.config.ThemeResolver = resolver
	recipe, err := rt.resolveLayerRecipeRef(facet.LayerRecipeRef{Family: "rt-test", Name: "canvas"}, gfx.Rect{})
	if err != nil {
		t.Fatalf("resolve theme recipe: %v", err)
	}
	if recipe.PolicyKind != layout.LayerLayoutFree {
		t.Fatalf("PolicyKind = %v, want LayerLayoutFree", recipe.PolicyKind)
	}
}

// TestRuntimeResolveLayerRecipeRefUnknownFails pins RX-1 §7.2 fail-fast: an
// attachment recipe that resolves to nothing is an error, never a silent
// fallback to the default recipe.
func TestRuntimeResolveLayerRecipeRefUnknownFails(t *testing.T) {
	rt := mustRuntime(t)
	if _, err := rt.resolveLayerRecipeRef(facet.LayerRecipeRef{Name: "bogus"}, gfx.Rect{}); err == nil {
		t.Fatal("unknown standard recipe resolved without error")
	}
	if _, err := rt.resolveLayerRecipeRef(facet.LayerRecipeRef{Family: "no-such-family", Name: "canvas"}, gfx.Rect{}); err == nil {
		t.Fatal("unregistered theme recipe resolved without error")
	}
}

// TestRuntimeNewRejectsUnknownLayerRecipe pins the attach-time guard: an
// unresolvable per-attachment layer recipe fails runtime construction with the
// facet id instead of producing an overlay that silently never renders
// (§7.2 fail-fast; the A-5 class).
func TestRuntimeNewRejectsUnknownLayerRecipe(t *testing.T) {
	reg := testLayerRegistry(t)
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	child := newRuntimeRenderFacet("child", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	facet.AttachLayer(root, child, facet.LayerAttachment{
		Band:   facet.ZBandModal,
		Recipe: facet.LayerRecipeRef{Name: "bogus"},
	})
	_, err := New(func() Config {
		cfg := DefaultConfig()
		cfg.LayerRegistry = reg
		return cfg
	}(), nil, nil, &backendFixture{}, root)
	if err == nil {
		t.Fatal("New accepted an unresolvable layer recipe")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("%d", child.ID())) {
		t.Fatalf("error %q missing facet id %d", err.Error(), child.ID())
	}
}

// TestRuntimeNewAcceptsStandardLayerRecipe pins the positive path of the
// attach-time guard: the studio's canonical declaration (Recipe{Name:"modal"})
// resolves and the runtime constructs.
func TestRuntimeNewAcceptsStandardLayerRecipe(t *testing.T) {
	reg := testLayerRegistry(t)
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	child := newRuntimeRenderFacet("child", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	facet.AttachLayer(root, child, facet.LayerAttachment{
		Band:   facet.ZBandModal,
		Recipe: facet.LayerRecipeRef{Name: "modal"},
	})
	if _, err := New(func() Config {
		cfg := DefaultConfig()
		cfg.LayerRegistry = reg
		return cfg
	}(), nil, nil, &backendFixture{}, root); err != nil {
		t.Fatalf("New rejected the standard modal recipe: %v", err)
	}
}

// TestRuntimeLayerRecipeOverrideConflictingRefsFails pins the group-level rule:
// two layer children of one layer group that declare different recipe overrides
// are a configuration error, not a silently-first-wins ambiguity.
func TestRuntimeLayerRecipeOverrideConflictingRefsFails(t *testing.T) {
	rt := mustRuntime(t)
	root := rt.root
	parent := newRuntimeRenderFacet("parent", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	childA := newRuntimeRenderFacet("childA", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	childB := newRuntimeRenderFacet("childB", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	root.Base().AddChild(parent.Base())
	facet.AttachLayer(parent, childA, facet.LayerAttachment{Band: facet.ZBandModal, Recipe: facet.LayerRecipeRef{Name: "modal"}})
	facet.AttachLayer(parent, childB, facet.LayerAttachment{Band: facet.ZBandModal, Recipe: facet.LayerRecipeRef{Name: "free"}})
	children := []layout.LayerChild{
		{FacetID: childA.ID(), Attachment: facet.Attachment{}},
		{FacetID: childB.ID(), Attachment: facet.Attachment{}},
	}
	_, _, err := rt.layerRecipeOverride(children, gfx.Rect{})
	if err == nil {
		t.Fatal("conflicting recipe overrides resolved without error")
	}
	if !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("error %q missing 'conflicting'", err.Error())
	}
}

// TestRuntimeLayerRecipeOverrideSameRefCoalesces pins the group-level rule's
// positive path: children that agree on one override resolve to it once.
func TestRuntimeLayerRecipeOverrideSameRefCoalesces(t *testing.T) {
	rt := mustRuntime(t)
	root := rt.root
	parent := newRuntimeRenderFacet("parent", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	childA := newRuntimeRenderFacet("childA", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	childB := newRuntimeRenderFacet("childB", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	root.Base().AddChild(parent.Base())
	facet.AttachLayer(parent, childA, facet.LayerAttachment{Band: facet.ZBandModal, Recipe: facet.LayerRecipeRef{Name: "modal"}})
	facet.AttachLayer(parent, childB, facet.LayerAttachment{Band: facet.ZBandModal, Recipe: facet.LayerRecipeRef{Name: "modal"}})
	children := []layout.LayerChild{
		{FacetID: childA.ID(), Attachment: facet.Attachment{}},
		{FacetID: childB.ID(), Attachment: facet.Attachment{}},
	}
	recipe, ok, err := rt.layerRecipeOverride(children, gfx.Rect{})
	if err != nil {
		t.Fatalf("shared override resolved with error: %v", err)
	}
	if !ok {
		t.Fatal("shared override reported not-present")
	}
	if recipe.Grid.Columns != 1 || recipe.Grid.Rows != 1 {
		t.Fatalf("modal grid = %dx%d, want 1x1", recipe.Grid.Columns, recipe.Grid.Rows)
	}
}
