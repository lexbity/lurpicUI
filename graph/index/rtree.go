package index

import (
	"math"
	"sort"

	"codeburg.org/lexbit/lurpicui/gfx"
)

const maxCap = 8 // maximum entries per R*-tree node

// Canceller is satisfied by *job.CancelToken and test mocks.
type Canceller interface {
	Cancelled() bool
}

// entity is an internal representation of a spatial entity.
type entity struct {
	id     EntityID
	bounds gfx.Rect
	cx, cy float32
}

// treeNode is one node in the pre-allocated array-backed R*-tree.
// Either leaf (eN > 0, cN == 0) or internal (cN > 0, eN == 0).
type treeNode struct {
	bounds gfx.Rect
	cx, cy float32
	total  int32    // entity count in subtree
	rep    EntityID // representative entity ID

	// Leaf fields:
	leaf    bool
	eids    [maxCap]EntityID
	eBounds [maxCap]gfx.Rect
	eN      int32

	// Internal fields:
	cids [maxCap]int32
	cN   int32
}

// rstarTree is the built, immutable R*-tree (STR bulk-loaded).
type rstarTree struct {
	entities  []entity
	nodes     []treeNode
	rootIdx   int32
	totalLen  int
	allBounds gfx.Rect
	byID      map[EntityID]int32
}

// Verify interface compliance at compile time.
var _ LODIndex = (*rstarTree)(nil)

// RStarIndexBuilder accumulates entities and bulk-loads an R*-tree on Build.
type RStarIndexBuilder struct {
	entities []entity
}

// NewRStarIndexBuilder creates a builder pre-allocated for capacity entities.
func NewRStarIndexBuilder(capacity int) *RStarIndexBuilder {
	if capacity <= 0 {
		capacity = 64
	}
	return &RStarIndexBuilder{entities: make([]entity, 0, capacity)}
}

// Add registers one entity.
func (b *RStarIndexBuilder) Add(id EntityID, bounds gfx.Rect) {
	cx := (bounds.Min.X + bounds.Max.X) * 0.5
	cy := (bounds.Min.Y + bounds.Max.Y) * 0.5
	b.entities = append(b.entities, entity{id: id, bounds: bounds, cx: cx, cy: cy})
}

// Build constructs the spatial index from accumulated entities.
// Implements IndexBuilder.
func (b *RStarIndexBuilder) Build() SpatialIndex {
	return b.buildWithCancel(nil)
}

// BuildWithCancel builds the index, returning an empty index early if cancel fires.
func (b *RStarIndexBuilder) BuildWithCancel(cancel Canceller) LODIndex {
	return b.buildWithCancel(cancel)
}

func (b *RStarIndexBuilder) buildWithCancel(cancel Canceller) *rstarTree {
	if cancel != nil && cancel.Cancelled() {
		return &rstarTree{}
	}
	n := len(b.entities)
	t := &rstarTree{
		entities: make([]entity, n),
		nodes:    make([]treeNode, 0, max(n/maxCap*2+8, 8)),
		totalLen: n,
		byID:     make(map[EntityID]int32, n),
	}
	copy(t.entities, b.entities)

	for i, e := range t.entities {
		if cancel != nil && i%1000 == 0 && cancel.Cancelled() {
			return &rstarTree{} // cancelled — return empty index
		}
		t.byID[e.id] = int32(i)
	}
	if n == 0 {
		return t
	}

	// Compute allBounds
	t.allBounds = t.entities[0].bounds
	for _, e := range t.entities[1:] {
		t.allBounds = t.allBounds.Union(e.bounds)
	}

	// Build via STR bulk-loading
	indices := make([]int32, n)
	for i := range indices {
		indices[i] = int32(i)
	}
	leafIdxs := t.strLeaves(indices)
	if len(leafIdxs) == 1 {
		t.rootIdx = leafIdxs[0]
	} else {
		t.rootIdx = t.strInternal(leafIdxs)
	}
	return t
}

