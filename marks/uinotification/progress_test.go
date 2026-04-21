package uinotification

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uinotification"
)

func TestProgress_linear_determinate_uses_value_fraction(t *testing.T) {
	p := &Progress{
		Mode:  ProgressDeterminate,
		Shape: ProgressLinear,
		Value: store.NewBinding(0.5),
	}
	rect := p.linearFillRect()
	if got := rect.Width(); got != 120 {
		t.Fatalf("linear fill width = %v, want 120", got)
	}
}

func TestProgress_circular_determinate_arc_matches_fraction(t *testing.T) {
	p := &Progress{
		Mode:  ProgressDeterminate,
		Shape: ProgressCircular,
		Value: store.NewBinding(0.25),
	}
	path := p.circularPath()
	if len(path.Segments) < 2 {
		t.Fatalf("circular path segments = %d, want > 1", len(path.Segments))
	}
	last := path.Segments[len(path.Segments)-1].Pts[0]
	if last == (gfx.Point{}) {
		t.Fatal("expected non-zero arc endpoint")
	}
}

func TestProgress_indeterminate_animates(t *testing.T) {
	p := &Progress{
		Mode:  ProgressIndeterminate,
		Shape: ProgressLinear,
	}
	p.ensureInit()
	if !p.Tick(16 * time.Millisecond) {
		t.Fatal("expected indeterminate progress to tick")
	}
}

func TestProgress_recipe_slots_present(t *testing.T) {
	slots, report := uirecipe.ResolveProgressRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if _, ok := report.SlotSource("Track"); !ok {
		t.Fatal("expected progress recipe report to include track slot")
	}
	if slots.Track.Base.Fills == nil || slots.Indicator.Base.Fills == nil {
		t.Fatal("expected progress slots to be populated")
	}
	p := &Progress{
		Mode:  ProgressDeterminate,
		Shape: ProgressLinear,
		Value: store.NewBinding(0.5),
	}
	if specs := p.project(facet.ProjectionContext{}); specs == nil || len(specs.Commands) == 0 {
		t.Fatal("expected progress projection commands")
	}
}
