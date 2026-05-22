package action

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	input "codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestRadialMenuComposesExistingMarks(t *testing.T) {
	center := input.NewColorPicker("Palette")
	split := NewSplitButton("Brush", []SplitButtonItem{
		{Key: "soft", Label: "Soft", IconRef: "brush"},
	})
	toolbar := NewToolbar("Canvas", []ToolbarGroup{
		{
			Key: "canvas",
			Actions: []ActionGroupAction{
				{Key: "mirror", Label: "Mirror", IconRef: "mirror"},
			},
		},
	}, nil)

	menu := NewRadialMenu("Radial", center, []RadialChild{
		{Child: split, Placement: facet.RadialPlacement{Angle: 0, RadiusTrack: 120}},
		{Child: toolbar, Placement: facet.RadialPlacement{Angle: math.Pi / 2, RadiusTrack: 120}},
	})

	children := menu.Children()
	if len(children) != 3 {
		t.Fatalf("Children() len = %d, want 3", len(children))
	}
	if children[0].MarkID != radialMenuMarkIDCenterSlot {
		t.Fatalf("center MarkID = %d, want %d", children[0].MarkID, radialMenuMarkIDCenterSlot)
	}
	if children[1].Attachment.Placement.Mode != facet.PlacementRadial || children[2].Attachment.Placement.Mode != facet.PlacementRadial {
		t.Fatal("expected radial placement for orbit children")
	}

	resolved := theme.DefaultResolvedContext()
	size := menu.measure(facet.MeasureContext{Theme: resolved, WritingDirection: facet.WritingDirectionLTR}, facet.Constraints{MaxSize: gfx.Size{W: 360, H: 360}}).Size
	if size.W <= 0 || size.H <= 0 {
		t.Fatalf("measure size = %#v, want positive", size)
	}

	menu.arrange(facet.ArrangeContext{Theme: resolved, Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(0, 0, size.W, size.H))
	if center.Base().LayoutRole().ArrangedBounds.IsEmpty() {
		t.Fatal("expected center child to be arranged")
	}
	if split.Base().LayoutRole().ArrangedBounds.IsEmpty() {
		t.Fatal("expected split button child to be arranged")
	}
	if toolbar.Base().LayoutRole().ArrangedBounds.IsEmpty() {
		t.Fatal("expected toolbar child to be arranged")
	}
	if len(menu.cachedArrangedChildren) != 3 {
		t.Fatalf("cached arranged count = %d, want 3", len(menu.cachedArrangedChildren))
	}
	if len(menu.buildCommands(menu.layoutRole.ArrangedBounds, nil, 1)) == 0 {
		t.Fatal("expected shell geometry commands")
	}
}

func TestRadialMenuDismissals(t *testing.T) {
	menu := NewRadialMenu("Radial", nil, nil)
	menu.SetOpen(true)

	if !menu.onDismiss(facet.DismissEvent{Trigger: facet.DismissalTriggerPointer}) {
		t.Fatal("expected pointer dismissal to be handled")
	}
	if menu.Open {
		t.Fatal("expected menu to close on dismiss")
	}

	menu.SetOpen(true)
	if !menu.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Button: platform.PointerLeft, Position: gfx.Point{X: 5, Y: 5}}) {
		t.Fatal("expected outside press to dismiss the menu")
	}
	if menu.Open {
		t.Fatal("expected menu to close on outside press")
	}

	menu.SetOpen(true)
	if !menu.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEscape}) {
		t.Fatal("expected escape to dismiss the menu")
	}
	if menu.Open {
		t.Fatal("expected menu to close on escape")
	}
}
