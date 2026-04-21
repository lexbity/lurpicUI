package basic

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestEllipse_hit_equation(t *testing.T) {
	ellipse := &Ellipse{
		Bounds: BoundsProps{X: 0, Y: 0, W: 20, H: 10},
		Style:  PrimitiveStyleProps{Visible: true, Opacity: 1},
	}
	if !ellipse.HitTest(gfx.Point{X: 10, Y: 5}) {
		t.Fatal("expected center point to hit")
	}
	if ellipse.HitTest(gfx.Point{X: 19, Y: 9}) {
		t.Fatal("expected outside point to miss")
	}
}

func TestEllipse_anchors_cardinal_positions(t *testing.T) {
	ellipse := &Ellipse{
		Bounds: BoundsProps{X: 10, Y: 20, W: 40, H: 20},
		Style:  PrimitiveStyleProps{Visible: true, Opacity: 1},
	}
	anchors := ellipse.ExportAnchors(layout.AnchorExportContext{})
	want := map[layout.AnchorID]gfx.Point{
		"center": {X: 30, Y: 30},
		"north":  {X: 30, Y: 20},
		"east":   {X: 50, Y: 30},
		"south":  {X: 30, Y: 40},
		"west":   {X: 10, Y: 30},
	}
	for id, pt := range want {
		got, ok := anchors[id]
		if !ok {
			t.Fatalf("missing anchor %q", id)
		}
		if got != pt {
			t.Fatalf("anchor %q = %+v want %+v", id, got, pt)
		}
	}
}

func TestEllipse_projects_path_commands(t *testing.T) {
	ellipse := &Ellipse{
		Bounds: BoundsProps{X: 0, Y: 0, W: 20, H: 10},
		Style: PrimitiveStyleProps{
			Fill: theme.Material{
				Fills:   []theme.Fill{{Type: theme.FillSolid, Color: gfx.Color{G: 1, A: 1}, Opacity: 1}},
				Opacity: 1,
			},
			Visible: true,
			Opacity: 1,
		},
	}
	cmds := renderMark(t, ellipse)
	if !containsCommandType(cmds, gfx.FillPath{}) {
		t.Fatalf("expected fill path command, got %#v", cmds)
	}
}
