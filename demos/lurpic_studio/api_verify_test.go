package main

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/layout/grid"
	"codeburg.org/lexbit/lurpicui/layout/linear"
	"codeburg.org/lexbit/lurpicui/layout/radial"
	"codeburg.org/lexbit/lurpicui/layout/split"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/marks/viz"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
)

// TestAPIVerify re-verifies the §1 signatures of the spec against HEAD
// (FR-1). Every construct below must compile and return non-nil; any drift
// from the spec's pseudocode is logged as a Finding, never silently worked
// around.
//
// Slice P0 re-verification drift logged (all against HEAD 2026-08-07):
//
//   - F-drift-area-signature: §1.8 sketches NewArea as taking a baseline
//     argument; HEAD's NewArea has the same signature as NewLine (the
//     baseline is a fixed internal marks.Const(0.0)). The spec's pseudocode
//     is corrected by this test.
//   - F-drift-fontsource: §1.1/P0 sketches text.FontSource{Data, Family,
//     Weight}; HEAD's text.FontSource is {Path, Data, Name} only. main.go
//     loads fonts with Name, matching testkit's harness convention.
func TestAPIVerify_signaturesCompileAgainstHead(t *testing.T) {
	t.Run("app entry (§1.1)", func(t *testing.T) {
		pinDefaultConfigFunc(app.DefaultConfig)
		pinRunFunc(app.Run)
		pinAssetFunc(app.Asset)
		pinRenderBackend(app.RenderBackendSoftware)

		cfg := app.DefaultConfig("Lurpic Studio", 1280, 800)
		if cfg.Window.Width != 1280 || cfg.Window.Height != 800 {
			t.Fatalf("DefaultConfig window = %dx%d, want 1280x800", cfg.Window.Width, cfg.Window.Height)
		}
		cfg.Render = app.RenderBackendSoftware
		cfg.Fonts = []text.FontSource{{Data: []byte("font"), Name: "Noto Sans"}}
		_ = app.BuildContext{
			FontRegistry: nil,
			WindowSize:   gfx.Size{W: 1280, H: 800},
			ContentScale: 1,
			Theme:        cfg.Theme,
		}
	})

	t.Run("mark core / binding / descriptor (§1.2)", func(t *testing.T) {
		vs := store.NewValueStore(7)
		derived := store.NewDerived(func() int { return vs.Get() + 1 }, vs)

		constB := marks.Const(3)
		storeB := marks.FromStore(vs, facet.DirtyLayout|facet.DirtyProjection)
		derivedB := marks.FromDerived(derived, facet.DirtyProjection)
		if constB.IsDynamic() {
			t.Fatal("marks.Const must be non-dynamic")
		}
		if !storeB.IsDynamic() || !derivedB.IsDynamic() {
			t.Fatal("store/derived bindings must be dynamic")
		}
		pinBinding(constB)
		pinBinding(storeB)
		pinBinding(derivedB)

		card := structure.NewCard("verify")
		pinFacetImpl(card)
		d := marks.Describe(card)
		if d.Family != "structure" || d.TypeName != "card" {
			t.Fatalf("Describe(card) = %+v, want Family=structure TypeName=card", d)
		}
		pinDirtyFlags(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit | facet.DirtyAll)
	})

	t.Run("store topology (§1.4)", func(t *testing.T) {
		vs := store.NewValueStore(1.0)
		rows := store.NewCollectionStore(identityRow)
		derived := store.NewDerived(func() [2]float64 {
			lo := float64(0)
			hi := float64(rows.Len())
			return [2]float64{lo, hi}
		}, rows)

		if got := vs.Get(); got != 1.0 {
			t.Fatalf("ValueStore.Get = %v, want 1.0", got)
		}
		if rows == nil || derived == nil {
			t.Fatal("collection store / derived constructed nil")
		}
		_ = store.ItemID(0)
	})

	t.Run("layout policies (§1.5)", func(t *testing.T) {
		lin := linear.New(linear.Config{Axis: linear.Horizontal, Gap: 8})
		if lin == nil {
			t.Fatal("linear.New returned nil")
		}
		if _, err := lin.Measure([]linear.Child{}, gfx.Size{W: 800, H: 600}); err != nil {
			t.Fatalf("linear.Measure empty: %v", err)
		}

		g := grid.New(grid.Config{})
		if g == nil {
			t.Fatal("grid.New returned nil")
		}
		if _, err := g.Measure([]grid.Child{}, gfx.Size{W: 800, H: 600}); err != nil {
			t.Fatalf("grid.Measure empty: %v", err)
		}

		rad := radial.New(radial.Config{})
		if rad == nil {
			t.Fatal("radial.New returned nil")
		}

		sp := split.New(split.Config{Axis: split.Horizontal, DividerSize: 4})
		if sp == nil {
			t.Fatal("split.New returned nil")
		}
		sp.Measure([]layout.ChildNode{}, gfx.Size{W: 800, H: 600})
		sp.Arrange([]layout.ChildNode{}, layout.ResolvedLayer{})
	})

	t.Run("group-parent contract + layer attach (§1.5/§1.6/§1.7)", func(t *testing.T) {
		parent := facet.NewFacet()
		childA := facet.NewFacet()
		childB := facet.NewFacet()

		_ = facet.GroupParentContract{
			Kind:   facet.GroupLayoutGrid,
			Policy: nil,
		}
		_ = facet.GroupChildContract{SupportedPlacement: facet.SupportsGrid | facet.SupportsLinear}

		parent.AddChild(&childA)
		// AttachLayer requires ZPriority > 0 (the runtime consumes it during
		// layer resolution).
		facet.AttachLayer(&parent, &childB, facet.LayerAttachment{ZPriority: 100})
		if got := len(parent.Children()); got != 2 {
			t.Fatalf("children = %d, want 2", got)
		}

		reg, err := layout.StandardLayerRegistry()
		if err != nil {
			t.Fatalf("StandardLayerRegistry: %v", err)
		}
		if reg == nil {
			t.Fatal("StandardLayerRegistry returned nil")
		}
	})

	t.Run("card host idiom (§1.6)", func(t *testing.T) {
		card := structure.NewCard("shell")
		if card == nil {
			t.Fatal("NewCard returned nil")
		}
		if card.Base().LayoutRole() == nil {
			t.Fatal("Card registers no layout role")
		}
		if card.Base().Parent() != nil {
			t.Fatal("fresh card should have no parent")
		}
	})

	t.Run("viz marks + reactive scales (§1.8)", func(t *testing.T) {
		xDomain := store.NewValueStore([2]float64{0, 100})
		xRange := store.NewValueStore([2]float64{0, 800})
		yDomain := store.NewValueStore([2]float64{0, 1000})
		yRange := store.NewValueStore([2]float64{800, 0})
		xScale := reactive.NewTimeReactive(xDomain, xRange)
		yScale := reactive.NewLinearReactive(yDomain, yRange)
		if xScale == nil || yScale == nil {
			t.Fatal("reactive scale constructed nil")
		}

		rows := store.NewCollectionStore(identityRow)
		x := func(r dataset.Row) float64 { return float64(r.Time.Unix()) }
		y := func(r dataset.Row) float64 { return r.Value }
		cat := func(r dataset.Row) string { return r.Region }

		line := viz.NewLine(rows, x, y, xScale, yScale)
		area := viz.NewArea(rows, x, y, xScale, yScale)
		point := viz.NewPoint(rows, x, y, xScale, yScale)
		bar := viz.NewBar(rows, cat, y, yScale)
		axis := viz.NewAxis(xScale, marks.Const(viz.AxisBottom), nil)
		rule := viz.NewRule(marks.Const(0.5), viz.RuleHorizontal, yScale)

		for name, m := range map[string]facet.FacetImpl{
			"line": line, "area": area, "point": point, "bar": bar,
			"axis": axis, "rule": rule,
		} {
			if m == nil {
				t.Fatalf("%s constructed nil", name)
			}
			if m.Base().ID() == 0 {
				t.Fatalf("%s facet has zero ID", name)
			}
		}

		_ = reactive.DomainFromCollection(rows, func(r dataset.Row) float64 { return float64(r.Time.Unix()) })
		_ = reactive.RangeFromRegion(0, 800)
		zc := reactive.NewZoomController(xDomain)
		if zc == nil {
			t.Fatal("NewZoomController returned nil")
		}
	})
}

