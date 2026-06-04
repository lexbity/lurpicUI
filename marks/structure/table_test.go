package structure

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestTableGeometryContracts(t *testing.T) {
	table := NewTable("Geometry table", TableData{
		Columns: []TableColumn{
			{Key: "id", Label: "ID", Align: facet.AlignEnd},
			{Key: "name", Label: "Name"},
			{Key: "status", Label: "Status", Align: facet.AlignCenter},
		},
		Rows: []TableRow{
			{Key: "row-1", Cells: []string{"001", "Short title", "Open"}},
			{Key: "row-2", Cells: []string{"002", strings.Join([]string{"Line one", "Line two"}, "\n"), "Closed"}, Selected: true},
		},
		SortColumnKey: "name",
	})
	rt := cardRuntimeStub{fonts: mustCardFontRegistry(t)}
	ctx := listResolvedContext(listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)

	facet.Attach(table, facet.AttachContext{Runtime: rt, Theme: ctx})
	result := table.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 360, H: 180}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}
	bounds := gfx.RectFromXYWH(16, 16, 320, 160)
	table.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: table.Layout.Parent,
		ChildGroup:  table.Layout.Child,
	}, bounds)

	headerNameBounds, ok := tableChildBounds(table, "header:name")
	if !ok {
		t.Fatal("missing header cell bounds for name")
	}
	bodyNameRow1Bounds, ok := tableChildBounds(table, "body:row-1:name")
	if !ok {
		t.Fatal("missing row-1 body cell bounds for name")
	}
	bodyNameRow2Bounds, ok := tableChildBounds(table, "body:row-2:name")
	if !ok {
		t.Fatal("missing row-2 body cell bounds for name")
	}
	sortBounds, ok := tableChildBounds(table, "sort:name")
	if !ok {
		t.Fatal("missing sort indicator bounds for name")
	}
	selectionHeaderBounds, ok := tableChildBounds(table, "selection:header")
	if !ok {
		t.Fatal("missing selection header bounds")
	}
	selectionRowBounds, ok := tableChildBounds(table, "selection:row-2")
	if !ok {
		t.Fatal("missing row-2 selection bounds")
	}

	if headerNameBounds.Min.X != bodyNameRow1Bounds.Min.X || headerNameBounds.Width() != bodyNameRow1Bounds.Width() {
		t.Fatalf("expected header/body alignment to match, got header=%#v body=%#v", headerNameBounds, bodyNameRow1Bounds)
	}
	if bodyNameRow1Bounds.Min.X != bodyNameRow2Bounds.Min.X || bodyNameRow1Bounds.Width() != bodyNameRow2Bounds.Width() {
		t.Fatalf("expected body cells to share alignment, got row1=%#v row2=%#v", bodyNameRow1Bounds, bodyNameRow2Bounds)
	}
	if sortBounds.Min.X < headerNameBounds.Min.X || sortBounds.Max.X > headerNameBounds.Max.X {
		t.Fatalf("expected sort indicator inside header cell, got sort=%#v header=%#v", sortBounds, headerNameBounds)
	}
	if sortBounds.Min.X < headerNameBounds.Min.X+headerNameBounds.Width()*0.5 {
		t.Fatalf("expected sort indicator to be end-aligned, got sort=%#v header=%#v", sortBounds, headerNameBounds)
	}
	selectionColumnBounds := table.cachedColumnBounds["__selection__"]
	if selectionColumnBounds.IsEmpty() {
		t.Fatal("missing selection column bounds")
	}
	selectionColumnCenter := (selectionColumnBounds.Min.X + selectionColumnBounds.Max.X) * 0.5
	selectionHeaderCenter := (selectionHeaderBounds.Min.X + selectionHeaderBounds.Max.X) * 0.5
	selectionRowCenter := (selectionRowBounds.Min.X + selectionRowBounds.Max.X) * 0.5
	if selectionHeaderBounds.Min.X < selectionColumnBounds.Min.X || selectionHeaderBounds.Max.X > selectionColumnBounds.Max.X {
		t.Fatalf("expected selection header inside selection column, got header=%#v column=%#v", selectionHeaderBounds, selectionColumnBounds)
	}
	if selectionRowBounds.Min.X < selectionColumnBounds.Min.X || selectionRowBounds.Max.X > selectionColumnBounds.Max.X {
		t.Fatalf("expected selection row indicator inside selection column, got row=%#v column=%#v", selectionRowBounds, selectionColumnBounds)
	}
	if selectionHeaderCenter-selectionColumnCenter > 0.01 || selectionColumnCenter-selectionHeaderCenter > 0.01 || selectionRowCenter-selectionColumnCenter > 0.01 || selectionColumnCenter-selectionRowCenter > 0.01 {
		t.Fatalf("expected selection cells to be centered in the selection column, got header=%#v row=%#v column=%#v", selectionHeaderBounds, selectionRowBounds, selectionColumnBounds)
	}

	headerRowBounds := table.cachedRowBounds["__header__"]
	if headerRowBounds.IsEmpty() {
		t.Fatalf("expected header row bounds, got %#v", headerRowBounds)
	}
	if bodyNameRow2Bounds.Height() <= bodyNameRow1Bounds.Height() {
		t.Fatalf("expected multiline row to be taller, got row1=%#v row2=%#v", bodyNameRow1Bounds, bodyNameRow2Bounds)
	}
	if bodyNameRow1Bounds.Min.Y <= headerRowBounds.Max.Y {
		t.Fatalf("expected row 1 below header with gap, got header=%#v row1=%#v", headerRowBounds, bodyNameRow1Bounds)
	}
	if bodyNameRow2Bounds.Min.Y <= headerRowBounds.Max.Y {
		t.Fatalf("expected row 2 below header with gap, got header=%#v row2=%#v", headerRowBounds, bodyNameRow2Bounds)
	}
	upperRowBounds := bodyNameRow1Bounds
	lowerRowBounds := bodyNameRow2Bounds
	if bodyNameRow2Bounds.Min.Y < bodyNameRow1Bounds.Min.Y {
		upperRowBounds, lowerRowBounds = bodyNameRow2Bounds, bodyNameRow1Bounds
	}
	if lowerRowBounds.Min.Y <= upperRowBounds.Max.Y {
		t.Fatalf("expected row gap between sorted rows, got upper=%#v lower=%#v", upperRowBounds, lowerRowBounds)
	}
}

