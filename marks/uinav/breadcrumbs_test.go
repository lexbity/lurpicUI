package uinav

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestBreadcrumbs_render_all_when_space_available(t *testing.T) {
	b := &Breadcrumbs{
		Items: []BreadcrumbItem{{Key: "a", Label: "Home"}, {Key: "b", Label: "Library"}, {Key: "c", Label: "Docs"}},
		Current: store.NewBinding("c"),
	}
	items := b.visibleItems(1000)
	if len(items) != 3 {
		t.Fatalf("visible items = %d, want 3", len(items))
	}
}

func TestBreadcrumbs_collapse_middle_when_constrained(t *testing.T) {
	b := &Breadcrumbs{
		Items: []BreadcrumbItem{{Key: "a", Label: "Home"}, {Key: "b", Label: "Library"}, {Key: "c", Label: "Docs"}, {Key: "d", Label: "Detail"}},
		Current: store.NewBinding("d"),
	}
	items := b.visibleItems(80)
	if len(items) != 3 || items[1].Key != "..." {
		t.Fatalf("collapsed items = %#v", items)
	}
}

func TestBreadcrumbs_current_item_styled_distinctly(t *testing.T) {
	b := &Breadcrumbs{
		Items: []BreadcrumbItem{{Key: "a", Label: "Home"}, {Key: "b", Label: "Library"}, {Key: "c", Label: "Docs"}},
		Current: store.NewBinding("b"),
	}
	list := b.project(facet.ProjectionContext{})
	if list == nil || len(list.Commands) == 0 {
		t.Fatal("expected commands")
	}
	foundCurrent := false
	for _, cmd := range list.Commands {
		if fill, ok := cmd.(gfx.FillRect); ok {
			if fill.Brush.Color == (gfx.Color{R: 1, G: 1, B: 1, A: 1}) {
				continue
			}
			foundCurrent = true
			break
		}
	}
	if !foundCurrent {
		t.Fatal("expected current breadcrumb to use distinct styling")
	}
}
