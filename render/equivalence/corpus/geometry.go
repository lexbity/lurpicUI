package corpus

import (
	"math"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/equivalence"
)

func geometryFixtures() []equivalence.FrameFixture {
	red := gfx.ColorFromRGBA8(230, 60, 60, 255)
	green := gfx.ColorFromRGBA8(60, 200, 90, 255)
	blue := gfx.ColorFromRGBA8(70, 120, 230, 255)

	return []equivalence.FrameFixture{
		fixture{
			name: "solid_rect_axis_aligned", width: 64, height: 64,
			frame: func() *render.Frame {
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.FillRect{Rect: gfx.RectFromXYWH(8, 8, 32, 32), Brush: gfx.SolidBrush(red)},
				)
			},
		},
		fixture{
			name: "solid_rect_batch_transform_wrapper", width: 64, height: 64,
			frame: func() *render.Frame {
				// The runtime wraps a non-identity batch transform in
				// PushTransform/PopTransform; the encoder must lift it into the
				// batch header without changing the rendered result.
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.PushTransform{Matrix: gfx.Translation(10, 12)},
					gfx.FillRect{Rect: gfx.RectFromXYWH(4, 4, 20, 20), Brush: gfx.SolidBrush(red)},
					gfx.PopTransform{},
				)
			},
		},
		fixture{
			name: "solid_rect_rotated_45", width: 64, height: 64,
			frame: func() *render.Frame {
				// A 45-degree rotation produces diagonal edges; the GPU renders
				// them with MSAA (Q8), the software oracle with analytic AA.
				// (Feature-specific tolerance calibrated in the baseline notes.)
				rot := gfx.Rotation(45 * math.Pi / 180)
				// Rotate around the rect center, then center it in the canvas.
				center := gfx.Point{X: 32, Y: 32}
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.PushTransform{Matrix: gfx.Translation(center.X, center.Y).Multiply(rot).Multiply(gfx.Translation(-20, -20))},
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 24, 24), Brush: gfx.SolidBrush(red)},
					gfx.PopTransform{},
				)
			},
		},
		fixture{
			name: "solid_rect_scaled", width: 64, height: 64,
			frame: func() *render.Frame {
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.PushTransform{Matrix: gfx.Translation(4, 4).Multiply(gfx.Scale(2, 1.5))},
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 16, 16), Brush: gfx.SolidBrush(blue)},
					gfx.PopTransform{},
				)
			},
		},
		fixture{
			name: "many_rects_instanced", width: 128, height: 128,
			frame: func() *render.Frame {
				var cmds []gfx.Command
				for i := 0; i < 1200; i++ {
					x := float32((i % 40) * 3)
					y := float32((i / 40) * 3)
					cmds = append(cmds, gfx.FillRect{
						Rect:  gfx.RectFromXYWH(x, y, 2, 2),
						Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(uint8(40+((i*7)%200)), uint8((i*13)%255), uint8((i*29)%255), 255)),
					})
				}
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 128, 128), 1, cmds...)
			},
		},
		fixture{
			name: "solid_rect_opacity", width: 64, height: 64,
			frame: func() *render.Frame {
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 0.5,
					gfx.FillRect{Rect: gfx.RectFromXYWH(8, 8, 32, 32), Brush: gfx.SolidBrush(red)},
				)
			},
		},
		fixture{
			name: "two_rects_overlap", width: 64, height: 64,
			frame: func() *render.Frame {
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.FillRect{Rect: gfx.RectFromXYWH(8, 8, 32, 32), Brush: gfx.SolidBrush(red)},
					gfx.FillRect{Rect: gfx.RectFromXYWH(24, 24, 32, 32), Brush: gfx.SolidBrush(blue)},
				)
			},
		},
		fixture{
			name: "nested_clip", width: 64, height: 64,
			frame: func() *render.Frame {
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.PushClipRect{Rect: gfx.RectFromXYWH(8, 8, 24, 24)},
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 64, 64), Brush: gfx.SolidBrush(red)},
					gfx.PopClip{},
				)
			},
		},
		fixture{
			name: "opacity_stack", width: 64, height: 64,
			frame: func() *render.Frame {
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.PushOpacity{Alpha: 0.5},
					gfx.PushOpacity{Alpha: 0.5},
					gfx.FillRect{Rect: gfx.RectFromXYWH(8, 8, 32, 32), Brush: gfx.SolidBrush(red)},
					gfx.PopOpacity{},
					gfx.PopOpacity{},
				)
			},
		},
		fixture{
			name: "nested_transform_clip", width: 64, height: 64,
			frame: func() *render.Frame {
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.PushTransform{Matrix: gfx.Translation(4, 4)},
					gfx.PushClipRect{Rect: gfx.RectFromXYWH(0, 0, 24, 24)},
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 40, 40), Brush: gfx.SolidBrush(green)},
					gfx.PopClip{},
					gfx.PopTransform{},
				)
			},
		},
		fixture{
			name: "layer_clip", width: 64, height: 64,
			frame: func() *render.Frame {
				b := render.RenderBatch{
					ID:      1,
					Bounds:  gfx.RectFromXYWH(0, 0, 64, 64),
					Opacity: 1,
					Commands: gfx.CommandList{Commands: []gfx.Command{
						gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 64, 64), Brush: gfx.SolidBrush(blue)},
					}},
				}
				return &render.Frame{
					RenderBatchs: []render.RenderBatch{b},
					FramePacket: render.FramePacket{Layers: []render.LayeredBatch{
						{RenderOrder: 0, ClipRect: gfx.RectFromXYWH(8, 8, 32, 32), Batches: []render.RenderBatch{b}},
					}},
				}
			},
		},
		fixture{
			name: "fill_path_rect", width: 64, height: 64,
			frame: func() *render.Frame {
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.FillPath{Path: gfx.RectPath(gfx.RectFromXYWH(8, 8, 32, 32)), Brush: gfx.SolidBrush(green)},
				)
			},
		},
		fixture{
			name: "stroke_rect_axis_aligned", width: 64, height: 64,
			frame: func() *render.Frame {
				// Even width so stroke edges land on integer coordinates (the
				// stepping-stone raster and the oracle agree on coverage).
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.StrokeRect{Rect: gfx.RectFromXYWH(10, 10, 40, 40), Stroke: gfx.DefaultStroke(4), Brush: gfx.SolidBrush(red)},
				)
			},
		},
		fixture{
			name: "stroke_path_rect_deferred", width: 64, height: 64,
			frame: func() *render.Frame {
				// General closed-path strokes need OffsetContour expansion with
				// hole-aware filling (Slice 8). Kept in the corpus so the
				// command is covered at the wire level; skipped by the corpus
				// runner until the GPU stroke pipeline lands.
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.StrokePath{Path: gfx.RectPath(gfx.RectFromXYWH(10, 10, 40, 40)), Stroke: gfx.DefaultStroke(2), Brush: gfx.SolidBrush(blue)},
				)
			},
		},
		fixture{
			name: "polyline_open", width: 64, height: 64,
			frame: func() *render.Frame {
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.DrawPolyline{
						Points: []gfx.Point{{X: 8, Y: 8}, {X: 40, Y: 8}, {X: 40, Y: 40}},
						Stroke: gfx.DefaultStroke(2),
						Brush:  gfx.SolidBrush(green),
					},
				)
			},
		},
		fixture{
			name: "polyline_closed", width: 64, height: 64,
			frame: func() *render.Frame {
				// Axis-aligned closed outline: both backends rasterize strokes as
				// per-segment quads, which agree for integer-width segments.
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.DrawPolyline{
						Points: []gfx.Point{{X: 10, Y: 10}, {X: 50, Y: 10}, {X: 50, Y: 50}, {X: 10, Y: 50}},
						Stroke: gfx.DefaultStroke(2),
						Brush:  gfx.SolidBrush(blue),
						Closed: true,
					},
				)
			},
		},
		fixture{
			name: "points_grid", width: 64, height: 64,
			frame: func() *render.Frame {
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.DrawPoints{
						Points: []gfx.Point{{X: 12, Y: 12}, {X: 32, Y: 12}, {X: 12, Y: 32}, {X: 32, Y: 32}},
						Radius: 4,
						Brush:  gfx.SolidBrush(red),
					},
				)
			},
		},
		fixture{
			name: "selection_rects", width: 64, height: 64,
			frame: func() *render.Frame {
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.DrawSelectionRects{
						Rects: []gfx.Rect{gfx.RectFromXYWH(10, 10, 20, 6), gfx.RectFromXYWH(10, 18, 20, 6)},
						Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(64, 128, 255, 96)),
					},
				)
			},
		},
		fixture{
			name: "gradient_2stop_horizontal", width: 64, height: 64,
			frame: func() *render.Frame {
				brush := gfx.LinearGradientBrush(
					gfx.Point{X: 0, Y: 0},
					gfx.Point{X: 64, Y: 0},
					[]gfx.GradientStop{
						{Offset: 0, Color: red},
						{Offset: 1, Color: blue},
					},
				)
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.FillRect{Rect: gfx.RectFromXYWH(4, 4, 56, 56), Brush: brush},
				)
			},
		},
		fixture{
			name: "gradient_5stop_diagonal", width: 64, height: 64,
			frame: func() *render.Frame {
				brush := gfx.LinearGradientBrush(
					gfx.Point{X: 0, Y: 0},
					gfx.Point{X: 64, Y: 64},
					[]gfx.GradientStop{
						{Offset: 0, Color: red},
						{Offset: 0.25, Color: gfx.ColorFromRGBA8(240, 200, 60, 255)},
						{Offset: 0.5, Color: green},
						{Offset: 0.75, Color: gfx.ColorFromRGBA8(80, 180, 220, 255)},
						{Offset: 1, Color: blue},
					},
				)
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.FillPath{Path: gfx.RectPath(gfx.RectFromXYWH(4, 4, 56, 56)), Brush: brush},
				)
			},
		},
		fixture{
			name: "many_rects", width: 64, height: 64,
			frame: func() *render.Frame {
				var cmds []gfx.Command
				for i := 0; i < 12; i++ {
					x := float32((i%4)*14 + 2)
					y := float32((i/4)*14 + 2)
					cmds = append(cmds, gfx.FillRect{
						Rect:  gfx.RectFromXYWH(x, y, 10, 10),
						Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(uint8(60+16*i), 90, 140, 255)),
					})
				}
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1, cmds...)
			},
		},
	}
}
