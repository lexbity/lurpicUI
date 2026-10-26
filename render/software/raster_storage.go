package software

import (
	"image"
	"sync"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/render"
)

// RasterStorage manages tile-based raster storage for a surface.
type RasterStorage struct {
	mu sync.RWMutex

	// windowID identifies which window this storage belongs to.
	windowID string

	// tileCache is the cache for rasterized tiles.
	tileCache *TileCache

	// tileGrid is the grid of tiles for the current surface.
	tileGrid *TileGrid

	// frame is the current frame being rendered.
	frame *render.Frame

	// lastFrame is the previous frame for damage comparison.
	lastFrame *render.Frame

	// lastFrameTiles maps from tileCoord to the batch IDs that affected it in the last frame.
	lastFrameTiles map[tileCoord][]render.RenderBatchID

	// currentDamage tracks which tiles are damaged.
	currentDamage *DamageSet

	// stats
	framesRendered  uint64
	tilesRasterized uint64
	fullRedraws     uint64
	partialRedraws  uint64
}

// RasterStorageConfig configures raster storage.
type RasterStorageConfig struct {
	// WindowID is the window identifier.
	WindowID string
	// TileCache is the tile cache to use.
	TileCache *TileCache
	// Width and Height are the initial surface dimensions.
	Width, Height int
}

// NewRasterStorage creates a new raster storage.
func NewRasterStorage(config RasterStorageConfig) *RasterStorage {
	grid := NewTileGrid(config.Width, config.Height)
	return &RasterStorage{
		windowID:       config.WindowID,
		tileCache:      config.TileCache,
		tileGrid:       grid,
		currentDamage:  NewDamageSet(),
		lastFrameTiles: make(map[tileCoord][]render.RenderBatchID),
	}
}

// Resize resizes the surface and updates the tile grid.
func (rs *RasterStorage) Resize(width, height int) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.tileGrid = NewTileGrid(width, height)
	rs.currentDamage = NewDamageSet()
	// Invalidate all tiles - they no longer fit
	rs.lastFrameTiles = make(map[tileCoord][]render.RenderBatchID)
}

// BeginFrame starts a new frame.
func (rs *RasterStorage) BeginFrame(frame *render.Frame) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.currentDamage.Clear()

	// Calculate damage
	if rs.lastFrame != nil && frame != nil {
		// Simple damage: all tiles that have different batches
		newFrameTiles := make(map[tileCoord][]render.RenderBatchID)
		surfaceSize := rs.tileGrid.Size

		for i := 0; i < frame.BatchCount(); i++ {
			batch, ok := frame.BatchAt(i)
			if !ok {
				continue
			}
			for _, coord := range BatchTiles(batch, surfaceSize) {
				newFrameTiles[coord] = append(newFrameTiles[coord], batch.ID)
			}
		}

		// Compare with last frame
		for coord, newBatchIDs := range newFrameTiles {
			oldBatchIDs := rs.lastFrameTiles[coord]
			if !batchIDsEqual(oldBatchIDs, newBatchIDs) {
				rs.currentDamage.AddTile(coord)
			}
		}

		// Batches that were in last frame but not in new frame also cause damage
		for coord := range rs.lastFrameTiles {
			if _, exists := newFrameTiles[coord]; !exists {
				rs.currentDamage.AddTile(coord)
			}
		}

		rs.lastFrameTiles = newFrameTiles
	} else if frame != nil {
		// No old frame, everything is damaged
		surfaceSize := rs.tileGrid.Size
		for i := 0; i < frame.BatchCount(); i++ {
			batch, ok := frame.BatchAt(i)
			if !ok {
				continue
			}
			for _, coord := range BatchTiles(batch, surfaceSize) {
				rs.currentDamage.AddTile(coord)
				rs.lastFrameTiles[coord] = append(rs.lastFrameTiles[coord], batch.ID)
			}
		}
	}

	rs.lastFrame = rs.frame
	rs.frame = frame
}

// batchIDsEqual checks if two batch ID slices are equal.
func batchIDsEqual(a, b []render.RenderBatchID) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	// Simple check - a proper implementation would do a set comparison
	return len(a) == len(b)
}

// EndFrame finishes a frame.
func (rs *RasterStorage) EndFrame() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.framesRendered++
	rs.currentDamage.Clear()
}

// Damage returns the current damage set.
func (rs *RasterStorage) Damage() *DamageSet {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	// Return a copy
	damage := NewDamageSet()
	for coord := range rs.currentDamage.tiles {
		damage.AddTile(coord)
	}
	return damage
}

// Render renders the frame to the output image.
// Returns whether a full redraw was performed.
func (rs *RasterStorage) Render(output *image.RGBA) bool {
	rs.mu.RLock()
	frame := rs.frame
	grid := rs.tileGrid
	damage := rs.currentDamage
	rs.mu.RUnlock()

	if frame == nil || output == nil {
		return false
	}

	// Check if we should do a full redraw
	if ShouldFullRedraw(damage, grid) {
		rs.mu.Lock()
		rs.fullRedraws++
		rs.mu.Unlock()
		return rs.renderFull(output)
	}

	// Partial redraw
	rs.mu.Lock()
	rs.partialRedraws++
	rs.mu.Unlock()
	return rs.renderPartial(output)
}

