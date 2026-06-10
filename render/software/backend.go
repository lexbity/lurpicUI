package software

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"sync"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/renderutil"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/text"

	gotextrender "github.com/go-text/render"
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	_ "golang.org/x/image/tiff"
	"golang.org/x/image/vector"
)

var _ = text.GlyphRun{}
var _ = gotextrender.Renderer{}

type blitSurface interface {
	render.SoftwareSurface
}

type RenderBatchCacheEntry struct {
	bounds      gfx.Rect
	commandHash uint64
	buffer      *image.RGBA
}

type glyphEntry struct {
	bitmap  *image.Alpha
	image   image.Image
	color   bool
	offsetX float32
	offsetY float32
}

type glyphAtlas struct {
	mu             sync.Mutex
	entries        map[renderutil.GlyphAtlasKey]*glyphEntry
	rasterizeCount int
}

type renderState struct {
	transform gfx.Transform
	clip      gfx.Rect
	opacity   float32
}

type SoftwareRenderer struct {
	mu               sync.RWMutex
	surface          blitSurface
	output           *image.RGBA
	texBackend       *render.SoftwareBackend
	RenderBatchCache map[render.RenderBatchID]*RenderBatchCacheEntry
	diffCache        *renderutil.RenderBatchCache
	glyphAtlas       *glyphAtlas
	width            int
	height           int

	rasterizeCount int
}

func NewSoftwareRenderer() *SoftwareRenderer {
	return &SoftwareRenderer{
		RenderBatchCache: make(map[render.RenderBatchID]*RenderBatchCacheEntry),
		diffCache:        renderutil.NewRenderBatchCache(),
		glyphAtlas:       &glyphAtlas{entries: make(map[renderutil.GlyphAtlasKey]*glyphEntry)},
		texBackend:       &render.SoftwareBackend{},
	}
}

func (r *SoftwareRenderer) Initialize(surface render.Surface) error {
	if surface == nil {
		return errors.New("software renderer: nil surface")
	}
	blit, ok := surface.(blitSurface)
	if !ok {
		return errors.New("software renderer: surface must implement render.SoftwareSurface")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.surface = blit
	w, h := surface.Size()
	r.allocateOutput(w, h)
	return nil
}

func (r *SoftwareRenderer) Resize(width, height int) error {
	if width < 0 || height < 0 {
		return errors.New("software renderer: invalid size")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.allocateOutput(width, height)
	if r.surface != nil {
		if resizable, ok := any(r.surface).(interface{ Resize(int, int) }); ok {
			resizable.Resize(width, height)
		}
	}
	return nil
}

func (r *SoftwareRenderer) Destroy() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.surface = nil
	r.output = nil
	r.width = 0
	r.height = 0
	r.RenderBatchCache = make(map[render.RenderBatchID]*RenderBatchCacheEntry)
	r.diffCache = renderutil.NewRenderBatchCache()
	r.glyphAtlas = &glyphAtlas{entries: make(map[renderutil.GlyphAtlasKey]*glyphEntry)}
}

// EvictCaches releases recoverable renderer caches without dropping the surface.
func (r *SoftwareRenderer) EvictCaches() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.RenderBatchCache = make(map[render.RenderBatchID]*RenderBatchCacheEntry)
	r.diffCache = renderutil.NewRenderBatchCache()
	r.glyphAtlas = &glyphAtlas{entries: make(map[renderutil.GlyphAtlasKey]*glyphEntry)}
	r.rasterizeCount = 0
}

func (r *SoftwareRenderer) RasterizeCount() int {
	return r.rasterizeCount
}

// GlyphRasterizeCount reports the number of unique glyph bitmaps generated.
func (r *SoftwareRenderer) GlyphRasterizeCount() int {
	if r == nil || r.glyphAtlas == nil {
		return 0
	}
	r.glyphAtlas.mu.Lock()
	defer r.glyphAtlas.mu.Unlock()
	return r.glyphAtlas.rasterizeCount
}

