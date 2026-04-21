package uinav

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestPagination_generates_expected_window(t *testing.T) {
	p := &Pagination{Page: store.NewBinding(5), TotalPages: 10, WindowSize: 5}
	items := p.windowItems()
	if len(items) == 0 || items[0].Page != 1 {
		t.Fatalf("window items = %#v", items)
	}
	if items[len(items)-1].Page != 10 {
		t.Fatalf("window items = %#v", items)
	}
}

func TestPagination_ellipsis_rules(t *testing.T) {
	p := &Pagination{Page: store.NewBinding(5), TotalPages: 10, WindowSize: 5}
	items := p.windowItems()
	ellipses := 0
	for _, item := range items {
		if item.Ellipsis {
			ellipses++
		}
	}
	if ellipses != 2 {
		t.Fatalf("ellipsis count = %d, want 2", ellipses)
	}
}

func TestPagination_click_updates_page_store(t *testing.T) {
	p := &Pagination{Page: store.NewBinding(1), TotalPages: 3, WindowSize: 5}
	p.ensureInit()
	if !p.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 40, Y: 10}}) {
		t.Fatal("expected page click to be handled")
	}
	if got := p.Page.Get(); got != 2 {
		t.Fatalf("page = %d, want 2", got)
	}
}
