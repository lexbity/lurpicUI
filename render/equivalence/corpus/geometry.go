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
			name: "path_convex", width: 64, height: 64,
			frame: func() *render.Frame {
				// A convex pentagon.
				path := gfx.NewPath().
					MoveTo(gfx.Point{X: 32, Y: 8}).
					LineTo(gfx.Point{X: 52, Y: 24}).
					LineTo(gfx.Point{X: 44, Y: 48}).
					LineTo(gfx.Point{X: 20, Y: 48}).
					LineTo(gfx.Point{X: 12, Y: 24}).
					Close().Build()
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.FillPath{Path: path, Brush: gfx.SolidBrush(red)},
				)
			},
		},
		fixture{
			name: "path_concave", width: 64, height: 64,
			frame: func() *render.Frame {
				// A concave star-like shape (a notch carved into a square).
				path := gfx.NewPath().
					MoveTo(gfx.Point{X: 10, Y: 10}).
					LineTo(gfx.Point{X: 50, Y: 10}).
					LineTo(gfx.Point{X: 50, Y: 50}).
					LineTo(gfx.Point{X: 40, Y: 50}).
					LineTo(gfx.Point{X: 40, Y: 30}).
					LineTo(gfx.Point{X: 20, Y: 30}).
					LineTo(gfx.Point{X: 20, Y: 50}).
					LineTo(gfx.Point{X: 10, Y: 50}).
					Close().Build()
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.FillPath{Path: path, Brush: gfx.SolidBrush(blue)},
				)
			},
		},
		fixture{
			name: "path_self_intersecting", width: 64, height: 64,
			frame: func() *render.Frame {
				// A figure-eight (bowtie): the lobes have opposite winding; the
				// nonzero rule fills both.
				path := gfx.NewPath().
					MoveTo(gfx.Point{X: 14, Y: 16}).
					LineTo(gfx.Point{X: 50, Y: 16}).
					LineTo(gfx.Point{X: 14, Y: 48}).
					LineTo(gfx.Point{X: 50, Y: 48}).
					Close().Build()
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.FillPath{Path: path, Brush: gfx.SolidBrush(green)},
				)
			},
		},
		fixture{
			name: "path_with_hole", width: 64, height: 64,
			frame: func() *render.Frame {
				// An outer square with an inner hole (opposite winding).
				path := gfx.NewPath().
					MoveTo(gfx.Point{X: 8, Y: 8}).
					LineTo(gfx.Point{X: 56, Y: 8}).
					LineTo(gfx.Point{X: 56, Y: 56}).
					LineTo(gfx.Point{X: 8, Y: 56}).
					Close().
					MoveTo(gfx.Point{X: 24, Y: 24}).
					LineTo(gfx.Point{X: 24, Y: 40}).
					LineTo(gfx.Point{X: 40, Y: 40}).
					LineTo(gfx.Point{X: 40, Y: 24}).
					Close().Build()
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.FillPath{Path: path, Brush: gfx.SolidBrush(red)},
				)
			},
		},
		fixture{
			name: "path_quadratic_curve", width: 64, height: 64,
			frame: func() *render.Frame {
				path := gfx.NewPath().
					MoveTo(gfx.Point{X: 10, Y: 50}).
					QuadTo(gfx.Point{X: 16, Y: 14}, gfx.Point{X: 32, Y: 14}).
					QuadTo(gfx.Point{X: 48, Y: 14}, gfx.Point{X: 54, Y: 50}).
					LineTo(gfx.Point{X: 10, Y: 50}).
					Close().Build()
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.FillPath{Path: path, Brush: gfx.SolidBrush(green)},
				)
			},
		},
		fixture{
			name: "path_cubic_curve", width: 64, height: 64,
			frame: func() *render.Frame {
				path := gfx.NewPath().
					MoveTo(gfx.Point{X: 8, Y: 40}).
					CubicTo(gfx.Point{X: 16, Y: 8}, gfx.Point{X: 48, Y: 8}, gfx.Point{X: 56, Y: 40}).
					LineTo(gfx.Point{X: 8, Y: 40}).
					Close().Build()
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.FillPath{Path: path, Brush: gfx.SolidBrush(blue)},
				)
			},
		},
		fixture{
			name: "path_many_segments", width: 128, height: 128,
			frame: func() *render.Frame {
				var path gfx.Path
				for i := 0; i < 200; i++ {
					seg := gfx.RectPath(gfx.RectFromXYWH(
						float32((i%10)*12)+4, float32((i/10)*12)+4, 8, 8,
					))
					path.Segments = append(path.Segments, seg.Segments...)
				}
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 128, 128), 1,
					gfx.FillPath{Path: path, Brush: gfx.SolidBrush(red)},
				)
			},
		},
		fixture{
			name: "chart_area_fill", width: 128, height: 96,
			frame: func() *render.Frame {
				// A chart area: a baseline with a sawtooth top edge.
				path := gfx.NewPath().
					MoveTo(gfx.Point{X: 8, Y: 88}).
					LineTo(gfx.Point{X: 8, Y: 60}).
					LineTo(gfx.Point{X: 24, Y: 40}).
					LineTo(gfx.Point{X: 40, Y: 70}).
					LineTo(gfx.Point{X: 56, Y: 32}).
					LineTo(gfx.Point{X: 72, Y: 64}).
					LineTo(gfx.Point{X: 88, Y: 28}).
					LineTo(gfx.Point{X: 104, Y: 52}).
					LineTo(gfx.Point{X: 120, Y: 20}).
					LineTo(gfx.Point{X: 120, Y: 88}).
					Close().Build()
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 128, 96), 1,
					gfx.FillPath{Path: path, Brush: gfx.SolidBrush(green)},
				)
			},
		},
		fixture{
			// Wire-level only: DrawBlurredShadow is not rendered by the GPU
			// pipeline until Slice 9. It must round-trip so the corpus covers
			// every v2 opcode.
			name: "blurred_shadow_rect", width: 64, height: 64,
			frame: func() *render.Frame {
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.DrawBlurredShadow{
						Path:       gfx.RectPath(gfx.RectFromXYWH(16, 16, 24, 24)),
						Color:      gfx.ColorFromRGBA8(10, 10, 30, 160),
						BlurRadius: 4,
						Offset:     gfx.Point{X: 3, Y: 4},
						Inner:      false,
					},
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
			name: "stroke_path_rect", width: 64, height: 64,
			frame: func() *render.Frame {
				// General closed-path strokes expand via OffsetContour (Slice 8);
				// the ring between the outer and inner offset contours fills.
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.StrokePath{Path: gfx.RectPath(gfx.RectFromXYWH(10, 10, 40, 40)), Stroke: gfx.DefaultStroke(2), Brush: gfx.SolidBrush(blue)},
				)
			},
		},
		fixture{
			name: "stroke_butt_cap", width: 64, height: 64,
			frame: func() *render.Frame {
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.StrokePath{Path: gfx.LinePath(gfx.Point{X: 8, Y: 40}, gfx.Point{X: 48, Y: 12}), Stroke: gfx.DefaultStroke(6), Brush: gfx.SolidBrush(red)},
				)
			},
		},
		fixture{
			name: "stroke_round_cap", width: 64, height: 64,
			frame: func() *render.Frame {
				stroke := gfx.DefaultStroke(6)
				stroke.Cap = gfx.LineCapRound
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.StrokePath{Path: gfx.LinePath(gfx.Point{X: 8, Y: 40}, gfx.Point{X: 48, Y: 12}), Stroke: stroke, Brush: gfx.SolidBrush(red)},
				)
			},
		},
		fixture{
			name: "stroke_square_cap", width: 64, height: 64,
			frame: func() *render.Frame {
				stroke := gfx.DefaultStroke(6)
				stroke.Cap = gfx.LineCapSquare
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.StrokePath{Path: gfx.LinePath(gfx.Point{X: 8, Y: 40}, gfx.Point{X: 48, Y: 12}), Stroke: stroke, Brush: gfx.SolidBrush(red)},
				)
			},
		},
		fixture{
			name: "stroke_miter_join", width: 64, height: 64,
			frame: func() *render.Frame {
				path := gfx.NewPath().
					MoveTo(gfx.Point{X: 8, Y: 8}).
					LineTo(gfx.Point{X: 48, Y: 8}).
					LineTo(gfx.Point{X: 48, Y: 48}).
					Build()
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.StrokePath{Path: path, Stroke: gfx.DefaultStroke(6), Brush: gfx.SolidBrush(red)},
				)
			},
		},
		fixture{
			name: "stroke_round_join", width: 64, height: 64,
			frame: func() *render.Frame {
				stroke := gfx.DefaultStroke(6)
				stroke.Join = gfx.LineJoinRound
				path := gfx.NewPath().
					MoveTo(gfx.Point{X: 8, Y: 8}).
					LineTo(gfx.Point{X: 48, Y: 8}).
					LineTo(gfx.Point{X: 48, Y: 48}).
					Build()
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.StrokePath{Path: path, Stroke: stroke, Brush: gfx.SolidBrush(red)},
				)
			},
		},
		fixture{
			name: "stroke_bevel_join", width: 64, height: 64,
			frame: func() *render.Frame {
				stroke := gfx.DefaultStroke(6)
				stroke.Join = gfx.LineJoinBevel
				path := gfx.NewPath().
					MoveTo(gfx.Point{X: 8, Y: 8}).
					LineTo(gfx.Point{X: 48, Y: 8}).
					LineTo(gfx.Point{X: 48, Y: 48}).
					Build()
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.StrokePath{Path: path, Stroke: stroke, Brush: gfx.SolidBrush(red)},
				)
			},
		},
		fixture{
			name: "stroke_miter_limit_clip", width: 64, height: 64,
			frame: func() *render.Frame {
				// A narrow V: the miter at the point extends ~3.3x the half-width
				// beyond the vertex, far over the limit, so the join must fall
				// back to a bevel.
				stroke := gfx.DefaultStroke(6)
				stroke.MiterLimit = 2
				path := gfx.NewPath().
					MoveTo(gfx.Point{X: 20, Y: 48}).
					LineTo(gfx.Point{X: 32, Y: 10}).
					LineTo(gfx.Point{X: 44, Y: 48}).
					Build()
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.StrokePath{Path: path, Stroke: stroke, Brush: gfx.SolidBrush(blue)},
				)
			},
		},
		fixture{
			name: "stroke_dashed", width: 64, height: 64,
			frame: func() *render.Frame {
				stroke := gfx.DefaultStroke(4)
				stroke.Dash = []float32{8, 6}
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.StrokePath{Path: gfx.LinePath(gfx.Point{X: 6, Y: 32}, gfx.Point{X: 58, Y: 32}), Stroke: stroke, Brush: gfx.SolidBrush(green)},
				)
			},
		},
		fixture{
			name: "chart_line_stroke", width: 96, height: 64,
			frame: func() *render.Frame {
				// The shape marks/viz/line.go emits: a DrawPolyline with a
				// plain width-only stroke.
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 96, 64), 1,
					gfx.DrawPolyline{
						Points: []gfx.Point{{X: 8, Y: 48}, {X: 24, Y: 30}, {X: 40, Y: 42}, {X: 56, Y: 18}, {X: 72, Y: 36}, {X: 88, Y: 12}},
						Stroke: gfx.StrokeStyle{Width: 2},
						Brush:  gfx.SolidBrush(green),
					},
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
			name: "gradient_5stop_rect", width: 64, height: 64,
			frame: func() *render.Frame {
				brush := gfx.LinearGradientBrush(
					gfx.Point{X: 0, Y: 0},
					gfx.Point{X: 64, Y: 0},
					[]gfx.GradientStop{
						{Offset: 0, Color: red},
						{Offset: 0.25, Color: gfx.ColorFromRGBA8(240, 200, 60, 255)},
						{Offset: 0.5, Color: green},
						{Offset: 0.75, Color: gfx.ColorFromRGBA8(80, 180, 220, 255)},
						{Offset: 1, Color: blue},
					},
				)
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.FillRect{Rect: gfx.RectFromXYWH(4, 4, 56, 56), Brush: brush},
				)
			},
		},
		fixture{
			name: "gradient_rotated", width: 64, height: 64,
			frame: func() *render.Frame {
				brush := gfx.LinearGradientBrush(
					gfx.Point{X: 0, Y: 32},
					gfx.Point{X: 64, Y: 32},
					[]gfx.GradientStop{
						{Offset: 0, Color: red},
						{Offset: 1, Color: blue},
					},
				)
				rot := gfx.Rotation(45 * math.Pi / 180)
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.PushTransform{Matrix: gfx.Translation(32, 32).Multiply(rot).Multiply(gfx.Translation(-20, -20))},
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 24, 24), Brush: brush},
					gfx.PopTransform{},
				)
			},
		},
		fixture{
			name: "gradient_stroke_rect", width: 64, height: 64,
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
					gfx.StrokeRect{Rect: gfx.RectFromXYWH(10, 10, 40, 40), Stroke: gfx.DefaultStroke(4), Brush: brush},
				)
			},
		},
		fixture{
			// A gradient on a FillPath: the path fill pipeline (stencil, Slice 7)
			// is required, so this is deferred with the other path fixtures.
			name: "gradient_in_path", width: 64, height: 64,
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