func (r *SoftwareRenderer) Submit(frame *render.Frame) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.surface == nil {
		return errors.New("software renderer: not initialized")
	}
	if frame == nil {
		return nil
	}

	w, h := r.surface.Size()
	if w != r.width || h != r.height || r.output == nil {
		r.allocateOutput(w, h)
	}

	diff := r.diffCache.Diff(frame)
	clearRGBA(r.output)

	seen := make(map[render.RenderBatchID]struct{}, len(frame.RenderBatchs))
	if len(frame.Layers) > 0 {
		for _, layer := range frame.Layers {
			for _, RenderBatch := range layer.Batches {
				seen[RenderBatch.ID] = struct{}{}
				ld := diff.RenderBatchs[RenderBatch.ID]
				if ld.Kind == renderutil.RenderBatchUnchanged {
					if entry := r.RenderBatchCache[RenderBatch.ID]; entry != nil && entry.buffer != nil {
						r.compositeRenderBatch(r.output, entry.buffer, RenderBatch.Bounds.Min, RenderBatch.Opacity)
					}
					continue
				}
				if ld.Kind == renderutil.RenderBatchRemoved {
					delete(r.RenderBatchCache, RenderBatch.ID)
					continue
				}

				sizeW := int(math.Ceil(float64(RenderBatch.Bounds.Width())))
				sizeH := int(math.Ceil(float64(RenderBatch.Bounds.Height())))
				if sizeW < 0 {
					sizeW = 0
				}
				if sizeH < 0 {
					sizeH = 0
				}

				buffer := image.NewRGBA(image.Rect(0, 0, sizeW, sizeH))
				r.rasterizeRenderBatch(buffer, &RenderBatch, layer.ClipRect)
				r.rasterizeCount++
				r.RenderBatchCache[RenderBatch.ID] = &RenderBatchCacheEntry{
					bounds:      RenderBatch.Bounds,
					commandHash: RenderBatch.CommandHash,
					buffer:      buffer,
				}
				r.compositeRenderBatch(r.output, buffer, RenderBatch.Bounds.Min, RenderBatch.Opacity)
			}
		}
	} else {
		for _, RenderBatch := range frame.RenderBatchs {
			seen[RenderBatch.ID] = struct{}{}
			ld := diff.RenderBatchs[RenderBatch.ID]
			if ld.Kind == renderutil.RenderBatchUnchanged {
				if entry := r.RenderBatchCache[RenderBatch.ID]; entry != nil && entry.buffer != nil {
					r.compositeRenderBatch(r.output, entry.buffer, RenderBatch.Bounds.Min, RenderBatch.Opacity)
				}
				continue
			}
			if ld.Kind == renderutil.RenderBatchRemoved {
				delete(r.RenderBatchCache, RenderBatch.ID)
				continue
			}

			sizeW := int(math.Ceil(float64(RenderBatch.Bounds.Width())))
			sizeH := int(math.Ceil(float64(RenderBatch.Bounds.Height())))
			if sizeW < 0 {
				sizeW = 0
			}
			if sizeH < 0 {
				sizeH = 0
			}

			buffer := image.NewRGBA(image.Rect(0, 0, sizeW, sizeH))
			r.rasterizeRenderBatch(buffer, &RenderBatch, gfx.Rect{})
			r.rasterizeCount++
			r.RenderBatchCache[RenderBatch.ID] = &RenderBatchCacheEntry{
				bounds:      RenderBatch.Bounds,
				commandHash: RenderBatch.CommandHash,
				buffer:      buffer,
			}
			r.compositeRenderBatch(r.output, buffer, RenderBatch.Bounds.Min, RenderBatch.Opacity)
		}
	}

	for id := range r.RenderBatchCache {
		if _, ok := seen[id]; !ok {
			delete(r.RenderBatchCache, id)
		}
	}

	if len(r.output.Pix) >= 4 {
		tl := sampleRGBAAt(r.output, 0, 0)
		center := sampleRGBAAt(r.output, r.width/2, r.height/2)
		androidTracef("software output sample w=%d h=%d tl=%02x%02x%02x%02x center=%02x%02x%02x%02x",
			r.width, r.height, tl[0], tl[1], tl[2], tl[3], center[0], center[1], center[2], center[3])
	}

	if err := r.blitToSurface(); err != nil {
		return err
	}

	buffers := make(map[render.RenderBatchID]*image.RGBA, len(r.RenderBatchCache))
	for id, entry := range r.RenderBatchCache {
		buffers[id] = entry.buffer
	}
	r.diffCache.Update(frame, buffers)
	return nil
}

func flattenLayerBatches(layers []render.LayeredBatch) []render.RenderBatch {
	if len(layers) == 0 {
		return nil
	}
	var total int
	for _, layer := range layers {
		total += len(layer.Batches)
	}
	out := make([]render.RenderBatch, 0, total)
	for _, layer := range layers {
		out = append(out, layer.Batches...)
	}
	return out
}

func (r *SoftwareRenderer) allocateOutput(width, height int) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	r.width = width
	r.height = height
	r.output = image.NewRGBA(image.Rect(0, 0, width, height))
}

func (r *SoftwareRenderer) blitToSurface() error {
	surface := r.surface
	if surface == nil || r.output == nil {
		return nil
	}
	if err := surface.Lock(); err != nil {
		return err
	}
	defer func() {
		_ = surface.Unlock(nil)
	}()

	dst := surface.Buffer()
	stride := surface.Stride()
	if len(dst) == 0 || r.output.Stride == 0 {
		return nil
	}
	if stride <= 0 {
		stride = r.width * 4
	}
	rowBytes := r.width * 4
	if rowBytes > r.output.Stride {
		rowBytes = r.output.Stride
	}
	if rowBytes > stride {
		rowBytes = stride
	}
	maxRows := r.height
	if maxRows > 0 {
		if liveRows := len(dst) / stride; liveRows < maxRows {
			maxRows = liveRows
		}
	}
	for y := 0; y < maxRows; y++ {
		srcOff := y * r.output.Stride
		dstOff := y * stride
		if srcOff >= len(r.output.Pix) || dstOff >= len(dst) {
			break
		}
		n := rowBytes
		if remaining := len(r.output.Pix) - srcOff; remaining < n {
			n = remaining
		}
		if remaining := len(dst) - dstOff; remaining < n {
			n = remaining
		}
		if n <= 0 {
			break
		}
		copy(dst[dstOff:dstOff+n], r.output.Pix[srcOff:srcOff+n])
	}
	return nil
}

func sampleRGBAAt(img *image.RGBA, x, y int) [4]byte {
	if img == nil || x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
		return [4]byte{}
	}
	off := y*img.Stride + x*4
	if off < 0 || off+4 > len(img.Pix) {
		return [4]byte{}
	}
	var out [4]byte
	copy(out[:], img.Pix[off:off+4])
	return out
}

