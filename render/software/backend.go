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
	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/internal/renderutil"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/text"

	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	_ "golang.org/x/image/tiff"
	"golang.org/x/image/vector"
)

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

// tileRenderingInfo holds information for rendering a single tile
type tileRenderingInfo struct {
	// The tile coordinate
	coord tileCoord
	// The pixel rectangle this tile covers
	tileRect image.Rectangle
	// The surface rectangle this tile covers
	surfaceRect image.Rectangle
	// List of batch indices that intersect this tile
	batchIndices []int
}

type SoftwareRenderer struct {
	windowID   string
	windows    map[string]*SoftwareRenderer
	mu         sync.RWMutex
	surface    render.SoftwareSurface
	output     *image.RGBA
	texBackend *render.SoftwareBackend
	glyphAtlas *glyphAtlas
	width      int
	height     int

	// Tile-based rendering infrastructure
	// rasterStorage manages tile-based rendering and damage tracking
	rasterStorage *RasterStorage
	// tileCache caches rasterized tiles
	tileCache *TileCache
	// tileGrid tracks the tile grid for the current surface
	tileGrid *TileGrid

	strokeScratch  gfx.StrokeScratch
	rasterizeCount int
}

func NewSoftwareRenderer() *SoftwareRenderer {
	return &SoftwareRenderer{
		windowID:   render.PrimaryWindow,
		glyphAtlas: &glyphAtlas{entries: make(map[renderutil.GlyphAtlasKey]*glyphEntry)},
		texBackend: &render.SoftwareBackend{},
	}
}