// strLeaves sorts entities via STR and creates leaf nodes.
func (t *rstarTree) strLeaves(indices []int32) []int32 {
	n := len(indices)
	if n == 0 {
		return nil
	}

	// Sort by X center
	sort.Slice(indices, func(i, j int) bool {
		return t.entities[indices[i]].cx < t.entities[indices[j]].cx
	})

	P := (n + maxCap - 1) / maxCap
	S := int(math.Ceil(math.Sqrt(float64(P))))
	sliceSize := (n + S - 1) / S

	var leafIdxs []int32
	for i := 0; i < n; i += sliceSize {
		end := i + sliceSize
		if end > n {
			end = n
		}
		slice := append([]int32(nil), indices[i:end]...)

		// Sort slice by Y center
		sort.Slice(slice, func(a, b int) bool {
			return t.entities[slice[a]].cy < t.entities[slice[b]].cy
		})

		// Pack into leaf nodes of maxCap
		for j := 0; j < len(slice); j += maxCap {
			endJ := j + maxCap
			if endJ > len(slice) {
				endJ = len(slice)
			}
			leafIdxs = append(leafIdxs, t.makeLeaf(slice[j:endJ]))
		}
	}
	return leafIdxs
}

// strInternal groups node indices into internal nodes using STR.
func (t *rstarTree) strInternal(nodeIdxs []int32) int32 {
	for len(nodeIdxs) > maxCap {
		// Sort by X center
		sort.Slice(nodeIdxs, func(i, j int) bool {
			return t.nodes[nodeIdxs[i]].cx < t.nodes[nodeIdxs[j]].cx
		})

		n := len(nodeIdxs)
		P := (n + maxCap - 1) / maxCap
		S := int(math.Ceil(math.Sqrt(float64(P))))
		sliceSize := (n + S - 1) / S

		var nextLevel []int32
		for i := 0; i < n; i += sliceSize {
			end := i + sliceSize
			if end > n {
				end = n
			}
			slice := append([]int32(nil), nodeIdxs[i:end]...)

			sort.Slice(slice, func(a, b int) bool {
				return t.nodes[slice[a]].cy < t.nodes[slice[b]].cy
			})

			for j := 0; j < len(slice); j += maxCap {
				endJ := j + maxCap
				if endJ > len(slice) {
					endJ = len(slice)
				}
				nextLevel = append(nextLevel, t.makeInternal(slice[j:endJ]))
			}
		}
		nodeIdxs = nextLevel
	}
	if len(nodeIdxs) == 1 {
		return nodeIdxs[0]
	}
	return t.makeInternal(nodeIdxs)
}

func (t *rstarTree) makeLeaf(eIndices []int32) int32 {
	idx := int32(len(t.nodes))
	n := int32(len(eIndices))
	var node treeNode
	node.leaf = true
	node.eN = n
	node.total = n
	var sumCx, sumCy float32
	for i, ei := range eIndices {
		e := &t.entities[ei]
		node.eids[i] = e.id
		node.eBounds[i] = e.bounds
		sumCx += e.cx
		sumCy += e.cy
		if i == 0 {
			node.bounds = e.bounds
			node.rep = e.id
		} else {
			node.bounds = node.bounds.Union(e.bounds)
		}
	}
	node.cx = sumCx / float32(n)
	node.cy = sumCy / float32(n)
	t.nodes = append(t.nodes, node)
	return idx
}

func (t *rstarTree) makeInternal(childIdxs []int32) int32 {
	idx := int32(len(t.nodes))
	n := int32(len(childIdxs))
	var node treeNode
	node.leaf = false
	node.cN = n
	var sumCx, sumCy float32
	for i, ci := range childIdxs {
		c := &t.nodes[ci]
		node.cids[i] = ci
		w := float32(c.total)
		sumCx += c.cx * w
		sumCy += c.cy * w
		node.total += c.total
		if i == 0 {
			node.bounds = c.bounds
			node.rep = c.rep
		} else {
			node.bounds = node.bounds.Union(c.bounds)
		}
	}
	if node.total > 0 {
		node.cx = sumCx / float32(node.total)
		node.cy = sumCy / float32(node.total)
	}
	t.nodes = append(t.nodes, node)
	return idx
}

// ── SpatialIndex implementation ──────────────────────────────────────────────

