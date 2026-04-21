package basic

import (
	"reflect"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestRect_projects_fill_and_stroke(t *testing.T) {
	rect := &Rect{
		ID:     "rect-1",
		Bounds: BoundsProps{X: 10, Y: 20, W: 30, H: 40},
		Style: PrimitiveStyleProps{
			Fill: theme.Material{
				Fills:   []theme.Fill{{Type: theme.FillSolid, Color: gfx.Color{R: 1, A: 1}, Opacity: 1}},
				Opacity: 1,
			},
			Stroke: theme.MaterialStroke{
				Paint: theme.Fill{Type: theme.FillSolid, Color: gfx.Color{B: 1, A: 1}, Opacity: 1},
				Width: 2,
			},
			Opacity: 1,
			Visible: true,
		},
	}
	cmds := renderMark(t, rect)
	if !containsCommandType(cmds, gfx.FillRect{}) {
		t.Fatalf("expected fill rect command, got %#v", cmds)
	}
	if !containsCommandType(cmds, gfx.StrokeRect{}) {
		t.Fatalf("expected stroke rect command, got %#v", cmds)
	}
}

func TestRect_hit_inside_and_outside(t *testing.T) {
	rect := &Rect{
		Bounds: BoundsProps{X: 0, Y: 0, W: 10, H: 20},
		Style:  PrimitiveStyleProps{Visible: true, Opacity: 1},
	}
	if !rect.HitTest(gfx.Point{X: 5, Y: 10}) {
		t.Fatal("expected inside point to hit")
	}
	if rect.HitTest(gfx.Point{X: 50, Y: 10}) {
		t.Fatal("expected outside point to miss")
	}
}

func TestRect_rounded_corner_respects_radius(t *testing.T) {
	square := &Rect{
		Bounds: BoundsProps{X: 0, Y: 0, W: 20, H: 20},
		Style:  PrimitiveStyleProps{Fill: theme.Material{Fills: []theme.Fill{{Type: theme.FillSolid, Color: gfx.Color{A: 1}, Opacity: 1}}, Opacity: 1}, Visible: true, Opacity: 1},
	}
	rounded := &Rect{
		Bounds: BoundsProps{X: 0, Y: 0, W: 20, H: 20},
		Radius: 4,
		Style:  PrimitiveStyleProps{Fill: theme.Material{Fills: []theme.Fill{{Type: theme.FillSolid, Color: gfx.Color{A: 1}, Opacity: 1}}, Opacity: 1}, Visible: true, Opacity: 1},
	}
	squareCmds := renderMark(t, square)
	roundedCmds := renderMark(t, rounded)
	if containsCommandType(roundedCmds, gfx.FillRect{}) || containsCommandType(roundedCmds, gfx.StrokeRect{}) {
		t.Fatalf("expected rounded rect to use path commands, got %#v", roundedCmds)
	}
	if !containsCommandType(roundedCmds, gfx.FillPath{}) && !containsCommandType(roundedCmds, gfx.StrokePath{}) {
		t.Fatalf("expected rounded rect to use path commands, got %#v", roundedCmds)
	}
	if !containsCommandType(squareCmds, gfx.FillRect{}) {
		t.Fatalf("expected square rect fill command, got %#v", squareCmds)
	}
}

func TestRect_exports_all_cardinal_anchors(t *testing.T) {
	rect := &Rect{
		Bounds: BoundsProps{X: 10, Y: 20, W: 30, H: 40},
		Style:  PrimitiveStyleProps{Visible: true, Opacity: 1},
	}
	anchors := rect.ExportAnchors(layout.AnchorExportContext{})
	for _, name := range []layout.AnchorID{"center", "top-left", "top", "top-right", "right", "bottom-right", "bottom", "bottom-left", "left"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
}

func TestRect_transform_affects_hit_and_anchors(t *testing.T) {
	rect := &Rect{
		Bounds: BoundsProps{X: 0, Y: 0, W: 10, H: 10},
		Style:  PrimitiveStyleProps{Visible: true, Opacity: 1},
		Tx:     TransformProps{Transform: gfx.Translation(12, 8)},
	}
	if !rect.HitTest(gfx.Point{X: 15, Y: 12}) {
		t.Fatal("expected transformed point to hit")
	}
	anchors := rect.ExportAnchors(layout.AnchorExportContext{})
	got, ok := anchors["center"]
	if !ok {
		t.Fatal("missing center anchor")
	}
	want := gfx.Point{X: 17, Y: 13}
	if got != want {
		t.Fatalf("center = %+v want %+v", got, want)
	}
}

func containsCommandType(cmds []gfx.Command, want any) bool {
	wantType := reflect.TypeOf(want)
	for _, cmd := range cmds {
		if reflect.TypeOf(cmd) == wantType {
			return true
		}
	}
	return false
}