func (r *SoftwareRenderer) compositeRenderBatch(dst *image.RGBA, src *image.RGBA, offset gfx.Point, opacity float32) {
	if dst == nil || src == nil || opacity <= 0 {
		return
	}
	ox := int(math.Round(float64(offset.X)))
	oy := int(math.Round(float64(offset.Y)))
	for sy := 0; sy < src.Bounds().Dy(); sy++ {
		dy := oy + sy
		if dy < 0 || dy >= dst.Bounds().Dy() {
			continue
		}
		for sx := 0; sx < src.Bounds().Dx(); sx++ {
			dx := ox + sx
			if dx < 0 || dx >= dst.Bounds().Dx() {
				continue
			}
			sIdx := sy*src.Stride + sx*4
			dIdx := dy*dst.Stride + dx*4
			blendPremul(dst.Pix[dIdx:dIdx+4], src.Pix[sIdx:sIdx+4], opacity)
		}
	}
}

func (r *SoftwareRenderer) rasterizeRenderBatch(target *image.RGBA, RenderBatch *render.RenderBatch, clip gfx.Rect) {
	if target == nil || RenderBatch == nil {
		return
	}
	state := renderState{
		transform: gfx.Translation(-RenderBatch.Bounds.Min.X, -RenderBatch.Bounds.Min.Y),
		clip:      gfx.RectFromXYWH(0, 0, float32(target.Bounds().Dx()), float32(target.Bounds().Dy())),
		opacity:   1,
	}
	if !clip.IsEmpty() {
		state.clip = intersectRects(state.clip, state.transform.TransformRect(clip))
	}
	stack := []renderState{state}

	for _, cmd := range RenderBatch.Commands.Commands {
		state = stack[len(stack)-1]
		switch c := cmd.(type) {
		case gfx.PushTransform:
			next := state
			next.transform = next.transform.Multiply(c.Matrix)
			stack = append(stack, next)
		case gfx.PopTransform:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case gfx.PushClipRect:
			next := state
			clip := state.transform.TransformRect(c.Rect)
			next.clip = intersectRects(next.clip, clip)
			stack = append(stack, next)
		case gfx.PopClip:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case gfx.PushOpacity:
			next := state
			next.opacity *= c.Alpha
			stack = append(stack, next)
		case gfx.PopOpacity:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case gfx.FillRect:
			fillRect(target, state, c.Rect, c.Brush)
		case gfx.StrokeRect:
			strokeRect(target, state, c.Rect, c.Stroke, c.Brush)
		case gfx.FillPath:
			fillPath(target, state, c.Path, c.Brush)
		case gfx.StrokePath:
			strokePath(target, state, c.Path, c.Stroke, c.Brush)
		case gfx.DrawPolyline:
			drawPolyline(target, state, c.Points, c.Stroke, c.Brush, c.Closed)
		case gfx.DrawPoints:
			drawPoints(target, state, c.Points, c.Radius, c.Brush)
		case gfx.DrawGlyphRun:
			r.drawGlyphRun(target, state, c)
		case gfx.DrawSelectionRects:
			for _, rr := range c.Rects {
				fillRect(target, state, rr, c.Brush)
			}
		case gfx.DrawImage:
			drawImage(target, state, c)
		case gfx.DrawTexture:
			r.drawTexture(target, state, c)
		case gfx.BeginRenderBatch, gfx.EndRenderBatch:
		}
	}
}

func fillRect(target *image.RGBA, state renderState, rect gfx.Rect, brush gfx.Brush) {
	rr := intersectRects(state.transform.TransformRect(rect), state.clip)
	if rr.IsEmpty() {
		return
	}

	minX := clampInt(int(math.Floor(float64(rr.Min.X))), 0, target.Bounds().Dx())
	minY := clampInt(int(math.Floor(float64(rr.Min.Y))), 0, target.Bounds().Dy())
	maxX := clampInt(int(math.Ceil(float64(rr.Max.X))), 0, target.Bounds().Dx())
	maxY := clampInt(int(math.Ceil(float64(rr.Max.Y))), 0, target.Bounds().Dy())
	if minX >= maxX || minY >= maxY {
		return
	}

	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			c := sampleBrush(brush, gfx.Point{X: float32(x), Y: float32(y)}, rr)
			blendAt(target, x, y, c, state.opacity)
		}
	}
}

func strokeRect(target *image.RGBA, state renderState, rect gfx.Rect, stroke gfx.StrokeStyle, brush gfx.Brush) {
	width := stroke.Width
	if width <= 0 {
		return
	}
	outer := rect.Inset(-width/2, -width/2)
	inner := rect.Inset(width/2, width/2)
	fillRect(target, state, gfx.Rect{Min: outer.Min, Max: gfx.Point{X: outer.Max.X, Y: inner.Min.Y}}, brush)
	fillRect(target, state, gfx.Rect{Min: gfx.Point{X: outer.Min.X, Y: inner.Min.Y}, Max: gfx.Point{X: inner.Min.X, Y: inner.Max.Y}}, brush)
	fillRect(target, state, gfx.Rect{Min: gfx.Point{X: inner.Max.X, Y: inner.Min.Y}, Max: gfx.Point{X: outer.Max.X, Y: inner.Max.Y}}, brush)
	fillRect(target, state, gfx.Rect{Min: gfx.Point{X: outer.Min.X, Y: inner.Max.Y}, Max: outer.Max}, brush)
}

func fillPath(target *image.RGBA, state renderState, path gfx.Path, brush gfx.Brush) {
	rasterizePath(target, state, path, brush, 1)
}

func strokePath(target *image.RGBA, state renderState, path gfx.Path, stroke gfx.StrokeStyle, brush gfx.Brush) {
	if len(path.Segments) == 0 || stroke.Width <= 0 {
		return
	}
	rasterizeStrokePath(target, state, path, stroke.Width, brush)
}

