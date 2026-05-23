package cook

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"golang.org/x/image/vector"

	"codeburg.org/lexbit/lurpicui/gfx"
	gfxsvg "codeburg.org/lexbit/lurpicui/gfx/svg"
)

const svgLOD1Size = 32

func compileSVGLOD1(doc gfxsvg.SVGDocument) []byte {
	img := image.NewRGBA(image.Rect(0, 0, svgLOD1Size, svgLOD1Size))
	srcBounds := sourceSVGBounds(doc)
	if srcBounds.IsEmpty() {
		return append([]byte(nil), img.Pix...)
	}

	transform := fitTransform(srcBounds, gfx.RectFromXYWH(0, 0, svgLOD1Size, svgLOD1Size), doc.PreserveAspectRatio)
	scale := transformScale(transform)
	for _, el := range doc.Elements {
		rasterizeSVGElement(img, el, transform, scale)
	}
	return append([]byte(nil), img.Pix...)
}

func rasterizeSVGElement(dst *image.RGBA, el gfxsvg.SVGElement, transform gfx.Transform, scale float32) {
	if dst == nil || len(el.Path.Segments) == 0 {
		return
	}

	path := gfxsvg.Transformed(el.Path, transform)
	if el.Fill.Kind != gfxsvg.SVGPaintNone && el.Fill.Opacity > 0 {
		if fill, ok := svgPaintColor(el.Fill, el.Opacity, gfx.Color{A: 1}); ok {
			rasterizeFilledPath(dst, path, fill)
		}
	}

	if el.Stroke != nil && el.Stroke.Width > 0 {
		strokePath := strokeContourPath(path, el.Stroke.Width*scale)
		if len(strokePath.Segments) > 0 {
			if stroke, ok := svgPaintColor(el.Stroke.Paint, el.Opacity, gfx.Color{A: 1}); ok {
				rasterizeFilledPath(dst, strokePath, stroke)
			}
		}
	}
}

func rasterizeFilledPath(dst *image.RGBA, path gfx.Path, paint gfx.Color) {
	if dst == nil || len(path.Segments) == 0 || paint.A <= 0 {
		return
	}

	bounds := gfxsvg.Bounds(path)
	rr := intersectSVGRect(bounds, gfx.RectFromXYWH(0, 0, float32(dst.Bounds().Dx()), float32(dst.Bounds().Dy())))
	if rr.IsEmpty() {
		return
	}

	minX := clampInt(int(math.Floor(float64(rr.Min.X))), 0, dst.Bounds().Dx())
	minY := clampInt(int(math.Floor(float64(rr.Min.Y))), 0, dst.Bounds().Dy())
	maxX := clampInt(int(math.Ceil(float64(rr.Max.X))), 0, dst.Bounds().Dx())
	maxY := clampInt(int(math.Ceil(float64(rr.Max.Y))), 0, dst.Bounds().Dy())
	if minX >= maxX || minY >= maxY {
		return
	}

	mask := image.NewAlpha(image.Rect(0, 0, maxX-minX, maxY-minY))
	ras := vector.NewRasterizer(mask.Bounds().Dx(), mask.Bounds().Dy())
	ras.DrawOp = draw.Src
	for _, seg := range path.Segments {
		switch seg.Verb {
		case gfx.PathMoveTo:
			ras.MoveTo(seg.Pts[0].X-float32(minX), seg.Pts[0].Y-float32(minY))
		case gfx.PathLineTo:
			ras.LineTo(seg.Pts[0].X-float32(minX), seg.Pts[0].Y-float32(minY))
		case gfx.PathQuadTo:
			ras.QuadTo(
				seg.Pts[0].X-float32(minX), seg.Pts[0].Y-float32(minY),
				seg.Pts[1].X-float32(minX), seg.Pts[1].Y-float32(minY),
			)
		case gfx.PathCubicTo:
			ras.CubeTo(
				seg.Pts[0].X-float32(minX), seg.Pts[0].Y-float32(minY),
				seg.Pts[1].X-float32(minX), seg.Pts[1].Y-float32(minY),
				seg.Pts[2].X-float32(minX), seg.Pts[2].Y-float32(minY),
			)
		case gfx.PathClose:
			ras.ClosePath()
		}
	}
	ras.Draw(mask, mask.Bounds(), image.NewUniform(color.Alpha{A: 255}), image.Point{})

	for y := 0; y < mask.Bounds().Dy(); y++ {
		for x := 0; x < mask.Bounds().Dx(); x++ {
			alpha := mask.AlphaAt(x, y).A
			if alpha == 0 {
				continue
			}
			blendRGBA(dst, minX+x, minY+y, paint, float32(alpha)/255)
		}
	}
}

