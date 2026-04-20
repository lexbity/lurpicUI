package hashutil

import (
	"image"
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
)

func TestHashutil_same_commandlist_same_hash(t *testing.T) {
	cl1 := gfx.CommandList{Commands: []gfx.Command{
		gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
	}}
	cl2 := gfx.CommandList{Commands: []gfx.Command{
		gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
	}}
	if got1, got2 := HashCommandList(cl1), HashCommandList(cl2); got1 != got2 {
		t.Fatalf("%d != %d", got1, got2)
	}
}

func TestHashutil_different_commands_different_hash(t *testing.T) {
	cl1 := gfx.CommandList{Commands: []gfx.Command{
		gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
	}}
	cl2 := gfx.CommandList{Commands: []gfx.Command{
		gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 20, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
	}}
	if HashCommandList(cl1) == HashCommandList(cl2) {
		t.Fatal("expected different hashes")
	}
}

func TestHashutil_order_matters(t *testing.T) {
	cl1 := gfx.CommandList{Commands: []gfx.Command{
		gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
		gfx.FillRect{Rect: gfx.RectFromXYWH(10, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})},
	}}
	cl2 := gfx.CommandList{Commands: []gfx.Command{
		gfx.FillRect{Rect: gfx.RectFromXYWH(10, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})},
		gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
	}}
	if HashCommandList(cl1) == HashCommandList(cl2) {
		t.Fatal("expected different hashes")
	}
}

func TestHashutil_image_content_affects_hash(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	cl1 := gfx.CommandList{Commands: []gfx.Command{gfx.DrawImage{Image: img}}}
	img2 := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img2.SetRGBA(0, 0, color.RGBA{G: 255, A: 255})
	cl2 := gfx.CommandList{Commands: []gfx.Command{gfx.DrawImage{Image: img2}}}
	if HashCommandList(cl1) == HashCommandList(cl2) {
		t.Fatal("expected different hashes")
	}
}

func TestHashutil_builder_methods_are_stable(t *testing.T) {
	a := NewCacheKeyBuilder()
	a.WriteUint8(1)
	a.WriteUint32(2)
	a.WriteUint64(3)
	a.WriteFloat32(4.5)
	a.WriteString("abc")
	a.WriteBytes([]byte{4, 5, 6})

	b := NewCacheKeyBuilder()
	b.WriteUint8(1)
	b.WriteUint32(2)
	b.WriteUint64(3)
	b.WriteFloat32(4.5)
	b.WriteString("abc")
	b.WriteBytes([]byte{4, 5, 6})

	if gotA, gotB := a.Sum(), b.Sum(); gotA != gotB {
		t.Fatalf("stable builder mismatch: %d != %d", gotA, gotB)
	}

	c := NewCacheKeyBuilder()
	c.WriteBytes([]byte{4, 5, 6})
	c.WriteString("abc")
	c.WriteFloat32(4.5)
	c.WriteUint64(3)
	c.WriteUint32(2)
	c.WriteUint8(1)
	if a.Sum() == c.Sum() {
		t.Fatal("expected order to affect hash")
	}
}

