package uinav

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/store"
)

func TestPhase5TabsMenuScrollbarDrawerGeometry(t *testing.T) {
	tabs := &Tabs{Items: []TabItem{{Key: "a"}, {Key: "b"}}, Selected: store.NewBinding("a")}
	if got := tabs.bounds(); got.Width() != 192 || got.Height() != 40 {
		t.Fatalf("tabs bounds = %#v", got)
	}
	if got := tabs.itemBounds(1); got.Width() != 96 || got.Height() != 40 {
		t.Fatalf("tab item bounds = %#v", got)
	}

	menu := &Menu{Items: []MenuItem{{Key: "a"}, {Key: "b"}}}
	if got := menu.bounds(); got.Width() != 220 || got.Height() != 64 {
		t.Fatalf("menu bounds = %#v", got)
	}
	menu.Dense = true
	if got := menu.bounds(); got.Width() != 220 || got.Height() != 52 {
		t.Fatalf("dense menu bounds = %#v", got)
	}

	scrollV := &Scrollbar{Orientation: ScrollbarVertical}
	if got := scrollV.bounds(); got.Width() != 12 || got.Height() != 240 {
		t.Fatalf("vertical scrollbar bounds = %#v", got)
	}
	scrollH := &Scrollbar{Orientation: ScrollbarHorizontal}
	if got := scrollH.bounds(); got.Width() != 240 || got.Height() != 12 {
		t.Fatalf("horizontal scrollbar bounds = %#v", got)
	}

	drawer := &Drawer{}
	if got := drawer.bounds(); got.Width() != 240 || got.Height() != 320 {
		t.Fatalf("drawer bounds = %#v", got)
	}
	if got := drawer.surfaceBounds(); got.Width() != 160 || got.Height() != 320 {
		t.Fatalf("drawer surface bounds = %#v", got)
	}
}