func drawPolyline(target *image.RGBA, state renderState, pts []gfx.Point, stroke gfx.StrokeStyle, brush gfx.Brush, closed bool) {
	if len(pts) == 0 || stroke.Width <= 0 {
		return
	}
	for i := 0; i < len(pts)-1; i++ {
		rasterizeSegment(target, state, pts[i], pts[i+1], stroke.Width, brush)
	}
	if closed && len(pts) >= 2 {
		rasterizeSegment(target, state, pts[len(pts)-1], pts[0], stroke.Width, brush)
	}
}

func drawPoints(target *image.RGBA, state renderState, pts []gfx.Point, radius float32, brush gfx.Brush) {
	if len(pts) == 0 || radius <= 0 {
		return
	}
	for _, p := range pts {
		fillRect(target, state, gfx.RectFromXYWH(p.X-radius, p.Y-radius, radius*2, radius*2), brush)
	}
}

// rasterizeStrokePath renders a closed-path stroke as an annular fill by feeding
// the outer-expanded and inner-contracted contours with opposite winding into a
// single rasterizer. golang.org/x/image/vector uses non-zero winding, so the
// opposing windings cancel in the interior, leaving only the ring band filled.
func rasterizeStrokePath(target *image.RGBA, state renderState, path gfx.Path, width float32, brush gfx.Brush) {
	half := width / 2
	if half <= 0 {
		return
	}

	// Build the annular path: outer contour CW then inner contour CCW.
	// The rasterizer accumulates winding counts; opposite windings cancel
	// inside the inner contour, so only the band between outer and inner fills.
	outerSegs := gfx.OffsetContour(path.Segments, half)
	innerSegs := gfx.OffsetContour(path.Segments, -half)

	if len(outerSegs) == 0 {
		return
	}

	// Combine into one path: outer (CW) followed by inner reversed (CCW).
	annular := gfx.Path{Segments: append(outerSegs, reverseContour(innerSegs)...)}
	rasterizePath(target, state, annular, brush, 1)
}

// reverseContour reverses segment order and re-winds a closed contour so it
// has opposite winding from the original. This is used to cut the interior
// out of the outer stroke contour.
func reverseContour(segs []gfx.PathSegment) []gfx.PathSegment {
	if len(segs) == 0 {
		return nil
	}

	// Collect the actual point sequence in forward order (skip Close/MoveTo verbs
	// as control-flow rather than point-bearing).
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

	// Reverse the sequence.
	for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
		pts[i], pts[j] = pts[j], pts[i]
	}

	out := make([]gfx.PathSegment, 0, len(pts)+2)
	// Start at the last point of the reversed sequence (first original point).
	start := segs[0].Pts[0] // original start
	out = append(out, gfx.PathSegment{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{start}})
	for _, pv := range pts {
		out = append(out, gfx.PathSegment{Verb: gfx.PathLineTo, Pts: pv.pts})
	}
	out = append(out, gfx.PathSegment{Verb: gfx.PathClose})
	return out
}

// rasterizeSegment strokes a single line segment A→B with the given half-width
// by rasterizing an axis-aligned rectangle around it.
func rasterizeSegment(target *image.RGBA, state renderState, a, b gfx.Point, width float32, brush gfx.Brush) {
	half := width / 2
	dx := b.X - a.X
	dy := b.Y - a.Y
	l := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if l <= 0 {
		// Degenerate segment — draw a square cap.
		fillRect(target, state, gfx.RectFromXYWH(a.X-half, a.Y-half, width, width), brush)
		return
	}
	// Perpendicular unit vector.
	nx := -dy / l
	ny := dx / l
	p1 := gfx.Point{X: a.X + nx*half, Y: a.Y + ny*half}
	p2 := gfx.Point{X: a.X - nx*half, Y: a.Y - ny*half}
	p3 := gfx.Point{X: b.X - nx*half, Y: b.Y - ny*half}
	p4 := gfx.Point{X: b.X + nx*half, Y: b.Y + ny*half}
	path := gfx.NewPath().
		MoveTo(p1).
		LineTo(p2).
		LineTo(p3).
		LineTo(p4).
		Close().
		Build()
	rasterizePath(target, state, path, brush, 1)
}

func (r *SoftwareRenderer) drawGlyphRun(target *image.RGBA, state renderState, cmd gfx.DrawGlyphRun) {
	if target == nil || len(cmd.Run.Glyphs) == 0 {
		return
	}
	atlas := r.glyphAtlas
	if atlas == nil {
		return
	}
	for _, glyph := range cmd.Run.Glyphs {
		entry := atlas.getOrRasterize(cmd.Run, glyph)
		if entry == nil || entry.bitmap == nil {
			continue
		}
		pos := state.transform.TransformPoint(gfx.Point{
			X: cmd.Origin.X + glyph.X + entry.offsetX,
			Y: cmd.Origin.Y + glyph.Y + entry.offsetY,
		})
		drawGlyphBitmap(target, state, entry, pos, cmd.Brush)
	}
}