func blendRGBA(dst *image.RGBA, x, y int, src gfx.Color, coverage float32) {
	if dst == nil || coverage <= 0 {
		return
	}
	if !image.Pt(x, y).In(dst.Bounds()) {
		return
	}

	sr, sg, sb, sa := src.ToRGBA8()
	saF := (float32(sa) / 255) * coverage
	if saF <= 0 {
		return
	}
	dr, dg, db, da := dst.At(x, y).RGBA()
	dstA := float32(da) / 65535
	dstR := float32(dr) / 65535
	dstG := float32(dg) / 65535
	dstB := float32(db) / 65535

	srcR := float32(sr) / 255
	srcG := float32(sg) / 255
	srcB := float32(sb) / 255

	outA := saF + dstA*(1-saF)
	var outR, outG, outB float32
	if outA > 0 {
		outR = (srcR*saF + dstR*dstA*(1-saF)) / outA
		outG = (srcG*saF + dstG*dstA*(1-saF)) / outA
		outB = (srcB*saF + dstB*dstA*(1-saF)) / outA
	}
	dst.SetRGBA(x, y, color.RGBA{
		R: uint8(math.Round(float64(clamp01(outR) * 255))),
		G: uint8(math.Round(float64(clamp01(outG) * 255))),
		B: uint8(math.Round(float64(clamp01(outB) * 255))),
		A: uint8(math.Round(float64(clamp01(outA) * 255))),
	})
}

func svgPaintColor(p gfxsvg.SVGPaint, elementOpacity float32, current gfx.Color) (gfx.Color, bool) {
	if p.Opacity <= 0 {
		return gfx.Color{}, false
	}
	alpha := p.Opacity * elementOpacity
	if alpha <= 0 {
		return gfx.Color{}, false
	}
	switch p.Kind {
	case gfxsvg.SVGPaintColor:
		return gfx.Color{R: p.Color.R * alpha, G: p.Color.G * alpha, B: p.Color.B * alpha, A: p.Color.A * alpha}, true
	case gfxsvg.SVGPaintCurrentColor:
		return gfx.Color{R: current.R * alpha, G: current.G * alpha, B: current.B * alpha, A: current.A * alpha}, true
	default:
		return gfx.Color{}, false
	}
}

func sourceSVGBounds(doc gfxsvg.SVGDocument) gfx.Rect {
	if !doc.ViewBox.IsEmpty() {
		return doc.ViewBox
	}
	if !doc.Bounds.IsEmpty() {
		return doc.Bounds
	}
	return unionElementBounds(doc.Elements)
}

func fitTransform(srcBox, target gfx.Rect, par gfxsvg.SVGPreserveAspectRatio) gfx.Transform {
	if target.IsEmpty() {
		return gfx.Identity()
	}
	if srcBox.IsEmpty() {
		return gfx.Translation(target.Min.X, target.Min.Y)
	}

	meet := true
	align := par.Align
	if align == gfxsvg.SVGAspectRatioAlignUnspecified {
		align = gfxsvg.SVGAspectRatioAlignXMidYMid
	}
	switch par.MeetOrSlice {
	case gfxsvg.SVGMeetOrSliceSlice:
		meet = false
	case gfxsvg.SVGMeetOrSliceMeet:
		meet = true
	}
	scaleX := target.Width() / srcBox.Width()
	scaleY := target.Height() / srcBox.Height()
	if align == gfxsvg.SVGAspectRatioAlignNone {
		return gfx.Transform{
			A:  scaleX,
			D:  scaleY,
			TX: target.Min.X - srcBox.Min.X*scaleX,
			TY: target.Min.Y - srcBox.Min.Y*scaleY,
		}
	}
	scale := math.Min(float64(scaleX), float64(scaleY))
	if !meet {
		scale = math.Max(float64(scaleX), float64(scaleY))
	}
	scaledW := float32(scale) * srcBox.Width()
	scaledH := float32(scale) * srcBox.Height()
	var offsetX float32
	var offsetY float32
	switch align {
	case gfxsvg.SVGAspectRatioAlignXMinYMin:
		offsetX = target.Min.X - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y - srcBox.Min.Y*float32(scale)
	case gfxsvg.SVGAspectRatioAlignXMidYMin:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y - srcBox.Min.Y*float32(scale)
	case gfxsvg.SVGAspectRatioAlignXMaxYMin:
		offsetX = target.Max.X - scaledW - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y - srcBox.Min.Y*float32(scale)
	case gfxsvg.SVGAspectRatioAlignXMinYMid:
		offsetX = target.Min.X - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*float32(scale)
	case gfxsvg.SVGAspectRatioAlignXMidYMid:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*float32(scale)
	case gfxsvg.SVGAspectRatioAlignXMaxYMid:
		offsetX = target.Max.X - scaledW - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*float32(scale)
	case gfxsvg.SVGAspectRatioAlignXMinYMax:
		offsetX = target.Min.X - srcBox.Min.X*float32(scale)
		offsetY = target.Max.Y - scaledH - srcBox.Min.Y*float32(scale)
	case gfxsvg.SVGAspectRatioAlignXMidYMax:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*float32(scale)
		offsetY = target.Max.Y - scaledH - srcBox.Min.Y*float32(scale)
	case gfxsvg.SVGAspectRatioAlignXMaxYMax:
		offsetX = target.Max.X - scaledW - srcBox.Min.X*float32(scale)
		offsetY = target.Max.Y - scaledH - srcBox.Min.Y*float32(scale)
	default:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*float32(scale)
	}
	return gfx.Transform{
		A:  float32(scale),
		D:  float32(scale),
		TX: offsetX,
		TY: offsetY,
	}
}

