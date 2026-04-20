package layout

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// LayerID is a parent-scoped stable identifier for a composition layer.
type LayerID uint32

// PlacementMode selects the algorithm used to arrange children inside a layer.
type PlacementMode uint8

const (
	PlacementStack PlacementMode = iota
	PlacementSplit
	PlacementGrid
	PlacementFree
	PlacementAnchor
	PlacementProjected
)

// MeasurementMode controls how a layer contributes to parent size resolution.
type MeasurementMode uint8

const (
	MeasureStructural MeasurementMode = iota
	MeasureNonStructural
	MeasureHybrid
)

// CoordSpace describes the coordinate system children inhabit within a layer.
type CoordSpace uint8

const (
	CoordParentLayout CoordSpace = iota
	CoordContent
	CoordViewport
	CoordScreenAligned
)

// LayerHitPolicy controls how hits traverse a layer.
type LayerHitPolicy uint8

const (
	HitNormal LayerHitPolicy = iota
	HitPassThrough
	HitBlockBelow
	HitDisabled
)

// ClipPolicy controls how a layer clips its children.
type ClipPolicy uint8

const (
	ClipNone ClipPolicy = iota
	ClipToParent
	ClipToContent
	ClipToViewport
)

// CoordLimits constrains the operating extent of a layer's coordinate space.
type CoordLimits struct {
	Bounds        gfx.Rect
	AllowOverflow bool
}

// PlacementHints carries mode-specific placement data for a child attachment.
type PlacementHints struct {
	Flex  float32
	Align Alignment

	ColStart int
	ColSpan  int
	RowStart int
	RowSpan  int

	FreeAnchor FreeAnchor
	Offset     gfx.Point

	AnchorRef  AnchorID
	AnchorSide AnchorSide
	AnchorGap  float32
}

// FreeAnchor identifies the corner or edge used as a free-placement origin.
type FreeAnchor uint8

const (
	FreeTopLeft FreeAnchor = iota
	FreeTopCenter
	FreeTopRight
	FreeCenterLeft
	FreeCenter
	FreeCenterRight
	FreeBottomLeft
	FreeBottomCenter
	FreeBottomRight
)

// AnchorSide describes the placement side relative to an exported anchor.
type AnchorSide uint8

const (
	AnchorAbove AnchorSide = iota
	AnchorBelow
	AnchorLeft
	AnchorRight
	AnchorCenter
)

// ChildAttachment is metadata stored on the parent-child edge.
type ChildAttachment struct {
	LayerID   LayerID
	Placement PlacementHints
	ZPriority int
}

// LayerSpec describes one composition layer owned by a parent facet.
type LayerSpec struct {
	ID           LayerID
	Placement    PlacementMode
	Measurement  MeasurementMode
	CoordSpace   CoordSpace
	CoordLimits  CoordLimits
	HitPolicy    LayerHitPolicy
	RenderOrder  int
	ClipPolicy   ClipPolicy
	HybridInsets gfx.Insets
}

// LayerDiff reports the minimum invalidation needed when LayerSpecs change.
type LayerDiff struct {
	NeedsLayout     bool
	NeedsProjection bool
}

// Viewport is a plain-data snapshot of viewport state.
type Viewport struct {
	Transform   gfx.Transform
	WorldBounds gfx.Rect
}

// AnchorID is a stable semantic name for an exported spatial anchor.
type AnchorID string

// AnchorSet contains exported anchor positions.
type AnchorSet map[AnchorID]gfx.Point

// AnchorChange reports a cache difference for one anchor.
type AnchorChange struct {
	ID      AnchorID
	Old     gfx.Point
	New     gfx.Point
	Removed bool
}

// ResolvedLayer is the runtime result of resolving a LayerSpec for a frame.
type ResolvedLayer struct {
	LayerID     LayerID
	Bounds      gfx.Rect
	Transform   gfx.Transform
	ClipRect    gfx.Rect
	CoordLimits CoordLimits
	AnchorCache *AnchorPositionCache
	HitPolicy   LayerHitPolicy
	RenderOrder int
	CoordSpace  CoordSpace
}