func rasterizePath(target *image.RGBA, state renderState, path gfx.Path, brush gfx.Brush, opacity float32) {
	if target == nil || len(path.Segments) == 0 {
		return
	}

	var transformed gfx.Path
	transformed.Segments = make([]gfx.PathSegment, 0, len(path.Segments))
	for _, seg := range path.Segments {
		out := gfx.PathSegment{Verb: seg.Verb}
		for i := range seg.Pts {
			out.Pts[i] = state.transform.TransformPoint(seg.Pts[i])
		}
		transformed.Segments = append(transformed.Segments, out)
	}

	bounds := pathBounds(transformed)
	rr := intersectRects(bounds, state.clip)
	if rr.IsEmpty() {
		return
	}
	minX := clampInt(int(math.Floor(float64(rr.Min.X))), 0, target.Bounds().Dx())
	minY := clampInt(int(math.Floor(float64(rr.Min.Y))), 0, target.Bounds().Dy())
	maxX := clampInt(int(math.Ceil(float64(rr.Max.X))), 0, target.Bounds().Dx())
	maxY := clampInt(int(math.Ceil(float64(rr.Max.Y))), 0, target.Bounds().Dy())
	if minX >= maxX || minY >= maxY {
		return
	}

	mask := image.NewAlpha(image.Rect(0, 0, maxX-minX, maxY-minY))
	ras := vector.NewRasterizer(mask.Bounds().Dx(), mask.Bounds().Dy())
	ras.DrawOp = draw.Src
	for _, seg := range transformed.Segments {
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
			a := mask.AlphaAt(x, y).A
			if a == 0 {
				continue
			}
			px := minX + x
			py := minY + y
			c := sampleBrush(brush, gfx.Point{X: float32(px), Y: float32(py)}, rr)
			blendAt(target, px, py, c, state.opacity*opacity*float32(a)/255)
		}
	}
}

func (a *glyphAtlas) getOrRasterize(run text.GlyphRun, glyph text.PositionedGlyph) *glyphEntry {
	if a == nil {
		return nil
	}
	key := renderutil.GlyphAtlasKeyFromRun(run, glyph.GlyphID)
	size := math.Float32frombits(key.SizeBits)

	a.mu.Lock()
	defer a.mu.Unlock()
	if entry := a.entries[key]; entry != nil {
		return entry
	}
	entry := rasterizeGlyphEntry(run, glyph, size)
	a.entries[key] = entry
	a.rasterizeCount++
	return entry
}

func rasterizeGlyphEntry(run text.GlyphRun, glyph text.PositionedGlyph, size float32) *glyphEntry {
	goFace := run.Face.GoFace()
	if goFace == nil {
		return nil
	}
	gid := font.GID(glyph.GlyphID)
	data := goFace.GlyphData(gid)
	if data == nil {
		return nil
	}
	scale := size / float32(goFace.Upem())
	if scale <= 0 {
		scale = 1
	}

	switch gd := data.(type) {
	case font.GlyphOutline:
		extents, ok := goFace.GlyphExtents(gid)
		return rasterizeOutlineGlyph(gd, extents, ok, scale)
	case font.GlyphSVG:
		extents, ok := goFace.GlyphExtents(gid)
		return rasterizeOutlineGlyph(gd.Outline, extents, ok, scale)
	case font.GlyphBitmap:
		extents, ok := goFace.GlyphExtents(gid)
		return rasterizeBitmapGlyph(gd, extents, ok, scale)
	default:
		return nil
	}
}

func rasterizeOutlineGlyph(outline font.GlyphOutline, extents font.GlyphExtents, hasExtents bool, scale float32) *glyphEntry {
	if len(outline.Segments) == 0 {
		return &glyphEntry{bitmap: image.NewAlpha(image.Rect(0, 0, 1, 1))}
	}
	minX, minY, maxX, maxY := outlineBounds(outline, scale)
	originX := float32(math.Floor(float64(minX)))
	originY := float32(math.Floor(float64(minY)))
	w := maxInt(1, int(math.Ceil(float64(maxX)))-int(originX)+2)
	h := maxInt(1, int(math.Ceil(float64(maxY)))-int(originY)+2)
	bmp := image.NewAlpha(image.Rect(0, 0, w, h))
	ras := vector.NewRasterizer(w, h)
	ras.DrawOp = draw.Src
	for _, seg := range outline.Segments {
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			ras.MoveTo(seg.Args[0].X*scale-originX+1, float32(h)-(seg.Args[0].Y*scale-originY+1))
		case ot.SegmentOpLineTo:
			ras.LineTo(seg.Args[0].X*scale-originX+1, float32(h)-(seg.Args[0].Y*scale-originY+1))
		case ot.SegmentOpQuadTo:
			ras.QuadTo(
				seg.Args[0].X*scale-originX+1, float32(h)-(seg.Args[0].Y*scale-originY+1),
				seg.Args[1].X*scale-originX+1, float32(h)-(seg.Args[1].Y*scale-originY+1),
			)
		case ot.SegmentOpCubeTo:
			ras.CubeTo(
				seg.Args[0].X*scale-originX+1, float32(h)-(seg.Args[0].Y*scale-originY+1),
				seg.Args[1].X*scale-originX+1, float32(h)-(seg.Args[1].Y*scale-originY+1),
				seg.Args[2].X*scale-originX+1, float32(h)-(seg.Args[2].Y*scale-originY+1),
			)
		}
	}
	ras.Draw(bmp, bmp.Bounds(), image.NewUniform(color.Alpha{A: 255}), image.Point{})
	entry := &glyphEntry{bitmap: bmp}
	entry.offsetX = originX - 1
	entry.offsetY = -originY - float32(h) + 1
	return entry
}

