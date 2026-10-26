// Package software implements a tile-based software renderer with damage tracking.
package software

import (
	"image"
	"math"

	"codeburg.org/lexbit/lurpicui/render"
)

// TileSize is the fixed size of each tile in pixels.
// Per FR-9: 64x64 tiles.
const TileSize = 64

// tileCoord represents a tile coordinate in the tile grid.
type tileCoord struct {
	X, Y int
}

// tileRect returns the pixel rectangle covered by this tile.
func (t tileCoord) tileRect() image.Rectangle {
	return image.Rect(
		t.X*TileSize,
		t.Y*TileSize,
		(t.X+1)*TileSize,
		(t.Y+1)*TileSize,
	)
}

// TilesFromRect returns all tile coordinates that intersect the given rectangle.
func TilesFromRect(r image.Rectangle) []tileCoord {
	if r.Empty() {
		return nil
	}
	minX := int(math.Floor(float64(r.Min.X) / float64(TileSize)))
	minY := int(math.Floor(float64(r.Min.Y) / float64(TileSize)))
	maxX := int(math.Ceil(float64(r.Max.X) / float64(TileSize)))
	maxY := int(math.Ceil(float64(r.Max.Y) / float64(TileSize)))

	var tiles []tileCoord
	for x := minX; x < maxX; x++ {
		for y := minY; y < maxY; y++ {
			tiles = append(tiles, tileCoord{X: x, Y: y})
		}
	}
	return tiles
}

// DamageSet represents a set of damaged tiles.
type DamageSet struct {
	// tiles is a map from tile coordinate to whether it's damaged.
	tiles map[tileCoord]bool
	// bounds is the overall affected rectangle.
	bounds image.Rectangle
}

// NewDamageSet creates a new empty damage set.
func NewDamageSet() *DamageSet {
	return &DamageSet{
		tiles: make(map[tileCoord]bool),
	}
}

// AddRect adds a rectangle to the damage set.
func (d *DamageSet) AddRect(r image.Rectangle) {
	if r.Empty() {
		return
	}
	for _, tile := range TilesFromRect(r) {
		d.tiles[tile] = true
		d.bounds = d.bounds.Union(tile.tileRect())
	}
}

// AddTile adds a specific tile to the damage set.
func (d *DamageSet) AddTile(t tileCoord) {
	d.tiles[t] = true
	d.bounds = d.bounds.Union(t.tileRect())
}

// Contains returns whether a tile is damaged.
func (d *DamageSet) Contains(t tileCoord) bool {
	return d.tiles[t]
}

// Tiles returns all damaged tile coordinates.
func (d *DamageSet) Tiles() []tileCoord {
	tiles := make([]tileCoord, 0, len(d.tiles))
	for t := range d.tiles {
		tiles = append(tiles, t)
	}
	return tiles
}

// Count returns the number of damaged tiles.
func (d *DamageSet) Count() int {
	return len(d.tiles)
}

// Clear clears all damage.
func (d *DamageSet) Clear() {
	d.tiles = make(map[tileCoord]bool)
	d.bounds = image.Rectangle{}
}

// Bounds returns the overall affected bounds.
func (d *DamageSet) Bounds() image.Rectangle {
	return d.bounds
}

// IsEmpty returns whether there is no damage.
func (d *DamageSet) IsEmpty() bool {
	return len(d.tiles) == 0
}

// TileGrid represents the grid of tiles for a surface.
type TileGrid struct {
	// Size is the size of the surface in pixels.
	Size image.Point
	// TileCount is the number of tiles in X and Y directions.
	TileCount image.Point
}

// NewTileGrid creates a new tile grid for a surface of the given size.
func NewTileGrid(width, height int) *TileGrid {
	tileWidth := (width + TileSize - 1) / TileSize
	tileHeight := (height + TileSize - 1) / TileSize
	return &TileGrid{
		Size:      image.Point{X: width, Y: height},
		TileCount: image.Point{X: tileWidth, Y: tileHeight},
	}
}

// TotalTiles returns the total number of tiles in the grid.
func (tg *TileGrid) TotalTiles() int {
	return tg.TileCount.X * tg.TileCount.Y
}

// TileAt returns the tile coordinate at the given pixel coordinate.
func (tg *TileGrid) TileAt(x, y int) tileCoord {
	return tileCoord{
		X: x / TileSize,
		Y: y / TileSize,
	}
}

// DamageRatio returns the ratio of damaged tiles to total tiles.
func DamageRatio(damage *DamageSet, grid *TileGrid) float64 {
	if grid == nil || grid.TotalTiles() == 0 {
		return 0
	}
	return float64(damage.Count()) / float64(grid.TotalTiles())
}

// ShouldFullRedraw returns true if more than 50% of tiles are damaged.
// Per FR-9: deterministic 50% threshold.
func ShouldFullRedraw(damage *DamageSet, grid *TileGrid) bool {
	return DamageRatio(damage, grid) > 0.5
}

// BatchTiles returns all tiles that a batch might affect.
func BatchTiles(batch render.FrameBatch, surfaceSize image.Point) []tileCoord {
	// Convert batch bounds to integer rectangle
	bounds := batch.Bounds
	minX := int(math.Floor(float64(bounds.Min.X)))
	minY := int(math.Floor(float64(bounds.Min.Y)))
	maxX := int(math.Ceil(float64(bounds.Max.X)))
	maxY := int(math.Ceil(float64(bounds.Max.Y)))

	// Clamp to surface bounds
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX > surfaceSize.X {
		maxX = surfaceSize.X
	}
	if maxY > surfaceSize.Y {
		maxY = surfaceSize.Y
	}

	// Calculate which tiles this affects
	return TilesFromRect(image.Rect(minX, minY, maxX, maxY))
}