func TestHashutil_command_variants_cover_sensitive_fields(t *testing.T) {
	glyphRun := text.GlyphRun{
		Face: text.FontFace{},
		Glyphs: []text.PositionedGlyph{{
			GlyphID: 1,
			X:       1,
			Y:       2,
		}},
	}
	img1 := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img1.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	img2 := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img2.SetRGBA(0, 0, color.RGBA{G: 255, A: 255})

	base := gfx.CommandList{Commands: []gfx.Command{
		gfx.PushTransform{Matrix: gfx.Translation(1, 2)},
		gfx.PushClipRect{Rect: gfx.RectFromXYWH(0, 0, 4, 4)},
		gfx.PushOpacity{Alpha: 0.5},
		gfx.DrawGlyphRun{Run: glyphRun, Origin: gfx.Point{X: 3, Y: 4}, Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
		gfx.DrawImage{
			Image:    img1,
			DestRect: gfx.RectFromXYWH(1, 1, 2, 2),
			SrcRect:  gfx.RectFromXYWH(0, 0, 1, 1),
			Sampling: gfx.SamplingBilinear,
			Opacity:  0.75,
		},
		gfx.PopOpacity{},
		gfx.PopClip{},
		gfx.BeginRenderBatch{Bounds: gfx.RectFromXYWH(0, 0, 10, 10), CacheID: 7},
		gfx.EndRenderBatch{},
	}}
	if got := HashCommandList(base); got == 0 {
		t.Fatal("expected hash")
	}

	same := gfx.CommandList{Commands: append([]gfx.Command(nil), base.Commands...)}
	if HashCommandList(base) != HashCommandList(same) {
		t.Fatal("expected identical lists to hash equally")
	}

	if HashCommandList(gfx.CommandList{Commands: []gfx.Command{gfx.PushOpacity{Alpha: 0.5}}}) ==
		HashCommandList(gfx.CommandList{Commands: []gfx.Command{gfx.PushOpacity{Alpha: 0.75}}}) {
		t.Fatal("expected opacity to affect hash")
	}

	if HashCommandList(gfx.CommandList{Commands: []gfx.Command{gfx.DrawGlyphRun{Run: glyphRun, Origin: gfx.Point{X: 3, Y: 4}, Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})}}}) ==
		HashCommandList(gfx.CommandList{Commands: []gfx.Command{gfx.DrawGlyphRun{Run: glyphRun, Origin: gfx.Point{X: 5, Y: 4}, Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})}}}) {
		t.Fatal("expected glyph origin to affect hash")
	}

	if HashCommandList(gfx.CommandList{Commands: []gfx.Command{gfx.DrawGlyphRun{Run: glyphRun, Origin: gfx.Point{X: 3, Y: 4}, Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})}}}) ==
		HashCommandList(gfx.CommandList{Commands: []gfx.Command{gfx.DrawGlyphRun{Run: glyphRun, Origin: gfx.Point{X: 3, Y: 4}, Brush: gfx.SolidBrush(gfx.Color{B: 1, A: 1})}}}) {
		t.Fatal("expected glyph brush to affect hash")
	}

	if HashCommandList(gfx.CommandList{Commands: []gfx.Command{gfx.DrawImage{Image: img1, DestRect: gfx.RectFromXYWH(1, 1, 2, 2), SrcRect: gfx.RectFromXYWH(0, 0, 1, 1), Sampling: gfx.SamplingBilinear, Opacity: 0.75}}}) ==
		HashCommandList(gfx.CommandList{Commands: []gfx.Command{gfx.DrawImage{Image: img2, DestRect: gfx.RectFromXYWH(1, 1, 2, 2), SrcRect: gfx.RectFromXYWH(0, 0, 1, 1), Sampling: gfx.SamplingBilinear, Opacity: 0.75}}}) {
		t.Fatal("expected image pixels to affect hash")
	}

	if HashCommandList(gfx.CommandList{Commands: []gfx.Command{gfx.DrawImage{Image: img1, DestRect: gfx.RectFromXYWH(1, 1, 2, 2), SrcRect: gfx.RectFromXYWH(0, 0, 1, 1), Sampling: gfx.SamplingNearest, Opacity: 0.75}}}) ==
		HashCommandList(gfx.CommandList{Commands: []gfx.Command{gfx.DrawImage{Image: img1, DestRect: gfx.RectFromXYWH(2, 1, 2, 2), SrcRect: gfx.RectFromXYWH(0, 0, 1, 1), Sampling: gfx.SamplingNearest, Opacity: 0.75}}}) {
		t.Fatal("expected destination rect to affect hash")
	}
}