func rasterizeBitmapGlyph(gd font.GlyphBitmap, extents font.GlyphExtents, hasExtents bool, scale float32) *glyphEntry {
	if gd.Width <= 0 || gd.Height <= 0 {
		return &glyphEntry{bitmap: image.NewAlpha(image.Rect(0, 0, 1, 1))}
	}

	bmp := image.NewAlpha(image.Rect(0, 0, gd.Width, gd.Height))
	switch gd.Format {
	case font.BlackAndWhite:
		if len(gd.Data) < ((gd.Width*gd.Height)+7)/8 {
			return &glyphEntry{bitmap: bmp}
		}
		for y := 0; y < gd.Height; y++ {
			for x := 0; x < gd.Width; x++ {
				idx := y*gd.Width + x
				bit := gd.Data[idx/8] >> uint(7-(idx%8))
				if bit&1 == 1 {
					bmp.SetAlpha(x, y, color.Alpha{A: 255})
				}
			}
		}
	default:
		src, _, err := image.Decode(bytes.NewReader(gd.Data))
		if err != nil {
			return &glyphEntry{bitmap: bmp}
		}
		bounds := src.Bounds()
		if bounds.Empty() {
			return &glyphEntry{bitmap: bmp}
		}
		entry := &glyphEntry{
			image: src,
			color: true,
		}
		if hasExtents {
			entry.offsetX = extents.XBearing * scale
			entry.offsetY = -extents.YBearing * scale
		}
		return entry
	}

	entry := &glyphEntry{bitmap: bmp}
	if gd.Outline != nil {
		minX, _, _, maxY := outlineBounds(*gd.Outline, scale)
		entry.offsetX = minX
		entry.offsetY = -maxY
		return entry
	}
	if hasExtents {
		entry.offsetX = extents.XBearing * scale
		entry.offsetY = -extents.YBearing * scale
	}
	return entry
}