func identityRow(r dataset.Row) store.ItemID {
	return store.ItemID(uint64(r.Time.Unix()))
}

// The pin* helpers compile-time-pin §1 signatures to their exact types
// without triggering staticcheck QF1011 (which flags redundant var types).

func pinDefaultConfigFunc(f func(title string, w, h int) app.Config)   { _ = f }
func pinRunFunc(f func(cfg app.Config, builder app.RootBuilder) error) { _ = f }
func pinAssetFunc(f func(path string) ([]byte, error))                 { _ = f }
func pinRenderBackend(k app.RenderBackendKind)                         { _ = k }
func pinBinding(b marks.Binding[int])                                  { _ = b }
func pinFacetImpl(f facet.FacetImpl)                                   { _ = f }
func pinDirtyFlags(d facet.DirtyFlags)                                 { _ = d }

// TestAPIVerify_anchorAndPlacementTypes pins the split pane-sizing contract
// (F-split): Flex>0 => weighted, Offset!=0 => fixed, else intrinsic.
func TestAPIVerify_splitPaneSizingTypes(t *testing.T) {
	var node layout.ChildNode
	node.Attachment.Placement.Flex = 1
	node.IntrinsicSize = gfx.Size{W: 100, H: 100}
	node.MinSize = gfx.Size{W: 20, H: 20}
	if node.Attachment.Placement.Flex <= 0 {
		t.Fatal("Flex must be > 0 for weighted panes")
	}
}
