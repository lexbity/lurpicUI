package canvas

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	gindex "codeburg.org/lexbit/lurpicui/graph/index"
)

// placeholderColor is shown while the spatial index is building.
var placeholderColor = gfx.ColorFromRGBA8(240, 240, 245, 255)

// nodeColor is the fill color for individual graph nodes.
var nodeColor = gfx.ColorFromRGBA8(100, 150, 220, 255)

// nodeBorderColor is the stroke color for individual graph nodes.
var nodeBorderColor = gfx.ColorFromRGBA8(60, 100, 180, 255)

// clusterColor is the fill color for collapsed clusters.
var clusterColor = gfx.ColorFromRGBA8(150, 100, 220, 200)

// project is the ProjectionRole.OnProject callback.
// It drains completed index builds, then renders the current view.
func (f *GraphCanvasFacet) project(ctx facet.ProjectionContext) *gfx.CommandList {
	// Commit any completed background index builds first.
	f.drainPool()

	list := &gfx.CommandList{}

	if f.nodeIndex == nil {
		// Placeholder while the index hasn't been built yet.
		list.Add(gfx.FillRect{
			Rect:  ctx.Bounds,
			Brush: gfx.SolidBrush(placeholderColor),
		})
		// Re-invalidate every frame while a background build is in flight
		// so we keep draining the pool until the result arrives.
		if f.buildCount.Load() > 0 {
			f.rt.Invalidate(f.Base().ID(), facet.DirtyProjection, "awaitingIndex")
		}
		return list
	}

	pan, zoom := f.getPanZoom()

	// Compute the world-space viewport from screen bounds.
	invZoom := float32(1.0)
	if zoom > 0 {
		invZoom = 1.0 / zoom
	}
	worldViewport := gfx.Rect{
		Min: gfx.Point{
			X: (ctx.Bounds.Min.X - pan.X) * invZoom,
			Y: (ctx.Bounds.Min.Y - pan.Y) * invZoom,
		},
		Max: gfx.Point{
			X: (ctx.Bounds.Max.X - pan.X) * invZoom,
			Y: (ctx.Bounds.Max.Y - pan.Y) * invZoom,
		},
	}

	lodResult := f.nodeIndex.QueryLOD(worldViewport, zoom)

	// Apply world→screen transform: scale by zoom, translate by pan.
	list.Add(gfx.PushTransform{Matrix: gfx.Transform{A: zoom, D: zoom, TX: pan.X, TY: pan.Y}})

	for _, cluster := range lodResult.Clusters {
		f.renderCluster(list, cluster)
	}
	for _, ind := range lodResult.Individuals {
		f.renderNode(list, ind)
	}

	list.Add(gfx.PopTransform{})
	return list
}

// getPanZoom returns the current pan and zoom from the viewport store.
func (f *GraphCanvasFacet) getPanZoom() (pan gfx.Point, zoom float32) {
	zoom = 1.0
	if f.viewportStore == nil {
		return
	}
	vs := f.viewportStore.Get()
	pan = vs.Pan
	if vs.Zoom > 0 {
		zoom = vs.Zoom
	}
	return
}

func (f *GraphCanvasFacet) renderNode(list *gfx.CommandList, e gindex.IndividualEntity) {
	list.Add(gfx.FillRect{
		Rect:  e.Bounds,
		Brush: gfx.SolidBrush(nodeColor),
	})
	list.Add(gfx.StrokeRect{
		Rect:   e.Bounds,
		Stroke: gfx.DefaultStroke(1),
		Brush:  gfx.SolidBrush(nodeBorderColor),
	})
}

func (f *GraphCanvasFacet) renderCluster(list *gfx.CommandList, c gindex.ClusterEntity) {
	list.Add(gfx.FillRect{
		Rect:  c.Bounds,
		Brush: gfx.SolidBrush(clusterColor),
	})
}

// hitTest maps a screen-space point to the nearest graph node.
// Uses a 5px screen radius converted to world units.
func (f *GraphCanvasFacet) hitTest(p gfx.Point) facet.HitResult {
	if f.nodeIndex == nil {
		return facet.HitResult{}
	}
	pan, zoom := f.getPanZoom()
	invZoom := float32(1.0)
	if zoom > 0 {
		invZoom = 1.0 / zoom
	}
	worldPt := gfx.Point{
		X: (p.X - pan.X) * invZoom,
		Y: (p.Y - pan.Y) * invZoom,
	}
	const screenRadius = float32(5.0)
	worldRadius := screenRadius * invZoom

	id, ok := f.nodeIndex.QueryPoint(worldPt, worldRadius)
	if !ok {
		return facet.HitResult{}
	}
	return facet.HitResult{
		Hit:    true,
		MarkID: facet.MarkID(id),
		Cursor: facet.CursorPointer,
	}
}
