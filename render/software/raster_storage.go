package software

import (
	"image"
	"math"
	"slices"
	"sync"

	"codeburg.org/lexbit/lurpicui/render"
)

// RasterStorage manages tile-based raster storage for a surface.
type RasterStorage struct {
	mu sync.RWMutex

	// windowID identifies which window this storage belongs to.
	windowID string

	// tileGrid is the grid of tiles for the current surface.
	tileGrid *TileGrid

	// frame is the current frame being rendered.
	frame *render.Frame

	// lastFrameTiles maps from tileCoord to the batch IDs that affected it in the last frame.
	// We only need the tile->batch mapping, not the full frame reference (C10).
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
	// Width and Height are the initial surface dimensions.
	Width, Height int
}

// NewRasterStorage creates a new raster storage.
func NewRasterStorage(config RasterStorageConfig) *RasterStorage {
	grid := NewTileGrid(config.Width, config.Height)
	return &RasterStorage{
		windowID:       config.WindowID,
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

	surfaceSize := rs.tileGrid.Size

	// Damage is the union of two independent sources (D1):
	// 1. Declared damage from assembly (conservative support rectangles)
	// 2. Structural diff as safety net (batch ID list equality)

	// First, add tiles from declared damage regions
	if frame != nil {
		for i := 0; i < frame.DirtyRegionCount(); i++ {
			rect, ok := frame.DirtyRegionAt(i)
			if ok && !rect.IsEmpty() {
				// Convert gfx.Rect to image.Rectangle and add affected tiles
				damageRect := image.Rect(
					int(rect.Min.X),
					int(rect.Min.Y),
					int(math.Ceil(float64(rect.Max.X))),
					int(math.Ceil(float64(rect.Max.Y))),
				)
				for _, coord := range TilesFromRect(damageRect) {
					rs.currentDamage.AddTile(coord)
				}
			}
		}
	}

	// Second, compute structural damage from batch ID comparison
	// This catches reordering, removal, addition, and identity substitution
	// that declared damage might miss (e.g., content changes with unchanged bounds).
	if len(rs.lastFrameTiles) > 0 && frame != nil {
		// Build new frame tile map: tileCoord -> ordered batch IDs
		newFrameTiles := make(map[tileCoord][]render.RenderBatchID)

		for i := 0; i < frame.BatchCount(); i++ {
			batch, ok := frame.BatchAt(i)
			if !ok {
				continue
			}
			for _, coord := range BatchTiles(batch, surfaceSize) {
				newFrameTiles[coord] = append(newFrameTiles[coord], batch.ID)
			}
		}

		// Compare with last frame using exact ordered equality (D1)
		// If batch ID lists differ, mark the tile as damaged
		for coord, newBatchIDs := range newFrameTiles {
			oldBatchIDs := rs.lastFrameTiles[coord]
			if !batchIDsEqual(oldBatchIDs, newBatchIDs) {
				rs.currentDamage.AddTile(coord)
			}
		}

		// Batches that were in last frame but not in new frame also cause damage
		// (removal case - the tile needs to be cleared)
		for coord := range rs.lastFrameTiles {
			if _, exists := newFrameTiles[coord]; !exists {
				rs.currentDamage.AddTile(coord)
			}
		}

		rs.lastFrameTiles = newFrameTiles
	} else if frame != nil {
		// First frame with no previous frame: compute all tiles from batch bounds
		// This is the union of all batch coverage for the first frame
		newFrameTiles := make(map[tileCoord][]render.RenderBatchID)
		for i := 0; i < frame.BatchCount(); i++ {
			batch, ok := frame.BatchAt(i)
			if !ok {
				continue
			}
			for _, coord := range BatchTiles(batch, surfaceSize) {
				newFrameTiles[coord] = append(newFrameTiles[coord], batch.ID)
				rs.currentDamage.AddTile(coord)
			}
		}
		rs.lastFrameTiles = newFrameTiles
	}

	rs.frame = frame
	// lastFrame field removed (C10): only lastFrameTiles is needed for damage comparison
}

// batchIDsEqual checks if two batch ID slices are equal using exact
// ordered equality. This is the safety-net structural diff for damage detection,
// catching reordering, removal, addition, and identity substitution (D1).
func batchIDsEqual(a, b []render.RenderBatchID) bool {
	return slices.Equal(a, b)
}

// EndFrame finishes a frame.
func (rs *RasterStorage) EndFrame() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.framesRendered++
	// Damage is cleared at the beginning of BeginFrame (ownership: BeginFrame
	// computes, Submit consumes, EndFrame retires). This allows the damage set
	// to survive until the render decision is made (D1).
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
