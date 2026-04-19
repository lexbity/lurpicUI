package software

import (
	"errors"
	"image"
	"image/color"
	"math"
	"sync"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/renderutil"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/text"
)

var _ = text.GlyphRun{}

type blitSurface interface {
	render.Surface
	Buffer() []byte
	Stride() int
	Lock() error
	Unlock([]gfx.Rect) error
}

type layerCacheEntry struct {
	bounds      gfx.Rect
	commandHash uint64
	buffer      *image.RGBA
}

type glyphKey struct {
	glyphID  uint32
	faceKey  uint64
	sizeBits uint32
}

type glyphEntry struct {
	bitmap  *image.Alpha
	offsetX float32
	offsetY float32
}

type glyphAtlas struct {
	mu             sync.Mutex
	entries        map[glyphKey]*glyphEntry
	rasterizeCount int
}

type renderState struct {
	transform gfx.Transform
	clip      gfx.Rect
	opacity   float32
}

type SoftwareRenderer struct {
	surface blitSurface
	output  *image.RGBA

	layerCache map[render.LayerID]*layerCacheEntry
	diffCache  *renderutil.LayerCache
	glyphAtlas *glyphAtlas
	width      int
	height     int

	rasterizeCount int
}

func NewSoftwareRenderer() *SoftwareRenderer {
	return &SoftwareRenderer{
		layerCache: make(map[render.LayerID]*layerCacheEntry),
		diffCache:  renderutil.NewLayerCache(),
		glyphAtlas: &glyphAtlas{entries: make(map[glyphKey]*glyphEntry)},
	}
}

func (r *SoftwareRenderer) Initialize(surface render.Surface) error {
	if surface == nil {
		return errors.New("software renderer: nil surface")
	}
	blit, ok := surface.(blitSurface)
	if !ok {
		return errors.New("software renderer: surface must implement platform.Surface")
	}

	r.surface = blit
	w, h := surface.Size()
	r.allocateOutput(w, h)
	return nil
}

func (r *SoftwareRenderer) Resize(width, height int) error {
	if width < 0 || height < 0 {
		return errors.New("software renderer: invalid size")
	}
	r.allocateOutput(width, height)
	if r.surface != nil {
		if resizable, ok := any(r.surface).(interface{ Resize(int, int) }); ok {
			resizable.Resize(width, height)
		}
	}
	return nil
}

func (r *SoftwareRenderer) Destroy() {
	r.surface = nil
	r.output = nil
	r.width = 0
	r.height = 0
	r.layerCache = make(map[render.LayerID]*layerCacheEntry)
	r.diffCache = renderutil.NewLayerCache()
	r.glyphAtlas = &glyphAtlas{entries: make(map[glyphKey]*glyphEntry)}
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

	seen := make(map[render.LayerID]struct{}, len(frame.Layers))
	for _, layer := range frame.Layers {
		seen[layer.ID] = struct{}{}
		ld := diff.Layers[layer.ID]
		if ld.Kind == renderutil.LayerUnchanged {
			if entry := r.layerCache[layer.ID]; entry != nil && entry.buffer != nil {
				r.compositeLayer(r.output, entry.buffer, layer.Bounds.Min, layer.Opacity)
			}
			continue
		}
		if ld.Kind == renderutil.LayerRemoved {
			delete(r.layerCache, layer.ID)
			continue
		}

		sizeW := int(math.Ceil(float64(layer.Bounds.Width())))
		sizeH := int(math.Ceil(float64(layer.Bounds.Height())))
		if sizeW < 0 {
			sizeW = 0
		}
		if sizeH < 0 {
			sizeH = 0
		}

		buffer := image.NewRGBA(image.Rect(0, 0, sizeW, sizeH))
		r.rasterizeLayer(buffer, &layer)
		r.rasterizeCount++
		r.layerCache[layer.ID] = &layerCacheEntry{
			bounds:      layer.Bounds,
			commandHash: layer.CommandHash,
			buffer:      buffer,
		}
		r.compositeLayer(r.output, buffer, layer.Bounds.Min, layer.Opacity)
	}

	for id := range r.layerCache {
		if _, ok := seen[id]; !ok {
			delete(r.layerCache, id)
		}
	}

	if err := r.blitToSurface(); err != nil {
		return err
	}

	buffers := make(map[render.LayerID]*image.RGBA, len(r.layerCache))
	for id, entry := range r.layerCache {
		buffers[id] = entry.buffer
	}
	r.diffCache.Update(frame, buffers)
	return nil
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
	if stride <= 0 {
		stride = r.width * 4
	}
	for y := 0; y < r.height; y++ {
		srcOff := y * r.output.Stride
		dstOff := y * stride
		copy(dst[dstOff:dstOff+r.width*4], r.output.Pix[srcOff:srcOff+r.width*4])
	}
	return nil
}