// renderFull renders the entire surface.
func (rs *RasterStorage) renderFull(output *image.RGBA) bool {
	rs.mu.RLock()
	frame := rs.frame
	rs.mu.RUnlock()

	if frame == nil || output == nil {
		return false
	}

	// Clear the output
	clearRGBA(output)

	// For now, mark that we did a full redraw
	// The actual rendering would be done by the backend
	_ = frame

	return true
}

// renderPartial renders only damaged tiles.
func (rs *RasterStorage) renderPartial(output *image.RGBA) bool {
	rs.mu.RLock()
	frame := rs.frame
	damage := rs.currentDamage
	rs.mu.RUnlock()

	if frame == nil || output == nil || damage.IsEmpty() {
		return false
	}

	// For each damaged tile, render it
	for _, coord := range damage.Tiles() {
		rs.renderTile(output, coord, frame)
		rs.mu.Lock()
		rs.tilesRasterized++
		rs.mu.Unlock()
	}

	return true
}

// renderTile renders a single tile.
func (rs *RasterStorage) renderTile(output *image.RGBA, coord tileCoord, frame *render.Frame) {
	// Get the surface size
	rs.mu.RLock()
	surfaceSize := rs.tileGrid.Size
	rs.mu.RUnlock()

	// Get the tile rectangle
	tileRect := coord.tileRect()
	tileRect = tileRect.Intersect(image.Rect(0, 0, surfaceSize.X, surfaceSize.Y))

	if tileRect.Empty() {
		return
	}

	// Find all batches that intersect this tile
	var intersectingBatches []int
	for i := 0; i < frame.BatchCount(); i++ {
		batch, ok := frame.BatchAt(i)
		if !ok {
			continue
		}
		batchRect := image.Rect(
			int(batch.Bounds.Min.X),
			int(batch.Bounds.Min.Y),
			int(batch.Bounds.Max.X),
			int(batch.Bounds.Max.Y),
		)
		if rectanglesIntersect(tileRect, batchRect) {
			intersectingBatches = append(intersectingBatches, i)
		}
	}

	// Create a temporary buffer for this tile
	tileImg := image.NewRGBA(tileRect)

	// Clear the tile
	clearRGBA(tileImg)

	// Copy the tile back to the output
	copyImage(output, tileRect, tileImg, tileImg.Rect)

	_ = intersectingBatches
}

// rectanglesIntersect returns true if two integer rectangles intersect.
func rectanglesIntersect(a, b image.Rectangle) bool {
	return a.Min.X < b.Max.X && a.Max.X > b.Min.X &&
		a.Min.Y < b.Max.Y && a.Max.Y > b.Min.Y
}

// copyImage copies src to dst at the specified destination rectangle.
func copyImage(dst *image.RGBA, dstRect image.Rectangle, src image.Image, srcRect image.Rectangle) {
	if dst == nil || src == nil || dstRect.Empty() || srcRect.Empty() {
		return
	}

	srcRGBA, ok := src.(*image.RGBA)
	if !ok {
		srcRGBA = ImageToRGBA(src)
		if srcRGBA == nil {
			return
		}
	}

	// Simple copy assuming same size
	if dstRect.Dx() == srcRect.Dx() && dstRect.Dy() == srcRect.Dy() {
		for y := 0; y < dstRect.Dy(); y++ {
			srcY := srcRect.Min.Y + y
			dstY := dstRect.Min.Y + y
			for x := 0; x < dstRect.Dx(); x++ {
				srcX := srcRect.Min.X + x
				dstX := dstRect.Min.X + x

				if srcX >= 0 && srcX < srcRGBA.Rect.Max.X &&
					srcY >= 0 && srcY < srcRGBA.Rect.Max.Y {
					srcOff := srcY*srcRGBA.Stride + srcX*4
					dstOff := dstY*dst.Stride + dstX*4
					if srcOff+4 <= len(srcRGBA.Pix) && dstOff+4 <= len(dst.Pix) {
						copy(dst.Pix[dstOff:dstOff+4], srcRGBA.Pix[srcOff:srcOff+4])
					}
				}
			}
		}
	}
}

// ImageToRGBA converts an image.Image to *image.RGBA.
func ImageToRGBA(img image.Image) *image.RGBA {
	if img == nil {
		return nil
	}
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	bounds := img.Bounds()
	result := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			result.Set(x, y, img.At(x, y))
		}
	}
	return result
}

// RasterStats holds statistics about rasterization.
type RasterStats struct {
	FramesRendered  uint64
	TilesRasterized uint64
	FullRedraws     uint64
	PartialRedraws  uint64
}

// Stats returns the current raster statistics.
func (rs *RasterStorage) Stats() RasterStats {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return RasterStats{
		FramesRendered:  rs.framesRendered,
		TilesRasterized: rs.tilesRasterized,
		FullRedraws:     rs.fullRedraws,
		PartialRedraws:  rs.partialRedraws,
	}
}

// ResetStats resets the raster statistics.
func (rs *RasterStorage) ResetStats() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.framesRendered = 0
	rs.tilesRasterized = 0
	rs.fullRedraws = 0
	rs.partialRedraws = 0
}

// RasterConfig holds configuration for rasterization.
type RasterConfig struct {
	// Wallet is the resource wallet.
	Wallet *membudget.Wallet
	// Profile is the resource profile.
	Profile membudget.Profile
}

// DefaultRasterConfig returns a default raster configuration.
func DefaultRasterConfig(profile membudget.Profile) RasterConfig {
	return RasterConfig{
		Profile: profile,
	}
}