func (t *rstarTree) Bounds() gfx.Rect { return t.allBounds }
func (t *rstarTree) Len() int         { return t.totalLen }

func (t *rstarTree) Query(region gfx.Rect) []EntityID {
	if t == nil || len(t.nodes) == 0 {
		return nil
	}
	var out []EntityID
	stack := []int32{t.rootIdx}
	for len(stack) > 0 {
		idx := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		node := &t.nodes[idx]
		if !node.bounds.Intersects(region) {
			continue
		}
		if node.leaf {
			for i := int32(0); i < node.eN; i++ {
				if node.eBounds[i].Intersects(region) {
					out = append(out, node.eids[i])
				}
			}
			continue
		}
		for i := node.cN - 1; i >= 0; i-- {
			stack = append(stack, node.cids[i])
			if i == 0 {
				break
			}
		}
	}
	return out
}

func (t *rstarTree) QueryPoint(p gfx.Point, radius float32) (EntityID, bool) {
	region := gfx.Rect{
		Min: gfx.Point{X: p.X - radius, Y: p.Y - radius},
		Max: gfx.Point{X: p.X + radius, Y: p.Y + radius},
	}
	ids := t.Query(region)
	if len(ids) == 0 {
		return 0, false
	}
	// Return closest center
	var bestID EntityID
	bestD2 := float32(math.MaxFloat32)
	for _, id := range ids {
		if ei, ok := t.byID[id]; ok {
			e := &t.entities[ei]
			dx := e.cx - p.X
			dy := e.cy - p.Y
			d2 := dx*dx + dy*dy
			if d2 < bestD2 {
				bestD2 = d2
				bestID = id
			}
		}
	}
	if bestD2 > radius*radius {
		return 0, false
	}
	return bestID, true
}

func (t *rstarTree) QueryNearest(p gfx.Point, maxDist float32) (EntityID, bool) {
	id, ok := t.QueryPoint(p, maxDist)
	return id, ok
}

// ── LODIndex implementation ──────────────────────────────────────────────────

func (t *rstarTree) QueryLOD(viewport gfx.Rect, pixelsPerUnit float32) LODResult {
	var out LODResult
	if t == nil || len(t.nodes) == 0 {
		return out
	}
	stack := []int32{t.rootIdx}
	for len(stack) > 0 {
		idx := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		node := &t.nodes[idx]
		if !node.bounds.Intersects(viewport) {
			continue
		}
		if node.bounds.Width()*pixelsPerUnit < MinIndividualPixels {
			area := node.bounds.Width() * node.bounds.Height()
			density := float32(0)
			if area > 0 {
				density = float32(node.total) / area
			}
			out.Clusters = append(out.Clusters, ClusterEntity{
				Bounds:         node.bounds,
				Center:         gfx.Point{X: node.cx, Y: node.cy},
				Count:          int(node.total),
				Density:        density,
				Representative: node.rep,
			})
			continue
		}
		if node.leaf {
			for i := int32(0); i < node.eN; i++ {
				b := node.eBounds[i]
				if !b.Intersects(viewport) {
					continue
				}
				var cx, cy float32
				if ei, ok := t.byID[node.eids[i]]; ok {
					cx = t.entities[ei].cx
					cy = t.entities[ei].cy
				} else {
					cx = (b.Min.X + b.Max.X) * 0.5
					cy = (b.Min.Y + b.Max.Y) * 0.5
				}
				if b.Width()*pixelsPerUnit >= MinIndividualPixels {
					out.Individuals = append(out.Individuals, IndividualEntity{
						ID:     node.eids[i],
						Bounds: b,
						Center: gfx.Point{X: cx, Y: cy},
					})
				} else {
					out.Clusters = append(out.Clusters, ClusterEntity{
						Bounds:         b,
						Center:         gfx.Point{X: cx, Y: cy},
						Count:          1,
						Density:        1,
						Representative: node.eids[i],
					})
				}
			}
			continue
		}
		for i := node.cN - 1; i >= 0; i-- {
			stack = append(stack, node.cids[i])
			if i == 0 {
				break
			}
		}
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
