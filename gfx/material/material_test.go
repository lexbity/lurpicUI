package material

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestCommandsLinearGradientAndStrokeOpacity(t *testing.T) {
	path := gfx.RectPath(gfx.RectFromXYWH(0, 0, 10, 10))
	m := theme.Material{
		Opacity: 0.5,
		Fills: []theme.Fill{
			{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(255, 0, 0, 255), Opacity: 0.5},
			{
				Type: theme.FillGradient,
				Gradient: theme.Gradient{
					Type:  theme.GradientLinear,
					Start: gfx.Point{X: 0, Y: 0},
					End:   gfx.Point{X: 10, Y: 0},
					Stops: []theme.GradientStop{
						{Position: 0, Color: gfx.ColorFromRGBA8(0, 0, 0, 255)},
						{Position: 1, Color: gfx.ColorFromRGBA8(255, 255, 255, 255)},
					},
				},
				Opacity: 0.8,
			},
		},
		Strokes: []theme.MaterialStroke{
			{
				Paint: theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(0, 0, 255, 255), Opacity: 0.25},
				Width: 2, Cap: theme.CapSquare, Join: theme.JoinBevel, Dash: []float32{1, 2}, DashOffset: 3,
			},
		},
	}

	cmds := Commands(path, m)
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}
	fill := cmds[0].(gfx.FillPath)
	if fill.Brush.Kind != gfx.BrushSolid {
		t.Fatalf("expected solid brush for first fill, got %v", fill.Brush.Kind)
	}
	if fill.Brush.Color.A != 0.25 {
		t.Fatalf("expected fill alpha 0.25, got %v", fill.Brush.Color.A)
	}
	gradient := cmds[1].(gfx.FillPath)
	if gradient.Brush.Kind != gfx.BrushLinearGradient {
		t.Fatalf("expected gradient brush, got %v", gradient.Brush.Kind)
	}
	if len(gradient.Brush.GradientStops) != 2 {
		t.Fatalf("expected 2 gradient stops, got %d", len(gradient.Brush.GradientStops))
	}
	if !almostEqual(gradient.Brush.GradientStops[0].Color.A, 0.4) {
		t.Fatalf("expected first gradient stop alpha 0.4, got %v", gradient.Brush.GradientStops[0].Color.A)
	}
	stroke := cmds[2].(gfx.StrokePath)
	if stroke.Stroke.Cap != gfx.LineCapSquare || stroke.Stroke.Join != gfx.LineJoinBevel {
		t.Fatalf("unexpected stroke style: %+v", stroke.Stroke)
	}
	if stroke.Brush.Color.A != 0.125 {
		t.Fatalf("expected stroke alpha 0.125, got %v", stroke.Brush.Color.A)
	}
	if len(stroke.Stroke.Dash) != 2 || stroke.Stroke.DashOffset != 3 {
		t.Fatalf("unexpected stroke dash settings: %+v", stroke.Stroke)
	}
}

func almostEqual(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-6 }

func TestColorAndTransparent(t *testing.T) {
	visible := theme.Material{
		Opacity: 0.5,
		Fills:   []theme.Fill{{Type: theme.FillNone, Opacity: 1}, {Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(10, 20, 30, 255), Opacity: 0.5}},
	}
	if theme.Transparent(visible) {
		t.Fatal("expected material to be visible")
	}
	if c := theme.Color(visible); c.A != 0.25 {
		t.Fatalf("expected color alpha 0.25, got %v", c.A)
	}
	if !theme.Transparent(theme.Material{
		Opacity: 1,
		Fills: []theme.Fill{{
			Type:     theme.FillGradient,
			Gradient: theme.Gradient{Type: theme.GradientRadial, Stops: []theme.GradientStop{{Position: 0, Color: gfx.ColorFromRGBA8(255, 0, 0, 255)}}},
			Opacity:  1,
		}},
	}) {
		t.Fatal("expected unsupported gradient material to be transparent")
	}
	if got := theme.Color(theme.Material{}); got != (gfx.Color{}) {
		t.Fatalf("expected zero color, got %+v", got)
	}
}