func outlineBounds(outline font.GlyphOutline, scale float32) (minX, minY, maxX, maxY float32) {
	first := true
	for _, seg := range outline.Segments {
		for _, pt := range seg.Args {
			x := pt.X * scale
			y := pt.Y * scale
			if first {
				minX, maxX = x, x
				minY, maxY = y, y
				first = false
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if first {
		return 0, 0, 0, 0
	}
	return
}

func drawGlyphBitmap(target *image.RGBA, state renderState, entry *glyphEntry, pos gfx.Point, brush gfx.Brush) {
	if target == nil || entry == nil {
		return
	}
	ox := int(math.Round(float64(pos.X)))
	oy := int(math.Round(float64(pos.Y)))
	if entry.color && entry.image != nil {
		bounds := entry.image.Bounds()
		for sy := 0; sy < bounds.Dy(); sy++ {
			dy := oy + sy
			if dy < 0 || dy >= target.Bounds().Dy() || dy < int(state.clip.Min.Y) || dy >= int(state.clip.Max.Y) {
				continue
			}
			for sx := 0; sx < bounds.Dx(); sx++ {
				dx := ox + sx
				if dx < 0 || dx >= target.Bounds().Dx() || dx < int(state.clip.Min.X) || dx >= int(state.clip.Max.X) {
					continue
				}
				cr, cg, cb, ca := entry.image.At(bounds.Min.X+sx, bounds.Min.Y+sy).RGBA()
				if ca == 0 {
					continue
				}
				c := gfx.Color{
					R: float32(cr) / 65535,
					G: float32(cg) / 65535,
					B: float32(cb) / 65535,
					A: float32(ca) / 65535 * state.opacity,
				}.Premultiply()
				sr, sg, sb, sa := colorToBytes(c, 1)
				off := target.PixOffset(dx, dy)
				if off < 0 || off+3 >= len(target.Pix) {
					continue
				}
				blendPremul(target.Pix[off:off+4], []byte{sr, sg, sb, sa}, 1)
			}
		}
		return
	}
	bmp := entry.bitmap
	if bmp == nil {
		return
	}
	bounds := bmp.Bounds()
	for sy := 0; sy < bounds.Dy(); sy++ {
		dy := oy + sy
		if dy < 0 || dy >= target.Bounds().Dy() || dy < int(state.clip.Min.Y) || dy >= int(state.clip.Max.Y) {
			continue
		}
		for sx := 0; sx < bounds.Dx(); sx++ {
			dx := ox + sx
			if dx < 0 || dx >= target.Bounds().Dx() || dx < int(state.clip.Min.X) || dx >= int(state.clip.Max.X) {
				continue
			}
			a := bmp.AlphaAt(bounds.Min.X+sx, bounds.Min.Y+sy).A
			if a == 0 {
				continue
			}
			c := sampleBrush(brush, gfx.Point{X: float32(dx), Y: float32(dy)}, gfx.Rect{})
			c = c.Premultiply()
			alpha := float32(a) / 255 * state.opacity
			c.R *= alpha
			c.G *= alpha
			c.B *= alpha
			c.A *= alpha
			sr, sg, sb, sa := colorToBytes(c, 1)
			off := target.PixOffset(dx, dy)
			if off < 0 || off+3 >= len(target.Pix) {
				continue
			}
			blendPremul(target.Pix[off:off+4], []byte{sr, sg, sb, sa}, 1)
		}
	}
}

func drawImage(target *image.RGBA, state renderState, cmd gfx.DrawImage) {
	if cmd.Image == nil {
		return
	}
	dest := intersectRects(state.transform.TransformRect(cmd.DestRect), state.clip)
	if dest.IsEmpty() {
		return
	}

	srcRect := cmd.SrcRect
	if srcRect.IsEmpty() {
		srcRect = gfx.RectFromXYWH(0, 0, float32(cmd.Image.Bounds().Dx()), float32(cmd.Image.Bounds().Dy()))
	}

	minX := clampInt(int(math.Floor(float64(dest.Min.X))), 0, target.Bounds().Dx())
	minY := clampInt(int(math.Floor(float64(dest.Min.Y))), 0, target.Bounds().Dy())
	maxX := clampInt(int(math.Ceil(float64(dest.Max.X))), 0, target.Bounds().Dx())
	maxY := clampInt(int(math.Ceil(float64(dest.Max.Y))), 0, target.Bounds().Dy())
	if minX >= maxX || minY >= maxY {
		return
	}

	srcW := srcRect.Width()
	srcH := srcRect.Height()
	dstW := dest.Width()
	dstH := dest.Height()
	if dstW == 0 || dstH == 0 {
		return
	}

	for y := minY; y < maxY; y++ {
		ty := (float32(y) + 0.5 - dest.Min.Y) / dstH
		sy := srcRect.Min.Y + ty*srcH
		for x := minX; x < maxX; x++ {
			tx := (float32(x) + 0.5 - dest.Min.X) / dstW
			sx := srcRect.Min.X + tx*srcW
			c := sampleImageNearest(cmd.Image, sx, sy)
			blendAt(target, x, y, c, state.opacity*cmd.Opacity)
		}
	}
}

func sampleBrush(brush gfx.Brush, p gfx.Point, bounds gfx.Rect) gfx.Color {
	switch brush.Kind {
	case gfx.BrushSolid:
		return brush.Color
	case gfx.BrushLinearGradient:
		return sampleLinearGradient(brush, p)
	default:
		return gfx.Color{}
	}
}

func sampleLinearGradient(brush gfx.Brush, p gfx.Point) gfx.Color {
	stops := brush.GradientStops
	if len(stops) == 0 {
		return gfx.Color{}
	}
	if len(stops) == 1 {
		return stops[0].Color
	}
	dx := brush.GradientEnd.X - brush.GradientStart.X
	dy := brush.GradientEnd.Y - brush.GradientStart.Y
	denom := dx*dx + dy*dy
	if denom == 0 {
		return stops[len(stops)-1].Color
	}
	t := ((p.X-brush.GradientStart.X)*dx + (p.Y-brush.GradientStart.Y)*dy) / denom
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	left := stops[0]
	right := stops[len(stops)-1]
	for i := 0; i < len(stops)-1; i++ {
		if t >= stops[i].Offset && t <= stops[i+1].Offset {
			left = stops[i]
			right = stops[i+1]
			break
		}
	}

	if right.Offset == left.Offset {
		return right.Color
	}
	f := (t - left.Offset) / (right.Offset - left.Offset)
	return lerpColor(left.Color, right.Color, f)
}

func sampleImageNearest(img *image.RGBA, sx, sy float32) gfx.Color {
	x := clampInt(int(math.Round(float64(sx))), img.Rect.Min.X, img.Rect.Max.X-1)
	y := clampInt(int(math.Round(float64(sy))), img.Rect.Min.Y, img.Rect.Max.Y-1)
	if x < img.Rect.Min.X || x >= img.Rect.Max.X || y < img.Rect.Min.Y || y >= img.Rect.Max.Y {
		return gfx.Color{}
	}
	off := img.PixOffset(x, y)
	r, g, b, a := img.Pix[off], img.Pix[off+1], img.Pix[off+2], img.Pix[off+3]
	return colorFromBytes(r, g, b, a)
}

func blendAt(img *image.RGBA, x, y int, src gfx.Color, opacity float32) {
	if img == nil {
		return
	}
	off := img.PixOffset(x, y)
	if off < 0 || off+3 >= len(img.Pix) {
		return
	}
	sr, sg, sb, sa := colorToBytes(src, opacity)
	blendPremul(img.Pix[off:off+4], []byte{sr, sg, sb, sa}, 1)
}

func blendPremul(dst []byte, src []byte, opacity float32) {
	if opacity <= 0 {
		return
	}
	sr, sg, sb, sa := src[0], src[1], src[2], src[3]
	if opacity != 1 {
		sr = scaleByte(sr, opacity)
		sg = scaleByte(sg, opacity)
		sb = scaleByte(sb, opacity)
		sa = scaleByte(sa, opacity)
	}
	inv := int(255 - sa)
	dst[0] = clampByte(int(sr) + mul255(dst[0], inv))
	dst[1] = clampByte(int(sg) + mul255(dst[1], inv))
	dst[2] = clampByte(int(sb) + mul255(dst[2], inv))
	dst[3] = clampByte(int(sa) + mul255(dst[3], inv))
}

func colorToBytes(c gfx.Color, opacity float32) (r, g, b, a uint8) {
	r = scaleByte(clampByte(int(math.Round(float64(c.R*255)))), opacity)
	g = scaleByte(clampByte(int(math.Round(float64(c.G*255)))), opacity)
	b = scaleByte(clampByte(int(math.Round(float64(c.B*255)))), opacity)
	a = scaleByte(clampByte(int(math.Round(float64(c.A*255)))), opacity)
	return
}

func colorFromBytes(r, g, b, a uint8) gfx.Color {
	return gfx.Color{
		R: float32(r) / 255,
		G: float32(g) / 255,
		B: float32(b) / 255,
		A: float32(a) / 255,
	}
}

func lerpColor(a, b gfx.Color, t float32) gfx.Color {
	return gfx.Color{
		R: a.R + (b.R-a.R)*t,
		G: a.G + (b.G-a.G)*t,
		B: a.B + (b.B-a.B)*t,
		A: a.A + (b.A-a.A)*t,
	}
}

func pathBounds(path gfx.Path) gfx.Rect {
	var bounds gfx.Rect
	first := true
	for _, seg := range path.Segments {
		var count int
		switch seg.Verb {
		case gfx.PathMoveTo, gfx.PathLineTo:
			count = 1
		case gfx.PathQuadTo:
			count = 2
		case gfx.PathCubicTo:
			count = 3
		default:
			count = 0
		}
		for i := 0; i < count; i++ {
			p := seg.Pts[i]
			if first {
				bounds = gfx.Rect{Min: p, Max: p}
				first = false
				continue
			}
			if p.X < bounds.Min.X {
				bounds.Min.X = p.X
			}
			if p.Y < bounds.Min.Y {
				bounds.Min.Y = p.Y
			}
			if p.X > bounds.Max.X {
				bounds.Max.X = p.X
			}
			if p.Y > bounds.Max.Y {
				bounds.Max.Y = p.Y
			}
		}
	}
	if first {
		return gfx.Rect{}
	}
	return bounds
}

func pointsBounds(pts []gfx.Point) gfx.Rect {
	if len(pts) == 0 {
		return gfx.Rect{}
	}
	bounds := gfx.Rect{Min: pts[0], Max: pts[0]}
	for _, p := range pts[1:] {
		if p.X < bounds.Min.X {
			bounds.Min.X = p.X
		}
		if p.Y < bounds.Min.Y {
			bounds.Min.Y = p.Y
		}
		if p.X > bounds.Max.X {
			bounds.Max.X = p.X
		}
		if p.Y > bounds.Max.Y {
			bounds.Max.Y = p.Y
		}
	}
	return bounds
}

func rectEqual(a, b gfx.Rect) bool {
	return a.Min == b.Min && a.Max == b.Max
}

func intersectRects(a, b gfx.Rect) gfx.Rect {
	if a.IsEmpty() || b.IsEmpty() {
		return gfx.Rect{}
	}
	minX := maxFloat32(a.Min.X, b.Min.X)
	minY := maxFloat32(a.Min.Y, b.Min.Y)
	maxX := minFloat32(a.Max.X, b.Max.X)
	maxY := minFloat32(a.Max.Y, b.Max.Y)
	if minX >= maxX || minY >= maxY {
		return gfx.Rect{}
	}
	return gfx.Rect{
		Min: gfx.Point{X: minX, Y: minY},
		Max: gfx.Point{X: maxX, Y: maxY},
	}
}

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func clearRGBA(img *image.RGBA) {
	for i := range img.Pix {
		img.Pix[i] = 0
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

func clampByte(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func mul255(a byte, b int) int {
	return (int(a)*b + 127) / 255
}

func scaleByte(v uint8, opacity float32) uint8 {
	if opacity <= 0 {
		return 0
	}
	if opacity >= 1 {
		return v
	}
	return uint8(math.Round(float64(float32(v) * opacity)))
}

var _ render.TextureBackend = (*SoftwareRenderer)(nil)

func (r *SoftwareRenderer) UploadTexture(req render.TextureUploadRequest) (render.TextureID, error) {
	if r.texBackend == nil {
		return 0, errors.New("software renderer: no texture backend")
	}
	return r.texBackend.UploadTexture(req)
}

func (r *SoftwareRenderer) FreeTexture(id render.TextureID) {
	if r.texBackend != nil {
		r.texBackend.FreeTexture(id)
	}
}

func (r *SoftwareRenderer) UploadBudgetBytesPerFrame() int {
	if r.texBackend == nil {
		return math.MaxInt
	}
	return r.texBackend.UploadBudgetBytesPerFrame()
}

func (r *SoftwareRenderer) TranscodeTarget() render.TextureFormat {
	if r.texBackend == nil {
		return render.TextureFormatRGBA8
	}
	return r.texBackend.TranscodeTarget()
}

func (r *SoftwareRenderer) drawTexture(target *image.RGBA, state renderState, cmd gfx.DrawTexture) {
	if r.texBackend == nil {
		return
	}
	pixels, w, h, ok := r.texBackend.GetTexture(render.TextureID(cmd.TextureID))
	if !ok || len(pixels) == 0 {
		return
	}

	dest := intersectRects(state.transform.TransformRect(cmd.DestRect), state.clip)
	if dest.IsEmpty() {
		return
	}

	srcRect := cmd.SrcRect
	if srcRect.IsEmpty() {
		srcRect = gfx.RectFromXYWH(0, 0, float32(w), float32(h))
	}

	minX := clampInt(int(math.Floor(float64(dest.Min.X))), 0, target.Bounds().Dx())
	minY := clampInt(int(math.Floor(float64(dest.Min.Y))), 0, target.Bounds().Dy())
	maxX := clampInt(int(math.Ceil(float64(dest.Max.X))), 0, target.Bounds().Dx())
	maxY := clampInt(int(math.Ceil(float64(dest.Max.Y))), 0, target.Bounds().Dy())
	if minX >= maxX || minY >= maxY {
		return
	}

	dstW := dest.Width()
	dstH := dest.Height()
	if dstW == 0 || dstH == 0 {
		return
	}

	srcW := srcRect.Width()
	srcH := srcRect.Height()

	for y := minY; y < maxY; y++ {
		ty := (float32(y) + 0.5 - dest.Min.Y) / dstH
		sy := srcRect.Min.Y + ty*srcH
		for x := minX; x < maxX; x++ {
			tx := (float32(x) + 0.5 - dest.Min.X) / dstW
			sx := srcRect.Min.X + tx*srcW

			px := clampInt(int(math.Round(float64(sx))), 0, int(w)-1)
			py := clampInt(int(math.Round(float64(sy))), 0, int(h)-1)
			off := (py*int(w) + px) * 4
			if off >= 0 && off+3 < len(pixels) {
				col := colorFromBytes(pixels[off], pixels[off+1], pixels[off+2], pixels[off+3])
				blendAt(target, x, y, col, state.opacity*cmd.Opacity)
			}
		}
	}
}