func TestTableMeasureProjectAnchorsAndAccessibility(t *testing.T) {
	table := newTableFixture()
	rt := cardRuntimeStub{fonts: mustCardFontRegistry(t)}
	ctx := listResolvedContext(listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)

	facet.Attach(table, facet.AttachContext{Runtime: rt, Theme: ctx})
	result := table.Layout.Measure(facet.MeasureContext{
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
	table.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: table.Layout.Parent,
		ChildGroup:  table.Layout.Child,
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
	expectBoundsAnchors(t, anchors, bounds)
	if got, ok := anchors["viewport"]; !ok {
		t.Fatal("missing viewport anchor")
	} else if want := rectCenter(bounds); got != want {
		t.Fatalf("viewport anchor = %#v, want %#v", got, want)
	}
	if !table.cachedContentBounds.IsEmpty() {
		if got, ok := anchors["content"]; !ok {
			t.Fatal("missing content anchor")
		} else if want := rectCenter(table.cachedContentBounds); got != want {
			t.Fatalf("content anchor = %#v, want %#v", got, want)
		}
	}

	cmds := table.Projection.Project(facet.ProjectionContext{
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
	data := table.Data.Get()
	data.Rows = []TableRow{
		{Key: "row-1", Cells: []string{"A", "B", "C", "D"}},
		{Key: "row-2", Cells: []string{"E", "F", "G", "H"}},
	}
	table.Data.Set(data)
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
		table.Disabled = marks.Const(true)
	})
}

func TestTableGoldenHighContrast(t *testing.T) {
	AssertTableGolden(t, "high_contrast", highContrastListTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(table *Table) {})
}

func TestTableGoldenRTL(t *testing.T) {
	ltr := AssertTableGolden(t, "default", listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(table *Table) {})
	rtl := AssertTableGolden(t, "rtl", listTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(table *Table) {})
	testkit.AssertDiffers(t, ltr, rtl, "table")
}

func AssertTableGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Table)) *testkit.MemorySurface {
	t.Helper()
	table := newTableGoldenFixture()
	if mutate != nil {
		mutate(table)
	}
	rt := cardRuntimeStub{fonts: mustCardFontRegistry(t)}
	ctx := listResolvedContext(tokens, density, direction)
	facet.Attach(table, facet.AttachContext{Runtime: rt, Theme: ctx})
	canvas := gfx.RectFromXYWH(16, 16, 2136, 810)
	_ = table.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	bounds := canvas
	table.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: table.Layout.Parent,
		ChildGroup:  table.Layout.Child,
	}, bounds)
	cmds := table.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil {
		t.Fatal("expected projected commands")
	}
	surface := testkit.NewMemorySurface(2170, 842)
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
	return surface
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

func newTableGoldenFixture() *Table {
	return NewTable("Load balancer table", TableData{
		Columns: []TableColumn{
			{Key: "name", Label: "Name"},
			{Key: "rule", Label: "Rule"},
			{Key: "status", Label: "Status"},
			{Key: "other", Label: "Other"},
			{Key: "example", Label: "Example"},
		},
		Rows: []TableRow{
			{Key: "row-1", Cells: []string{"Load Balancer 1", "Round robin", "Starting", "Test", "22"}},
			{Key: "row-2", Cells: []string{"Load Balancer 2", "DNS delegation", "Active", "Test", "22"}},
			{Key: "row-3", Cells: []string{"Load Balancer 3", "Round robin", "Disabled", "Test", "22"}},
			{Key: "row-4", Cells: []string{"Load Balancer 4", "Round robin", "Disabled", "Test", "22"}},
			{Key: "row-5", Cells: []string{"Load Balancer 5", "Round robin", "Disabled", "Test", "22"}},
			{Key: "row-6", Cells: []string{"Load Balancer 6", "Round robin", "Disabled", "Test", "22"}},
			{Key: "row-7", Cells: []string{"Load Balancer 7", "Round robin", "Disabled", "Test", "22"}},
		},
	})
}

func tableChildBounds(table *Table, key string) (gfx.Rect, bool) {
	if table == nil {
		return gfx.Rect{}, false
	}
	for i := range table.cachedChildSpecs {
		spec := table.cachedChildSpecs[i]
		if spec.Key != key || spec.Facet == nil {
			continue
		}
		base := spec.Facet.Base()
		if base == nil {
			return gfx.Rect{}, false
		}
		bounds, ok := table.cachedCellBounds[base.ID()]
		return bounds, ok
	}
	return gfx.Rect{}, false
}