func (r *SoftwareRenderer) Initialize(surface render.Surface) error {
	if surface == nil {
		return errors.New("software renderer: nil surface")
	}
	blit, ok := surface.(render.SoftwareSurface)
	if !ok {
		return errors.New("software renderer: surface must implement render.SoftwareSurface")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.surface = blit
	w, h := surface.Size()

	// Create tile grid
	r.tileGrid = NewTileGrid(w, h)

	// Create tile cache with a reasonable default configuration
	// This will be properly configured by the resource plan in a full implementation
	wallet, err := membudget.NewWallet(48 << 20) // 48 MiB for software raster
	if err != nil {
		return err
	}
	tileCache, err := NewTileCache(TileCacheConfig{
		Wallet:     wallet,
		MaxEntries: 16384, // 4096 for compact, 16384 for standard
	})
	if err != nil {
		return err
	}
	r.tileCache = tileCache

	// Create raster storage
	r.rasterStorage = NewRasterStorage(RasterStorageConfig{
		WindowID:  r.windowID,
		TileCache: tileCache,
		Width:     w,
		Height:    h,
	})

	r.allocateOutput(w, h)
	return nil
}

func (r *SoftwareRenderer) Resize(width, height int) error {
	if width < 0 || height < 0 {
		return errors.New("software renderer: invalid size")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	// Update tile grid
	r.tileGrid = NewTileGrid(width, height)

	// Resize raster storage
	if r.rasterStorage != nil {
		r.rasterStorage.Resize(width, height)
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
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, window := range r.windows {
		window.Destroy()
		delete(r.windows, id)
	}
	r.surface = nil
	r.output = nil
	r.width = 0
	r.height = 0
	r.glyphAtlas = &glyphAtlas{entries: make(map[renderutil.GlyphAtlasKey]*glyphEntry)}
	r.tileCache = nil
	r.rasterStorage = nil
	r.tileGrid = nil
}

// EvictCaches releases recoverable renderer caches without dropping the surface.
func (r *SoftwareRenderer) EvictCaches() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, window := range r.windows {
		window.EvictCaches()
	}
	r.glyphAtlas = &glyphAtlas{entries: make(map[renderutil.GlyphAtlasKey]*glyphEntry)}
	if r.tileCache != nil {
		r.tileCache.Clear()
	}
	if r.rasterStorage != nil {
		// Clear raster storage will be handled by tile cache eviction
	}
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

// calculateTileRenderingInfo determines which batches intersect each damaged tile
func (r *SoftwareRenderer) calculateTileRenderingInfo(damage *DamageSet, frame *render.Frame, surfaceSize image.Point) []tileRenderingInfo {
	if damage == nil || frame == nil {
		return nil
	}

	var result []tileRenderingInfo
	tileCoords := damage.Tiles()

	// For each damaged tile, find which batches intersect it
	for _, coord := range tileCoords {
		tileRect := coord.tileRect()
		// Clamp to surface bounds
		tileRect = tileRect.Intersect(image.Rect(0, 0, surfaceSize.X, surfaceSize.Y))
		if tileRect.Empty() {
			continue
		}

		info := tileRenderingInfo{
			coord:        coord,
			tileRect:     tileRect,
			surfaceRect:  tileRect,
			batchIndices: []int{},
		}

		// Find all batches that intersect this tile
		for i := 0; i < frame.BatchCount(); i++ {
			batch, ok := frame.BatchAt(i)
			if !ok {
				continue
			}
			batchRect := image.Rect(
				int(math.Floor(float64(batch.Bounds.Min.X))),
				int(math.Floor(float64(batch.Bounds.Min.Y))),
				int(math.Ceil(float64(batch.Bounds.Max.X))),
				int(math.Ceil(float64(batch.Bounds.Max.Y))),
			)
			if rectanglesIntersect(tileRect, batchRect) {
				info.batchIndices = append(info.batchIndices, i)
			}
		}

		if len(info.batchIndices) > 0 {
			result = append(result, info)
		}
	}

	return result
}

func (r *SoftwareRenderer) Submit(frame *render.Frame) error {
	if frame != nil && frame.WindowID() != r.windowID {
		return r.SubmitWindow(frame.WindowID(), frame)
	}
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

	// Begin frame in raster storage
	r.rasterStorage.BeginFrame(frame)

	// Get current damage
	damage := r.rasterStorage.Damage()

	// Check if we should do a full redraw
	shouldFullRedraw := ShouldFullRedraw(damage, r.tileGrid)

	if shouldFullRedraw {
		// Full redraw - render everything
		r.renderFullRedraw(frame)
	} else if !damage.IsEmpty() {
		// Partial redraw - render only damaged tiles
		r.renderPartialRedraw(frame, damage)
	} else {
		// No damage - check if we still need to render (e.g., first frame)
		// If there are batches but no damage, it might be the first frame
		if frame.BatchCount() > 0 {
			r.renderFullRedraw(frame)
		}
	}

	// End frame
	r.rasterStorage.EndFrame()

	if err := r.blitToSurface(); err != nil {
		return err
	}
	return nil
}

// renderFullRedraw renders the entire surface
func (r *SoftwareRenderer) renderFullRedraw(frame *render.Frame) {
	if r.output == nil || frame == nil {
		return
	}

	clearRGBA(r.output)

	clips := make([]gfx.Rect, frame.BatchCount())
	for li := 0; li < frame.LayerCount(); li++ {
		layer, ok := frame.LayerAt(li)
		if !ok {
			return
		}
		for i := layer.FirstBatch; i < layer.FirstBatch+layer.BatchCount; i++ {
			clips[i] = layer.ClipRect
		}
	}

	// For full redraw, we still need to render all batches
	// But we do it directly to the output, not through per-batch buffers
	for i := 0; i < frame.BatchCount(); i++ {
		batch, ok := frame.BatchAt(i)
		if !ok {
			return
		}

		// The frame clear above initializes the output once. Clearing a later
		// batch's bounds here would erase lower batches before alpha compositing;
		// render the canonical batch order directly over the existing pixels.
		r.rasterizeRenderBatchToOutput(r.output, frame, i, batch, clips[i])
		r.rasterizeCount++
	}
}

// renderPartialRedraw renders only damaged tiles
func (r *SoftwareRenderer) renderPartialRedraw(frame *render.Frame, damage *DamageSet) {
	if r.output == nil || frame == nil || damage == nil {
		return
	}

	surfaceSize := image.Point{X: r.width, Y: r.height}

	// Calculate which tiles need to be rendered and which batches affect them
	tileInfos := r.calculateTileRenderingInfo(damage, frame, surfaceSize)

	// For each damaged tile, render it to a temporary buffer then composite to output
	for _, info := range tileInfos {
		// Create a temporary buffer for this tile
		tileBuffer := image.NewRGBA(image.Rect(0, 0, info.tileRect.Dx(), info.tileRect.Dy()))
		clearRGBA(tileBuffer)

		// Render all batches that intersect this tile
		for _, batchIdx := range info.batchIndices {
			batch, ok := frame.BatchAt(batchIdx)
			if !ok {
				continue
			}

			// Get the clip for this batch
			clip := r.getBatchClip(frame, batchIdx)

			// Render the batch to the tile buffer
			// We need to transform the batch to tile-local coordinates
			r.rasterizeRenderBatchToTile(tileBuffer, frame, batchIdx, batch, clip, info.tileRect)
		}

		// Composite the tile buffer to the output
		r.compositeTileToOutput(r.output, tileBuffer, info.surfaceRect)

		r.rasterizeCount++
		_ = tileBuffer
	}
}

// getBatchClip returns the clip rectangle for a batch
func (r *SoftwareRenderer) getBatchClip(frame *render.Frame, batchIndex int) gfx.Rect {
	for li := 0; li < frame.LayerCount(); li++ {
		layer, ok := frame.LayerAt(li)
		if !ok {
			return gfx.Rect{}
		}
		if batchIndex >= layer.FirstBatch && batchIndex < layer.FirstBatch+layer.BatchCount {
			return layer.ClipRect
		}
	}
	return gfx.Rect{}
}

// rasterizeRenderBatchToOutput renders a batch directly to the output image
func (r *SoftwareRenderer) rasterizeRenderBatchToOutput(target *image.RGBA, frame *render.Frame, batchIndex int, batch render.FrameBatch, clip gfx.Rect) {
	if target == nil || frame == nil {
		return
	}

	// Calculate the batch bounds in pixel coordinates
	minX := int(math.Floor(float64(batch.Bounds.Min.X)))
	minY := int(math.Floor(float64(batch.Bounds.Min.Y)))
	maxX := int(math.Ceil(float64(batch.Bounds.Max.X)))
	maxY := int(math.Ceil(float64(batch.Bounds.Max.Y)))

	// Clamp to target bounds
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX > target.Bounds().Dx() {
		maxX = target.Bounds().Dx()
	}
	if maxY > target.Bounds().Dy() {
		maxY = target.Bounds().Dy()
	}

	batchPixelRect := image.Rect(minX, minY, maxX, maxY)

	state := renderState{
		transform: gfx.Identity(),
		clip: gfx.RectFromXYWH(
			float32(batchPixelRect.Min.X),
			float32(batchPixelRect.Min.Y),
			float32(batchPixelRect.Dx()),
			float32(batchPixelRect.Dy()),
		),
		opacity: batch.Opacity,
	}
	if !clip.IsEmpty() {
		state.clip = intersectRects(state.clip, clip)
	}
	stack := []renderState{state}

	frame.VisitBatchCommands(batchIndex, func(cmd gfx.CommandView) bool {
		state = stack[len(stack)-1]
		switch c := cmd.Scalars().(type) {
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
			fillRect(target, state, c.Rect, cmd.Brush())
		case gfx.StrokeRect, gfx.StrokePath, gfx.DrawPolyline:
			segments := cmd.ExpandStroke(&r.strokeScratch)
			rasterizePath(target, state, gfx.ReadPath(gfx.Path{Segments: segments}), cmd.Brush(), 1)
		case gfx.FillPath:
			rasterizePath(target, state, cmd.Path(), cmd.Brush(), 1)
		case gfx.DrawPoints:
			drawPoints(target, state, cmd.Points(), c.Radius, cmd.Brush())
		case gfx.DrawGlyphRun:
			r.drawGlyphRun(target, state, c, cmd)
		case gfx.DrawSelectionRects:
			for rr := range cmd.Rects().All() {
				fillRect(target, state, rr, cmd.Brush())
			}
		case gfx.DrawImage:
			drawImage(target, state, c)
		case gfx.DrawTexture:
			r.drawTexture(target, state, c)
		case gfx.DrawBlurredShadow:
			drawBlurredShadow(target, state, c, cmd.Path())
		case gfx.BeginRenderBatch, gfx.EndRenderBatch:
		}
		return true
	})
}

// rasterizeRenderBatchToTile renders a batch to a tile buffer with coordinate transformation
func (r *SoftwareRenderer) rasterizeRenderBatchToTile(target *image.RGBA, frame *render.Frame, batchIndex int, batch render.FrameBatch, clip gfx.Rect, tileRect image.Rectangle) {
	if target == nil || frame == nil {
		return
	}

	state := renderState{
		transform: gfx.Translation(-float32(tileRect.Min.X), -float32(tileRect.Min.Y)),
		clip:      gfx.RectFromXYWH(0, 0, float32(target.Bounds().Dx()), float32(target.Bounds().Dy())),
		opacity:   batch.Opacity,
	}

	if !clip.IsEmpty() {
		clipInTile := gfx.Rect{
			Min: gfx.Point{
				X: clip.Min.X - float32(tileRect.Min.X),
				Y: clip.Min.Y - float32(tileRect.Min.Y),
			},
			Max: gfx.Point{
				X: clip.Max.X - float32(tileRect.Min.X),
				Y: clip.Max.Y - float32(tileRect.Min.Y),
			},
		}
		state.clip = intersectRects(state.clip, clipInTile)
	}

	stack := []renderState{state}

	frame.VisitBatchCommands(batchIndex, func(cmd gfx.CommandView) bool {
		state = stack[len(stack)-1]
		switch c := cmd.Scalars().(type) {
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
			fillRect(target, state, c.Rect, cmd.Brush())
		case gfx.StrokeRect, gfx.StrokePath, gfx.DrawPolyline:
			segments := cmd.ExpandStroke(&r.strokeScratch)
			rasterizePath(target, state, gfx.ReadPath(gfx.Path{Segments: segments}), cmd.Brush(), 1)
		case gfx.FillPath:
			rasterizePath(target, state, cmd.Path(), cmd.Brush(), 1)
		case gfx.DrawPoints:
			drawPoints(target, state, cmd.Points(), c.Radius, cmd.Brush())
		case gfx.DrawGlyphRun:
			r.drawGlyphRun(target, state, c, cmd)
		case gfx.DrawSelectionRects:
			for rr := range cmd.Rects().All() {
				fillRect(target, state, rr, cmd.Brush())
			}
		case gfx.DrawImage:
			drawImage(target, state, c)
		case gfx.DrawTexture:
			r.drawTexture(target, state, c)
		case gfx.DrawBlurredShadow:
			drawBlurredShadow(target, state, c, cmd.Path())
		case gfx.BeginRenderBatch, gfx.EndRenderBatch:
		}
		return true
	})
}

// compositeTileToOutput composites a tile buffer to the output at the specified rectangle
func (r *SoftwareRenderer) compositeTileToOutput(dst *image.RGBA, src *image.RGBA, dstRect image.Rectangle) {
	if dst == nil || src == nil || dstRect.Empty() {
		return
	}

	// Simple composite: copy tile to destination
	for y := 0; y < src.Bounds().Dy(); y++ {
		srcY := src.Bounds().Min.Y + y
		dstY := dstRect.Min.Y + y
		if dstY < 0 || dstY >= dst.Bounds().Dy() {
			continue
		}
		for x := 0; x < src.Bounds().Dx(); x++ {
			srcX := src.Bounds().Min.X + x
			dstX := dstRect.Min.X + x
			if dstX < 0 || dstX >= dst.Bounds().Dx() {
				continue
			}
			srcOff := srcY*src.Stride + srcX*4
			dstOff := dstY*dst.Stride + dstX*4
			if srcOff+4 <= len(src.Pix) && dstOff+4 <= len(dst.Pix) {
				// Alpha composite
				copy(dst.Pix[dstOff:dstOff+4], src.Pix[srcOff:srcOff+4])
			}
		}
	}
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

func fillRect(target *image.RGBA, state renderState, rect gfx.Rect, brush gfx.BrushView) {
	if state.transform.B != 0 || state.transform.C != 0 {
		// Non-axis-preserving transform (rotation/shear): the rect becomes a
		// polygon. Rasterize it as a filled contour so rotated rects render as
		// the rotated shape rather than their axis-aligned bounding box.
		rasterizePath(target, state, rectAsPath(rect), brush, 1)
		return
	}
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

	// Solid-brush fast path: an opaque solid fill is a straight overwrite with
	// a constant byte pattern — hoisting colorToBytes out of the per-pixel loop
	// avoids the float round-trip on every pixel (the software backend's hottest
	// fill case: surface + material fills dominate studio frames).
	if brush.Kind == gfx.BrushSolid && state.opacity == 1 {
		sr, sg, sb, sa := colorToBytes(brush.Color, 1)
		if sa == 255 {
			for y := minY; y < maxY; y++ {
				rowOff := y*target.Stride + minX*4
				for x := minX; x < maxX; x++ {
					off := rowOff + (x-minX)*4
					target.Pix[off] = sr
					target.Pix[off+1] = sg
					target.Pix[off+2] = sb
					target.Pix[off+3] = 255
				}
			}
			return
		}
		// Translucent solid: precompute bytes once, still constant per pixel.
		var packed [4]byte
		packed[0], packed[1], packed[2], packed[3] = sr, sg, sb, sa
		for y := minY; y < maxY; y++ {
			rowOff := y*target.Stride + minX*4
			for x := minX; x < maxX; x++ {
				off := rowOff + (x-minX)*4
				blendPremul(target.Pix[off:off+4], packed[:], 1)
			}
		}
		return
	}

	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			c := sampleBrush(brush, gfx.Point{X: float32(x), Y: float32(y)}, rr)
			blendAt(target, x, y, c, state.opacity)
		}
	}
}

// rectAsPath expresses a rect as a closed four-corner contour, used by
// fillRect when the transform is non-axis-preserving.
func rectAsPath(rect gfx.Rect) gfx.PathView {
	corners := []gfx.Point{
		rect.Min,
		{X: rect.Max.X, Y: rect.Min.Y},
		rect.Max,
		{X: rect.Min.X, Y: rect.Max.Y},
	}
	return gfx.ReadPath(gfx.Path{Segments: []gfx.PathSegment{
		{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{corners[0]}},
		{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{corners[1]}},
		{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{corners[2]}},
		{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{corners[3]}},
		{Verb: gfx.PathClose},
	}})
}

func drawPoints(target *image.RGBA, state renderState, pts gfx.Values[gfx.Point], radius float32, brush gfx.BrushView) {
	if pts.Len() == 0 || radius <= 0 {
		return
	}
	for p := range pts.All() {
		fillRect(target, state, gfx.RectFromXYWH(p.X-radius, p.Y-radius, radius*2, radius*2), brush)
	}
}

// drawBlurredShadow renders a soft shadow (Slice 9) with feature parity to the
// GPU blur pipeline: the path is rasterized to an 8-bit coverage mask over the
// path's bounds expanded by `round(blurRadius)` (integer-aligned so the mask
// array indices coincide with the GPU's texel grid), blurred by a separable
// Gaussian (sigma = radius/3, kernel = exp(-i^2 / 2 sigma^2) normalized), and
// composited tinted with the premultiplied shadow color at the offset, clipped
// to the active clip rect. `Inner` inverts the mask source before blurring.
func drawBlurredShadow(target *image.RGBA, state renderState, cmd gfx.DrawBlurredShadow, path gfx.PathView) {
	if target == nil || path.Segments.Len() == 0 || cmd.Color.A <= 0 {
		return
	}

	var transformed gfx.Path
	transformed.Segments = make([]gfx.PathSegment, 0, path.Segments.Len())
	for seg := range path.Segments.All() {
		out := gfx.PathSegment{Verb: seg.Verb}
		for i := range seg.Pts {
			out.Pts[i] = state.transform.TransformPoint(seg.Pts[i])
		}
		transformed.Segments = append(transformed.Segments, out)
	}

	pb := pathBounds(gfx.ReadPath(transformed))
	if pb.IsEmpty() {
		return
	}

	radius := int(math.Round(float64(cmd.BlurRadius)))
	if radius < 0 {
		radius = 0
	}

	rMinX := int(math.Floor(float64(pb.Min.X))) - radius
	rMinY := int(math.Floor(float64(pb.Min.Y))) - radius
	rMaxX := int(math.Ceil(float64(pb.Max.X))) + radius
	rMaxY := int(math.Ceil(float64(pb.Max.Y))) + radius
	rw := rMaxX - rMinX
	rh := rMaxY - rMinY
	if rw <= 0 || rh <= 0 {
		return
	}

	mask := image.NewAlpha(image.Rect(0, 0, rw, rh))
	ras := vector.NewRasterizer(rw, rh)
	ras.DrawOp = draw.Src
	for _, seg := range transformed.Segments {
		switch seg.Verb {
		case gfx.PathMoveTo:
			ras.MoveTo(seg.Pts[0].X-float32(rMinX), seg.Pts[0].Y-float32(rMinY))
		case gfx.PathLineTo:
			ras.LineTo(seg.Pts[0].X-float32(rMinX), seg.Pts[0].Y-float32(rMinY))
		case gfx.PathQuadTo:
			ras.QuadTo(
				seg.Pts[0].X-float32(rMinX), seg.Pts[0].Y-float32(rMinY),
				seg.Pts[1].X-float32(rMinX), seg.Pts[1].Y-float32(rMinY),
			)
		case gfx.PathCubicTo:
			ras.CubeTo(
				seg.Pts[0].X-float32(rMinX), seg.Pts[0].Y-float32(rMinY),
				seg.Pts[1].X-float32(rMinX), seg.Pts[1].Y-float32(rMinY),
				seg.Pts[2].X-float32(rMinX), seg.Pts[2].Y-float32(rMinY),
			)
		case gfx.PathClose:
			ras.ClosePath()
		}
	}
	ras.Draw(mask, mask.Bounds(), image.NewUniform(color.Alpha{A: 255}), image.Point{})

	src := make([]float32, rw*rh)
	for i := 0; i < rw*rh; i++ {
		cov := float32(mask.Pix[i]) / 255
		if cmd.Inner {
			cov = 1 - cov
		}
		src[i] = cov
	}

	if radius > 0 {
		sigma := float64(cmd.BlurRadius) / 3.0
		if sigma <= 0 {
			sigma = 1
		}
		inv2s := 1.0 / (2.0 * sigma * sigma)
		weights := make([]float64, 2*radius+1)
		total := 0.0
		for i := -radius; i <= radius; i++ {
			w := math.Exp(-float64(i*i) * inv2s)
			weights[i+radius] = w
			total += w
		}
		for i := range weights {
			weights[i] /= total
		}

		// Horizontal pass.
		tmp := make([]float32, rw*rh)
		for y := 0; y < rh; y++ {
			base := y * rw
			for x := 0; x < rw; x++ {
				acc := 0.0
				for i := -radius; i <= radius; i++ {
					xi := x + i
					if xi < 0 || xi >= rw {
						continue
					}
					acc += weights[i+radius] * float64(src[base+xi])
				}
				tmp[base+x] = float32(acc)
			}
		}
		// Vertical pass into src.
		for y := 0; y < rh; y++ {
			for x := 0; x < rw; x++ {
				acc := 0.0
				for j := -radius; j <= radius; j++ {
					yj := y + j
					if yj < 0 || yj >= rh {
						continue
					}
					acc += weights[j+radius] * float64(tmp[yj*rw+x])
				}
				src[y*rw+x] = float32(acc)
			}
		}
	}

	ox := int(math.Round(float64(cmd.Offset.X)))
	oy := int(math.Round(float64(cmd.Offset.Y)))
	clipMinX := clampInt(int(math.Floor(float64(state.clip.Min.X))), 0, target.Bounds().Dx())
	clipMinY := clampInt(int(math.Floor(float64(state.clip.Min.Y))), 0, target.Bounds().Dy())
	clipMaxX := clampInt(int(math.Ceil(float64(state.clip.Max.X))), 0, target.Bounds().Dx())
	clipMaxY := clampInt(int(math.Ceil(float64(state.clip.Max.Y))), 0, target.Bounds().Dy())
	for y := 0; y < rh; y++ {
		py := rMinY + y + oy
		if py < clipMinY || py >= clipMaxY {
			continue
		}
		for x := 0; x < rw; x++ {
			a := src[y*rw+x]
			if a <= 0 {
				continue
			}
			px := rMinX + x + ox
			if px < clipMinX || px >= clipMaxX {
				continue
			}
			blendAt(target, px, py, cmd.Color, state.opacity*a)
		}
	}
}

func (r *SoftwareRenderer) drawGlyphRun(target *image.RGBA, state renderState, cmd gfx.DrawGlyphRun, view gfx.CommandView) {
	if target == nil || view.Glyphs().Len() == 0 {
		return
	}
	atlas := r.glyphAtlas
	if atlas == nil {
		return
	}
	for glyph := range view.Glyphs().All() {
		entry := atlas.getOrRasterize(cmd.Run, glyph)
		if entry == nil || entry.bitmap == nil {
			continue
		}
		pos := state.transform.TransformPoint(gfx.Point{
			X: cmd.Origin.X + glyph.X + entry.offsetX,
			Y: cmd.Origin.Y + glyph.Y + entry.offsetY,
		})
		drawGlyphBitmap(target, state, entry, pos, view.Brush())
	}
}

func rasterizePath(target *image.RGBA, state renderState, path gfx.PathView, brush gfx.BrushView, opacity float32) {
	if target == nil || path.Segments.Len() == 0 {
		return
	}

	var transformed gfx.Path
	transformed.Segments = make([]gfx.PathSegment, 0, path.Segments.Len())
	for seg := range path.Segments.All() {
		out := gfx.PathSegment{Verb: seg.Verb}
		for i := range seg.Pts {
			out.Pts[i] = state.transform.TransformPoint(seg.Pts[i])
		}
		transformed.Segments = append(transformed.Segments, out)
	}

	bounds := pathBounds(gfx.ReadPath(transformed))
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

func drawGlyphBitmap(target *image.RGBA, state renderState, entry *glyphEntry, pos gfx.Point, brush gfx.BrushView) {
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
	if cmd.Image.IsZero() {
		return
	}
	dest := intersectRects(state.transform.TransformRect(cmd.DestRect), state.clip)
	if dest.IsEmpty() {
		return
	}

	srcRect := cmd.SrcRect
	if srcRect.IsEmpty() {
		srcRect = gfx.RectFromXYWH(0, 0, float32(cmd.Image.Width()), float32(cmd.Image.Height()))
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

func sampleBrush(brush gfx.BrushView, p gfx.Point, bounds gfx.Rect) gfx.Color {
	switch brush.Kind {
	case gfx.BrushSolid:
		return brush.Color
	case gfx.BrushLinearGradient:
		return sampleLinearGradient(brush, p)
	default:
		return gfx.Color{}
	}
}

func sampleLinearGradient(brush gfx.BrushView, p gfx.Point) gfx.Color {
	stops := brush.GradientStops
	if stops.Len() == 0 {
		return gfx.Color{}
	}
	if stops.Len() == 1 {
		return stops.At(0).Color
	}
	dx := brush.GradientEnd.X - brush.GradientStart.X
	dy := brush.GradientEnd.Y - brush.GradientStart.Y
	denom := dx*dx + dy*dy
	if denom == 0 {
		return stops.At(stops.Len() - 1).Color
	}
	t := ((p.X-brush.GradientStart.X)*dx + (p.Y-brush.GradientStart.Y)*dy) / denom
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	left := stops.At(0)
	right := stops.At(stops.Len() - 1)
	for i := 0; i < stops.Len()-1; i++ {
		if t >= stops.At(i).Offset && t <= stops.At(i+1).Offset {
			left = stops.At(i)
			right = stops.At(i + 1)
			break
		}
	}

	if right.Offset == left.Offset {
		return right.Color
	}
	f := (t - left.Offset) / (right.Offset - left.Offset)
	return lerpColor(left.Color, right.Color, f)
}

func sampleImageNearest(img gfx.ImageHandle, sx, sy float32) gfx.Color {
	x := clampInt(int(math.Round(float64(sx))), 0, img.Width()-1)
	y := clampInt(int(math.Round(float64(sy))), 0, img.Height()-1)
	r, g, b, a, ok := img.PixelAt(x, y)
	if !ok {
		return gfx.Color{}
	}
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
	// Stack-only [4]byte: the previous `[]byte{sr,...}` composite literal
	// escaped to a tiny heap allocation on every pixel — a measurable cost in
	// the software-blend hot path (and ~2x under -race shadow memory).
	var packed [4]byte
	packed[0], packed[1], packed[2], packed[3] = sr, sg, sb, sa
	blendPremul(img.Pix[off:off+4], packed[:], 1)
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
	if sr == 0 && sg == 0 && sb == 0 && sa == 0 {
		// Fully-transparent premultiplied source: no-op. NOTE: this is NOT a
		// bare `sa == 0` check — the reference formula for alpha 0 with a
		// nonzero RGB channel (a malformed premultiplied color) reduces to
		// dst' = clamp(src + dst), which is not identity. Valid premultiplied
		// sources always have RGB == 0 when A == 0, so the all-zero check is
		// byte-identical to the reference for canonical colors and matches it
		// for malformed ones too.
		return
	}
	inv := int(255 - sa)
	if inv == 0 {
		// Opaque source with full coverage overwrites: with sa == 255 the
		// general clause below reduces to dst' = sr, sg, sb and A' = 255
		// exactly. Direct copy is byte-identical and skip mul255/clamp.
		dst[0] = sr
		dst[1] = sg
		dst[2] = sb
		dst[3] = 255
		return
	}
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

func pathBounds(path gfx.PathView) gfx.Rect {
	var bounds gfx.Rect
	first := true
	for seg := range path.Segments.All() {
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
	// clear() zeroes in bulk (memset primitive); the previous per-byte loop
	// was one instrumented store per pixel, which the race detector makes 2x+
	// more expensive than the runtime's bulk zero.
	clear(img.Pix)
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
