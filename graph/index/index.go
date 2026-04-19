package index

import "codeburg.org/lexbit/lurpicui/gfx"

// EntityID is a stable spatial index identifier.
type EntityID uint64

// SpatialIndex is a read-only spatial query interface.
type SpatialIndex interface {
	Query(region gfx.Rect) []EntityID
	QueryPoint(p gfx.Point, expansionRadius float32) (EntityID, bool)
	QueryNearest(p gfx.Point, maxDistance float32) (EntityID, bool)
	Bounds() gfx.Rect
	Len() int
}

// IndividualEntity is a single entity returned at high zoom.
type IndividualEntity struct {
	ID     EntityID
	Bounds gfx.Rect
	Center gfx.Point
}

// ClusterEntity is a group of entities collapsed at low zoom.
type ClusterEntity struct {
	Bounds         gfx.Rect
	Center         gfx.Point
	Count          int
	Density        float32
	Representative EntityID
}

// LODResult holds the output of a LOD query.
type LODResult struct {
	Individuals []IndividualEntity
	Clusters    []ClusterEntity
}

// LODIndex extends SpatialIndex with level-of-detail queries.
type LODIndex interface {
	SpatialIndex
	QueryLOD(viewport gfx.Rect, pixelsPerUnit float32) LODResult
}

// IndexBuilder accumulates entities and produces a SpatialIndex.
type IndexBuilder interface {
	Add(id EntityID, bounds gfx.Rect)
	Build() SpatialIndex
}

// LOD rendering thresholds.
const (
	MinIndividualPixels = float32(8.0)
	MinLabelPixels      = float32(24.0)
)

func shouldRenderIndividually(worldSize, pixelsPerUnit float32) bool {
	return worldSize*pixelsPerUnit >= MinIndividualPixels
}

func shouldRenderLabel(worldSize, pixelsPerUnit float32) bool {
	return worldSize*pixelsPerUnit >= MinLabelPixels
}

var _ = shouldRenderLabel
var _ = shouldRenderIndividually
