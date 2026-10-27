package software

import (
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
		// No previous frame exists to diff against, so there is no safe
		// sub-frame damage: batch bounds do not include per-command support
		// overhang (stroke width, glyph bitmaps, shadow halos), and a partial
		// first frame would drop those pixels. Full damage forces the full
		// redraw path, which is the correct production strategy for a first
		// frame (spec D1/S3 replaces this with support-bounds damage).
		surfaceSize := rs.tileGrid.Size
		cols := (surfaceSize.X + TileSize - 1) / TileSize
		rows := (surfaceSize.Y + TileSize - 1) / TileSize
		for cy := 0; cy < rows; cy++ {
			for cx := 0; cx < cols; cx++ {
				rs.currentDamage.AddTile(tileCoord{X: cx, Y: cy})
			}
		}
		for i := 0; i < frame.BatchCount(); i++ {
			batch, ok := frame.BatchAt(i)
			if !ok {
				continue
			}
			for _, coord := range BatchTiles(batch, surfaceSize) {
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
