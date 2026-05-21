package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestTableMeasureProjectAnchorsAndAccessibility(t *testing.T) {
	table := newTableFixture()
	rt := cardRuntimeStub{fonts: mustCardFontRegistry(t)}
	ctx := listResolvedContext(listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)

	facet.Attach(table, facet.AttachContext{Runtime: rt, Theme: ctx})
	result := table.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 240, H: 160}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}
	bounds := gfx.RectFromXYWH(16, 16, 240, 160)
	table.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: table.layoutRole.Parent,
		ChildGroup:  table.layoutRole.Child,
	}, bounds)
	if got := table.AccessibilityRole(); got != "table" {
		t.Fatalf("accessibility role = %q, want table", got)
	}
	if got := table.AccessibleName(); got != "Sample table" {
		t.Fatalf("accessible name = %q, want Sample table", got)
	}
	if len(table.Children()) == 0 {
		t.Fatal("expected generated children")
	}
	if table.cachedVerticalTrack.IsEmpty() || table.cachedHorizontalTrack.IsEmpty() {
		t.Fatalf("expected visible scrollbars, got vertical=%#v horizontal=%#v", table.cachedVerticalTrack, table.cachedHorizontalTrack)
	}

	anchors := table.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := table.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
	}
}

func TestTableStoreChangeInvalidatesStructure(t *testing.T) {
	table := newTableFixture()
	oldVersion := table.Data.Version()
	table.SetRows([]TableRow{
		{Key: "row-1", Cells: []string{"A", "B", "C", "D"}},
		{Key: "row-2", Cells: []string{"E", "F", "G", "H"}},
	})
	if got := table.Data.Version(); got == oldVersion {
		t.Fatal("expected store version to change")
	}
	if len(table.Children()) == 0 {
		t.Fatal("expected generated children after update")
	}
}

func TestTableGoldenDefault(t *testing.T) {
	AssertTableGolden(t, "default", listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(table *Table) {})
}

func TestTableGoldenCompact(t *testing.T) {
	AssertTableGolden(t, "compact", listTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(table *Table) {})
}

func TestTableGoldenComfortable(t *testing.T) {
	AssertTableGolden(t, "comfortable", listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(table *Table) {})
}

func TestTableGoldenDisabled(t *testing.T) {
	AssertTableGolden(t, "disabled", listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(table *Table) {
		table.SetDisabled(true)
	})
}

func TestTableGoldenHighContrast(t *testing.T) {
	AssertTableGolden(t, "high_contrast", highContrastListTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(table *Table) {})
}

func TestTableGoldenRTL(t *testing.T) {
	AssertTableGolden(t, "rtl", listTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(table *Table) {})
}

func AssertTableGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Table)) {
	t.Helper()
	table := newTableFixture()
	if mutate != nil {
		mutate(table)
	}
	rt := cardRuntimeStub{fonts: mustCardFontRegistry(t)}
	ctx := listResolvedContext(tokens, density, direction)
	facet.Attach(table, facet.AttachContext{Runtime: rt, Theme: ctx})
	canvas := gfx.RectFromXYWH(16, 16, 240, 160)
	_ = table.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	bounds := canvas
	table.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: table.layoutRole.Parent,
		ChildGroup:  table.layoutRole.Child,
	}, bounds)
	cmds := table.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil {
		t.Fatal("expected projected commands")
	}
	surface := testkit.NewMemorySurface(272, 192)
	renderer := softwarerenderer.NewSoftwareRenderer()
	if err := renderer.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:          1,
			Bounds:      bounds,
			Opacity:     1,
			Commands:    *cmds,
			CommandHash: 1,
		}},
	}
	if err := renderer.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "table_"+name)
}

func newTableFixture() *Table {
	return NewTable("Sample table", TableData{
		Columns: []TableColumn{
			{Key: "id", Label: "ID"},
			{Key: "name", Label: "Long item name"},
			{Key: "status", Label: "Current status"},
			{Key: "owner", Label: "Owning team"},
		},
		Rows: []TableRow{
			{Key: "row-1", Cells: []string{"001", "Primary data row with a long title", "Ready", "Platform"}},
			{Key: "row-2", Cells: []string{"002", "Secondary data row with an even longer label", "In review", "Infrastructure"}},
			{Key: "row-3", Cells: []string{"003", "Archived data row with historical notes", "Archived", "Ops"}, Selected: true},
			{Key: "row-4", Cells: []string{"004", "Incoming data row with expanded metadata", "Queued", "Product"}},
			{Key: "row-5", Cells: []string{"005", "Pending data row with another descriptive title", "Blocked", "Research"}},
			{Key: "row-6", Cells: []string{"006", "Fulfilled data row with a very verbose label to force overflow", "Done", "Support"}},
			{Key: "row-7", Cells: []string{"007", "Duplicate data row for scroll testing", "Ready", "Design"}},
			{Key: "row-8", Cells: []string{"008", "Fallback data row for viewport testing", "Ready", "Data"}},
			{Key: "row-9", Cells: []string{"009", "Trace data row for viewport testing", "Ready", "QA"}},
			{Key: "row-10", Cells: []string{"010", "Last data row to guarantee vertical overflow", "Ready", "Ops"}},
		},
		SortColumnKey:  "name",
		SortDescending: false,
	})
}
