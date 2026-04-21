package uinav

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestTabs_pointer_selects_tab(t *testing.T) {
	tabs := &Tabs{
		Items:    []TabItem{{Key: "a"}, {Key: "b"}},
		Selected: store.NewBinding("a"),
	}
	tabs.ensureInit()
	if !tabs.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 120, Y: 10}}) {
		t.Fatal("expected press to be handled")
	}
	if got := tabs.Selected.Get(); got != "b" {
		t.Fatalf("selected = %q, want b", got)
	}
}

func TestTabs_arrow_keys_navigate(t *testing.T) {
	tabs := &Tabs{
		Items:    []TabItem{{Key: "a"}, {Key: "b"}, {Key: "c"}},
		Selected: store.NewBinding("a"),
	}
	tabs.ensureInit()
	tabs.state.focused = true
	if !tabs.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right key to be handled")
	}
	if got := tabs.Selected.Get(); got != "b" {
		t.Fatalf("selected = %q, want b", got)
	}
}

func TestTabs_indicator_animates_between_targets(t *testing.T) {
	tabs := &Tabs{
		Items:    []TabItem{{Key: "a"}, {Key: "b"}},
		Selected: store.NewBinding("a"),
	}
	tabs.ensureInit()
	start := tabs.indicator.Current()
	tabs.Selected.Set("b")
	if !tabs.Tick(80 * time.Millisecond) {
		t.Fatal("expected tick to advance indicator")
	}
	mid := tabs.indicator.Current()
	if mid == start || mid == 96 {
		t.Fatalf("indicator did not animate: start=%v mid=%v", start, mid)
	}
	if !tabs.Tick(200 * time.Millisecond) {
		t.Fatal("expected indicator to finish")
	}
	if got := tabs.indicator.Current(); got != 96 {
		t.Fatalf("indicator = %v, want 96", got)
	}
}