func TestHashutil_all_command_helpers_are_covered(t *testing.T) {
	path := gfx.NewPath().
		MoveTo(gfx.Point{X: 1, Y: 2}).
		LineTo(gfx.Point{X: 3, Y: 4}).
		QuadTo(gfx.Point{X: 5, Y: 6}, gfx.Point{X: 7, Y: 8}).
		CubicTo(gfx.Point{X: 9, Y: 10}, gfx.Point{X: 11, Y: 12}, gfx.Point{X: 13, Y: 14}).
		Close().
		Build()
	gradient := gfx.LinearGradientBrush(
		gfx.Point{X: 0, Y: 0},
		gfx.Point{X: 10, Y: 10},
		[]gfx.GradientStop{
			{Offset: 0, Color: gfx.Color{R: 1, A: 1}},
			{Offset: 1, Color: gfx.Color{B: 1, A: 1}},
		},
	)
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})

	cl := gfx.CommandList{Commands: []gfx.Command{
		gfx.PushTransform{Matrix: gfx.Translation(2, 3)},
		gfx.PushClipRect{Rect: gfx.RectFromXYWH(0, 0, 20, 20)},
		gfx.PushOpacity{Alpha: 0.5},
		gfx.FillRect{Rect: gfx.RectFromXYWH(1, 1, 4, 4), Brush: gradient},
		gfx.StrokeRect{Rect: gfx.RectFromXYWH(2, 2, 4, 4), Stroke: gfx.StrokeStyle{Width: 2, Cap: gfx.LineCapRound, Join: gfx.LineJoinBevel, MiterLimit: 7, Dash: []float32{1, 2}, DashOffset: 3}, Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})},
		gfx.FillPath{Path: path, Brush: gfx.SolidBrush(gfx.Color{B: 1, A: 1})},
		gfx.StrokePath{Path: path, Stroke: gfx.StrokeStyle{Width: 3, Cap: gfx.LineCapSquare, Join: gfx.LineJoinRound, MiterLimit: 5, Dash: []float32{2, 4}, DashOffset: 1}, Brush: gfx.SolidBrush(gfx.Color{R: 1, G: 1, A: 1})},
		gfx.DrawPolyline{Points: []gfx.Point{{1, 1}, {2, 3}, {4, 5}}, Stroke: gfx.StrokeStyle{Width: 1, Cap: gfx.LineCapButt, Join: gfx.LineJoinMiter, MiterLimit: 10, Dash: []float32{1}, DashOffset: 0.5}, Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1}), Closed: true},
		gfx.DrawPoints{Points: []gfx.Point{{6, 7}, {8, 9}}, Radius: 2, Brush: gradient},
		gfx.DrawGlyphRun{Run: text.GlyphRun{}, Origin: gfx.Point{X: 9, Y: 10}, Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
		gfx.DrawSelectionRects{Rects: []gfx.Rect{gfx.RectFromXYWH(0, 0, 1, 1), gfx.RectFromXYWH(1, 1, 2, 2)}, Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})},
		gfx.DrawImage{Image: nil, DestRect: gfx.RectFromXYWH(0, 0, 3, 3), SrcRect: gfx.RectFromXYWH(0, 0, 1, 1), Sampling: gfx.SamplingNearest, Opacity: 0.25},
		gfx.DrawImage{Image: img, DestRect: gfx.RectFromXYWH(1, 1, 3, 3), SrcRect: gfx.RectFromXYWH(0, 0, 1, 1), Sampling: gfx.SamplingBilinear, Opacity: 0.75},
		gfx.PopOpacity{},
		gfx.PopClip{},
		gfx.BeginRenderBatch{Bounds: gfx.RectFromXYWH(0, 0, 10, 10), CacheID: 7},
		gfx.EndRenderBatch{},
	}}
	got := HashCommandList(cl)
	if got == 0 {
		t.Fatal("expected non-zero hash")
	}
	if got != HashCommandList(cl) {
		t.Fatal("expected stable hash")
	}
}