// AnchorExportContext provides the spatial context needed to export anchors.
type AnchorExportContext struct {
	ResolvedLayer ResolvedLayer
	Viewport      Viewport
}

// AnchorExporter is implemented by facets that publish spatial anchors.
type AnchorExporter interface {
	ExportAnchors(ctx AnchorExportContext) AnchorSet
}

// PlacementPolicy arranges children within a resolved layer.
type PlacementPolicy interface {
	Measure(children []ChildNode, constraints gfx.Size) gfx.Size
	Arrange(children []ChildNode, layer ResolvedLayer)
}

// ChildNode is the narrow view of a child facet exposed to placement policies.
type ChildNode struct {
	FacetID       facet.FacetID
	Attachment    ChildAttachment
	IntrinsicSize gfx.Size
	MinSize       gfx.Size
	WorldPosition gfx.Point
	WorldSize     gfx.Size
	HasWorldSpace bool
	handle        *ChildArrangeHandle
}

// AttachArrangeHandle connects a runtime write handle to the child node.
func (n *ChildNode) AttachArrangeHandle(h *ChildArrangeHandle) {
	if n == nil {
		return
	}
	n.handle = h
}

// SetArrangedBounds records the arranged bounds for this child.
func (n *ChildNode) SetArrangedBounds(r gfx.Rect) {
	if n == nil || n.handle == nil {
		return
	}
	n.handle.SetArrangedBounds(r)
}

// ChildArrangeHandle is the runtime write channel for arranged bounds.
type ChildArrangeHandle struct {
	bounds  gfx.Rect
	written bool
}

// Reset clears the handle so it can be reused for another Arrange pass.
func (h *ChildArrangeHandle) Reset() {
	if h == nil {
		return
	}
	h.bounds = gfx.Rect{}
	h.written = false
}

// SetArrangedBounds stores the arranged bounds for a child during Arrange.
func (h *ChildArrangeHandle) SetArrangedBounds(r gfx.Rect) {
	if h == nil {
		return
	}
	if h.written {
		panic("layout: SetArrangedBounds called twice for the same child in one Arrange pass")
	}
	h.bounds = r
	h.written = true
}

// Bounds returns the arranged rectangle stored in the handle.
func (h *ChildArrangeHandle) Bounds() (gfx.Rect, bool) {
	if h == nil || !h.written {
		return gfx.Rect{}, false
	}
	return h.bounds, true
}

// ValidateLayerSpec reports contract violations in a layer declaration.
func ValidateLayerSpec(s LayerSpec) error {
	if s.ID == 0 {
		return fmt.Errorf("layout: layer spec has zero LayerID")
	}
	if s.RenderOrder < 0 {
		return fmt.Errorf("layout: layer %d has negative RenderOrder %d", s.ID, s.RenderOrder)
	}
	switch s.Placement {
	case PlacementAnchor:
		if s.Measurement != MeasureNonStructural {
			return fmt.Errorf("layout: layer %d placement Anchor requires MeasureNonStructural", s.ID)
		}
	case PlacementProjected:
		if s.Measurement != MeasureNonStructural {
			return fmt.Errorf("layout: layer %d placement Projected requires MeasureNonStructural", s.ID)
		}
	}
	return nil
}