func transformScale(t gfx.Transform) float32 {
	det := t.A*t.D - t.B*t.C
	if det <= 0 {
		return 1
	}
	return float32(math.Sqrt(float64(det)))
}

func intersectSVGRect(a, b gfx.Rect) gfx.Rect {
	if a.IsEmpty() || b.IsEmpty() {
		return gfx.Rect{}
	}
	minX := a.Min.X
	if b.Min.X > minX {
		minX = b.Min.X
	}
	minY := a.Min.Y
	if b.Min.Y > minY {
		minY = b.Min.Y
	}
	maxX := a.Max.X
	if b.Max.X < maxX {
		maxX = b.Max.X
	}
	maxY := a.Max.Y
	if b.Max.Y < maxY {
		maxY = b.Max.Y
	}
	if minX >= maxX || minY >= maxY {
		return gfx.Rect{}
	}
	return gfx.Rect{Min: gfx.Point{X: minX, Y: minY}, Max: gfx.Point{X: maxX, Y: maxY}}
}

func strokeContourPath(path gfx.Path, width float32) gfx.Path {
	half := width / 2
	if half <= 0 || len(path.Segments) == 0 {
		return gfx.Path{}
	}
	outerSegs := offsetPathContour(path.Segments, half)
	innerSegs := offsetPathContour(path.Segments, -half)
	if len(outerSegs) == 0 {
		return gfx.Path{}
	}
	annular := gfx.Path{Segments: append(outerSegs, reverseContour(innerSegs)...)}
	return annular
}

func offsetPathContour(segs []gfx.PathSegment, d float32) []gfx.PathSegment {
	if len(segs) == 0 {
		return nil
	}
	var cx, cy float32
	var n int
	for _, seg := range segs {
		count := segPointCount(seg.Verb)
		for i := 0; i < count; i++ {
			cx += seg.Pts[i].X
			cy += seg.Pts[i].Y
			n++
		}
	}
	if n == 0 {
		return nil
	}
	cx /= float32(n)
	cy /= float32(n)

	out := make([]gfx.PathSegment, len(segs))
	for i, seg := range segs {
		out[i].Verb = seg.Verb
		count := segPointCount(seg.Verb)
		for j := 0; j < count; j++ {
			p := seg.Pts[j]
			dx := p.X - cx
			dy := p.Y - cy
			len2 := dx*dx + dy*dy
			if len2 > 0 {
				l := float32(math.Sqrt(float64(len2)))
				out[i].Pts[j] = gfx.Point{X: p.X + dx/l*d, Y: p.Y + dy/l*d}
			} else {
				out[i].Pts[j] = p
			}
		}
	}
	return out
}

func reverseContour(segs []gfx.PathSegment) []gfx.PathSegment {
	if len(segs) == 0 {
		return nil
	}
	type ptVerb struct {
		verb gfx.PathVerb
		pts  [3]gfx.Point
	}
	var pts []ptVerb
	for _, seg := range segs {
		if seg.Verb == gfx.PathClose {
			continue
		}
		pts = append(pts, ptVerb{verb: seg.Verb, pts: seg.Pts})
	}
	if len(pts) == 0 {
		return nil
	}
	for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
		pts[i], pts[j] = pts[j], pts[i]
	}
	out := make([]gfx.PathSegment, 0, len(pts)+2)
	start := segs[0].Pts[0]
	out = append(out, gfx.PathSegment{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{start}})
	for _, pv := range pts {
		out = append(out, gfx.PathSegment{Verb: gfx.PathLineTo, Pts: pv.pts})
	}
	out = append(out, gfx.PathSegment{Verb: gfx.PathClose})
	return out
}

func segPointCount(v gfx.PathVerb) int {
	switch v {
	case gfx.PathMoveTo, gfx.PathLineTo:
		return 1
	case gfx.PathQuadTo:
		return 2
	case gfx.PathCubicTo:
		return 3
	default:
		return 0
	}
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