func (r *SoftwareRenderer) compositeLayer(dst *image.RGBA, src *image.RGBA, offset gfx.Point, opacity float32) {
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

func (r *SoftwareRenderer) rasterizeLayer(target *image.RGBA, layer *render.Layer) {
	if target == nil || layer == nil {
		return
	}
	state := renderState{
		transform: gfx.Identity(),
		clip:      gfx.RectFromXYWH(0, 0, float32(target.Bounds().Dx()), float32(target.Bounds().Dy())),
		opacity:   1,
	}
	stack := []renderState{state}

	for _, cmd := range layer.Commands.Commands {
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
		case gfx.BeginLayer, gfx.EndLayer:
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
	bounds := pathBounds(path)
	if bounds.IsEmpty() {
		return
	}
	fillRect(target, state, bounds, brush)
}

func strokePath(target *image.RGBA, state renderState, path gfx.Path, stroke gfx.StrokeStyle, brush gfx.Brush) {
	bounds := pathBounds(path)
	if bounds.IsEmpty() {
		return
	}
	fillRect(target, state, bounds.Inset(-stroke.Width/2, -stroke.Width/2), brush)
}

func drawPolyline(target *image.RGBA, state renderState, pts []gfx.Point, stroke gfx.StrokeStyle, brush gfx.Brush, closed bool) {
	if len(pts) == 0 {
		return
	}
	bounds := pointsBounds(pts)
	if bounds.IsEmpty() {
		return
	}
	fillRect(target, state, bounds.Inset(-stroke.Width/2, -stroke.Width/2), brush)
	_ = closed
}

func drawPoints(target *image.RGBA, state renderState, pts []gfx.Point, radius float32, brush gfx.Brush) {
	if len(pts) == 0 || radius <= 0 {
		return
	}
	for _, p := range pts {
		fillRect(target, state, gfx.RectFromXYWH(p.X-radius, p.Y-radius, radius*2, radius*2), brush)
	}
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
		drawGlyphBitmap(target, state, entry.bitmap, pos, cmd.Brush)
	}
}

func (a *glyphAtlas) getOrRasterize(run text.GlyphRun, glyph text.PositionedGlyph) *glyphEntry {
	if a == nil {
		return nil
	}
	size := run.Size
	if size <= 0 {
		size = run.Style.Size
	}
	if size <= 0 {
		size = 14
	}
	key := glyphKey{
		glyphID:  glyph.GlyphID,
		faceKey:  run.Face.CacheKey(),
		sizeBits: math.Float32bits(size),
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if entry := a.entries[key]; entry != nil {
		return entry
	}
	entry := rasterizeGlyphEntry(key, size)
	a.entries[key] = entry
	a.rasterizeCount++
	return entry
}

func rasterizeGlyphEntry(key glyphKey, size float32) *glyphEntry {
	w := maxInt(1, int(math.Ceil(float64(size*0.55))))
	h := maxInt(1, int(math.Ceil(float64(size*0.9))))
	if w < 3 {
		w = 3
	}
	if h < 4 {
		h = 4
	}
	bmp := image.NewAlpha(image.Rect(0, 0, w, h))
	seed := uint32(key.glyphID) ^ key.sizeBits ^ uint32(key.faceKey) ^ (uint32(key.glyphID)<<7 | uint32(key.faceKey>>32))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			on := x == 0 || y == 0 || x == w-1 || y == h-1 || x == y%w
			if !on {
				v := (uint32(x)*31 + uint32(y)*17 + seed) % 9
				on = v == 0 || (seed>>uint((x+y)%16))&1 == 1 && (x+y)%3 == 0
			}
			if on {
				// Slightly vary the alpha so different glyphs do not all look identical.
				alpha := uint8(200 + (seed+uint32(x*13+y*7))%55)
				bmp.SetAlpha(x, y, color.Alpha{A: alpha})
			}
		}
	}
	return &glyphEntry{bitmap: bmp}
}

func drawGlyphBitmap(target *image.RGBA, state renderState, bmp *image.Alpha, pos gfx.Point, brush gfx.Brush) {
	if target == nil || bmp == nil {
		return
	}
	ox := int(math.Round(float64(pos.X)))
	oy := int(math.Round(float64(pos.Y)))
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
		count := 0
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