// DiffLayerSpecs compares two layer spec slices and reports invalidation scope.
func DiffLayerSpecs(oldSpecs, newSpecs []LayerSpec) LayerDiff {
	if len(oldSpecs) != len(newSpecs) {
		return LayerDiff{NeedsLayout: true, NeedsProjection: true}
	}
	var diff LayerDiff
	for i := range oldSpecs {
		oldSpec := oldSpecs[i]
		newSpec := newSpecs[i]
		if oldSpec.ID != newSpec.ID {
			return LayerDiff{NeedsLayout: true, NeedsProjection: true}
		}
		if oldSpec.Placement != newSpec.Placement ||
			oldSpec.Measurement != newSpec.Measurement ||
			oldSpec.CoordSpace != newSpec.CoordSpace ||
			oldSpec.HybridInsets != newSpec.HybridInsets {
			diff.NeedsLayout = true
			diff.NeedsProjection = true
		}
		if oldSpec.CoordLimits != newSpec.CoordLimits {
			if oldSpec.Measurement == MeasureNonStructural && newSpec.Measurement == MeasureNonStructural {
				diff.NeedsProjection = true
			} else {
				diff.NeedsLayout = true
				diff.NeedsProjection = true
			}
		}
		if oldSpec.HitPolicy != newSpec.HitPolicy ||
			oldSpec.RenderOrder != newSpec.RenderOrder ||
			oldSpec.ClipPolicy != newSpec.ClipPolicy {
			diff.NeedsProjection = true
		}
	}
	return diff
}

// AnchorPositionCache stores exported anchors for one parent facet.
type AnchorPositionCache struct {
	positions map[AnchorID]gfx.Point
	version   uint64
}

// NewAnchorPositionCache constructs an empty anchor cache.
func NewAnchorPositionCache() *AnchorPositionCache {
	return &AnchorPositionCache{
		positions: make(map[AnchorID]gfx.Point),
	}
}

// Update stores an anchor position and reports whether the cache changed.
func (c *AnchorPositionCache) Update(id AnchorID, pos gfx.Point) bool {
	if c == nil {
		return false
	}
	if c.positions == nil {
		c.positions = make(map[AnchorID]gfx.Point)
	}
	old, ok := c.positions[id]
	if ok && old == pos {
		return false
	}
	c.positions[id] = pos
	c.version++
	return true
}

// Replace swaps the full cache contents and reports the anchors that changed.
func (c *AnchorPositionCache) Replace(anchors AnchorSet) []AnchorChange {
	if c == nil {
		return nil
	}
	next := make(map[AnchorID]gfx.Point, len(anchors))
	for id, pos := range anchors {
		next[id] = pos
	}
	var changes []AnchorChange
	for id, oldPos := range c.positions {
		if newPos, ok := next[id]; !ok {
			changes = append(changes, AnchorChange{ID: id, Old: oldPos, Removed: true})
		} else if newPos != oldPos {
			changes = append(changes, AnchorChange{ID: id, Old: oldPos, New: newPos})
		}
	}
	for id, newPos := range next {
		if _, ok := c.positions[id]; !ok {
			changes = append(changes, AnchorChange{ID: id, New: newPos})
		}
	}
	if len(changes) == 0 {
		return nil
	}
	c.positions = next
	c.version++
	return changes
}

// Reset clears the cache and reports any anchors that were removed.
func (c *AnchorPositionCache) Reset() []AnchorChange {
	return c.Replace(nil)
}

// Get returns the cached position for id.
func (c *AnchorPositionCache) Get(id AnchorID) (gfx.Point, bool) {
	if c == nil || c.positions == nil {
		return gfx.Point{}, false
	}
	pos, ok := c.positions[id]
	return pos, ok
}

// Snapshot returns a copy of the cached anchors.
func (c *AnchorPositionCache) Snapshot() AnchorSet {
	if c == nil || len(c.positions) == 0 {
		return nil
	}
	out := make(AnchorSet, len(c.positions))
	for id, pos := range c.positions {
		out[id] = pos
	}
	return out
}

// Len returns the number of cached anchors.
func (c *AnchorPositionCache) Len() int {
	if c == nil {
		return 0
	}
	return len(c.positions)
}

// Version returns the current cache version.
func (c *AnchorPositionCache) Version() uint64 {
	if c == nil {
		return 0
	}
	return c.version
}
