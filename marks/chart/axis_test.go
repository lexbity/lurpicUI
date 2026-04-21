package chart

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/theme"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/chart"
)

type axisRangeScale struct {
	length *float32
}

func (s axisRangeScale) Kind() ScaleKind { return ScaleLinear }

func (s axisRangeScale) Map(value any) float32 {
	length := float32(240)
	if s.length != nil {
		length = *s.length
	}
	if v, ok := value.(float64); ok {
		return float32(v / 100 * float64(length))
	}
	return 0
}

func (s axisRangeScale) Ticks(desired int) []any {
	if desired <= 0 {
		desired = 5
	}
	out := make([]any, desired)
	for i := range out {
		out[i] = float64(i) * 25
	}
	return out
}

func (s axisRangeScale) FormatTick(value any) string {
	return "tick"
}

func TestAxis_linear_ticks_monotonic(t *testing.T) {
	a := &Axis{
		Orientation: AxisBottom,
		Scale: LinearScale{
			DomainMin: 0,
			DomainMax: 100,
			RangeMin:  0,
			RangeMax:  240,
		},
	}
	positions := a.tickPositions()
	if len(positions) < 2 {
		t.Fatalf("positions = %#v", positions)
	}
	for i := 1; i < len(positions); i++ {
		if positions[i] < positions[i-1] {
			t.Fatalf("positions not monotonic: %#v", positions)
		}
	}
}

func TestAxis_vertical_positions_match_scale(t *testing.T) {
	a := &Axis{
		Orientation: AxisLeft,
		Scale: LinearScale{
			DomainMin: 0,
			DomainMax: 10,
			RangeMin:  20,
			RangeMax:  220,
		},
	}
	pos := a.tickPositions()
	if got := pos[0]; got != 20 {
		t.Fatalf("first position = %v, want 20", got)
	}
	if got := pos[len(pos)-1]; got != 220 {
		t.Fatalf("last position = %v, want 220", got)
	}
}

func TestAxis_label_layout_nonoverlap_under_nominal_density(t *testing.T) {
	a := &Axis{
		Orientation: AxisBottom,
		Title:       "Sales",
		Scale: LinearScale{
			DomainMin: 0,
			DomainMax: 100,
			RangeMin:  0,
			RangeMax:  240,
		},
	}
	labels := a.labelBoxes()
	for i := 1; i < len(labels); i++ {
		if labels[i].Min.X < labels[i-1].Max.X {
			t.Fatalf("labels overlap: %#v then %#v", labels[i-1], labels[i])
		}
	}
}

func TestAxis_gridline_generation_matches_tick_positions(t *testing.T) {
	a := &Axis{
		Orientation: AxisBottom,
		ShowGrid:    true,
		Scale: LinearScale{
			DomainMin: 0,
			DomainMax: 100,
			RangeMin:  0,
			RangeMax:  240,
		},
	}
	for i, pos := range a.tickPositions() {
		grid := a.gridLineRects(pos)
		if len(grid) != 1 {
			t.Fatalf("tick %d grid rects = %#v", i, grid)
		}
		if grid[0].Min.X != pos {
			t.Fatalf("grid line = %#v, want x=%v", grid[0], pos)
		}
	}
}

func TestAxis_title_position_by_orientation(t *testing.T) {
	top := &Axis{Orientation: AxisTop, Title: "Top"}
	bottom := &Axis{Orientation: AxisBottom, Title: "Bottom"}
	if got := top.titleBox(); got.Min.Y != 0 {
		t.Fatalf("top title y = %v, want 0", got.Min.Y)
	}
	if got := bottom.titleBox(); got.Max.Y != bottom.bounds().Height() {
		t.Fatalf("bottom title = %#v, want aligned to bottom", got)
	}
}

func TestAxis_slots_apply_to_all_parts(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uirecipe.ResolveAxisRecipe(ctx, uirecipe.AxisStandard)
	if _, ok := report.SlotSource("AxisLine"); !ok {
		t.Fatal("expected axis line slot source")
	}
	if slots.AxisLine.Base.Fills == nil || slots.Tick.Base.Fills == nil || slots.Title.Base.Fills == nil {
		t.Fatal("expected axis recipe slots to be populated")
	}
}

func TestAxis_tick_anchor_exports(t *testing.T) {
	a := &Axis{
		Orientation: AxisRight,
		Scale: LinearScale{
			DomainMin: 0,
			DomainMax: 10,
			RangeMin:  20,
			RangeMax:  220,
		},
		Title: "Right axis",
	}
	anchors := a.ExportAnchors(layout.AnchorExportContext{})
	if _, ok := anchors["baseline-start"]; !ok {
		t.Fatal("expected baseline-start anchor")
	}
	if _, ok := anchors["tick-0"]; !ok {
		t.Fatal("expected tick-0 anchor")
	}
	if _, ok := anchors["title-center"]; !ok {
		t.Fatal("expected title-center anchor")
	}
}

func TestAxis_projection_cache_hits(t *testing.T) {
	a := &Axis{
		Orientation: AxisBottom,
		Scale: LinearScale{
			DomainMin: 0,
			DomainMax: 100,
			RangeMin:  0,
			RangeMax:  240,
		},
	}
	if a.project(facet.ProjectionContext{}) == nil {
		t.Fatal("expected projection list")
	}
	if a.project(facet.ProjectionContext{}) == nil {
		t.Fatal("expected projection list on cache hit")
	}
	hits, misses := a.CacheStats()
	if hits == 0 || misses == 0 {
		t.Fatalf("cache stats = hits:%d misses:%d", hits, misses)
	}
}

func BenchmarkAxis_projection_cache_hit(b *testing.B) {
	a := &Axis{
		Orientation: AxisBottom,
		Scale: LinearScale{
			DomainMin: 0,
			DomainMax: 100,
			RangeMin:  0,
			RangeMax:  240,
		},
	}
	_ = a.project(facet.ProjectionContext{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = a.project(facet.ProjectionContext{})
	}
}

func TestAxis_projection_cache_invalidates_on_range_change(t *testing.T) {
	length := float32(240)
	a := &Axis{
		Orientation: AxisBottom,
		Scale:       axisRangeScale{length: &length},
	}
	_ = a.project(facet.ProjectionContext{})
	length = 360
	_ = a.project(facet.ProjectionContext{})
	hits, misses := a.CacheStats()
	if hits != 0 || misses != 2 {
		t.Fatalf("cache stats = hits:%d misses:%d, want hits:0 misses:2", hits, misses)
	}
}
