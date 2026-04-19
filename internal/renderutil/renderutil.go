package renderutil

import (
	"image"
	"reflect"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
)

type LayerDiffKind uint8

const (
	LayerUnchanged LayerDiffKind = iota
	LayerPartialChange
	LayerFullChange
	LayerAdded
	LayerRemoved
)

type LayerDiff struct {
	Kind       LayerDiffKind
	DirtyRects []gfx.Rect
}

type FrameDiff struct {
	Layers              map[render.LayerID]LayerDiff
	CompositeDirtyRects []gfx.Rect
}

type LayerCache struct {
	layers map[render.LayerID]layerSnapshot
	order  []render.LayerID
}

type layerSnapshot struct {
	layer       render.Layer
	commands    []gfx.Command
	order       int
	complexMove bool
}

func NewLayerCache() *LayerCache {
	return &LayerCache{
		layers: make(map[render.LayerID]layerSnapshot),
	}
}

func (c *LayerCache) Diff(frame *render.Frame) *FrameDiff {
	diff := &FrameDiff{
		Layers: make(map[render.LayerID]LayerDiff, len(frame.Layers)),
	}
	if c == nil || frame == nil {
		return diff
	}

	old := c.layers
	seen := make(map[render.LayerID]struct{}, len(frame.Layers))

	for idx, layer := range frame.Layers {
		seen[layer.ID] = struct{}{}
		snap, ok := old[layer.ID]
		if !ok {
			diff.Layers[layer.ID] = LayerDiff{Kind: LayerAdded, DirtyRects: []gfx.Rect{layer.Bounds}}
			diff.CompositeDirtyRects = append(diff.CompositeDirtyRects, layer.Bounds)
			continue
		}

		if snap.order != idx || !rectEqual(snap.layer.Bounds, layer.Bounds) || snap.layer.Opacity != layer.Opacity {
			diff.Layers[layer.ID] = LayerDiff{Kind: LayerFullChange, DirtyRects: []gfx.Rect{unionRects(snap.layer.Bounds, layer.Bounds)}}
			diff.CompositeDirtyRects = append(diff.CompositeDirtyRects, unionRects(snap.layer.Bounds, layer.Bounds))
			continue
		}

		if snap.layer.CommandHash == layer.CommandHash && reflect.DeepEqual(snap.commands, layer.Commands.Commands) {
			diff.Layers[layer.ID] = LayerDiff{Kind: LayerUnchanged}
			continue
		}

		if snap.complexMove || hasComplexTransforms(layer.Commands.Commands) {
			diff.Layers[layer.ID] = LayerDiff{Kind: LayerFullChange, DirtyRects: []gfx.Rect{layer.Bounds}}
			diff.CompositeDirtyRects = append(diff.CompositeDirtyRects, layer.Bounds)
			continue
		}

		kind, dirty := detectPartialChange(snap.commands, layer.Commands.Commands)
		if kind == LayerPartialChange {
			diff.Layers[layer.ID] = LayerDiff{Kind: kind, DirtyRects: dirty}
			diff.CompositeDirtyRects = append(diff.CompositeDirtyRects, dirty...)
			continue
		}

		diff.Layers[layer.ID] = LayerDiff{Kind: LayerFullChange, DirtyRects: []gfx.Rect{layer.Bounds}}
		diff.CompositeDirtyRects = append(diff.CompositeDirtyRects, layer.Bounds)
	}

	for id, snap := range old {
		if _, ok := seen[id]; ok {
			continue
		}
		diff.Layers[id] = LayerDiff{Kind: LayerRemoved, DirtyRects: []gfx.Rect{snap.layer.Bounds}}
		diff.CompositeDirtyRects = append(diff.CompositeDirtyRects, snap.layer.Bounds)
	}

	if len(frame.Layers) > 0 {
		diff.CompositeDirtyRects = PropagateDirty(frame.Layers, layerDirtyMap(diff.Layers))
		diff.CompositeDirtyRects = MergeRects(diff.CompositeDirtyRects, 0.25)
		diff.CompositeDirtyRects = RemoveContained(diff.CompositeDirtyRects)
	}

	return diff
}

func (c *LayerCache) Update(frame *render.Frame, rasterBuffers map[render.LayerID]*image.RGBA) {
	if c == nil {
		return
	}
	if frame == nil {
		c.layers = make(map[render.LayerID]layerSnapshot)
		c.order = c.order[:0]
		return
	}
	c.layers = make(map[render.LayerID]layerSnapshot, len(frame.Layers))
	c.order = c.order[:0]
	for idx, layer := range frame.Layers {
		cmds := make([]gfx.Command, len(layer.Commands.Commands))
		copy(cmds, layer.Commands.Commands)
		snap := layerSnapshot{
			layer:       layer,
			commands:    cmds,
			order:       idx,
			complexMove: hasComplexTransforms(cmds),
		}
		_ = rasterBuffers[layer.ID]
		c.layers[layer.ID] = snap
		c.order = append(c.order, layer.ID)
	}
}

func detectPartialChange(oldCmds, newCmds []gfx.Command) (LayerDiffKind, []gfx.Rect) {
	maxLen := len(oldCmds)
	if len(newCmds) > maxLen {
		maxLen = len(newCmds)
	}
	if maxLen == 0 {
		return LayerUnchanged, nil
	}

	var dirty []gfx.Rect
	changed := 0
	for i := 0; i < maxLen; i++ {
		var oldCmd, newCmd gfx.Command
		if i < len(oldCmds) {
			oldCmd = oldCmds[i]
		}
		if i < len(newCmds) {
			newCmd = newCmds[i]
		}
		if reflect.DeepEqual(oldCmd, newCmd) {
			continue
		}
		changed++
		if i < len(oldCmds) {
			if r := commandBounds(oldCmds[i]); !r.IsEmpty() {
				dirty = append(dirty, r)
			}
		}
		if i < len(newCmds) {
			if r := commandBounds(newCmds[i]); !r.IsEmpty() {
				dirty = append(dirty, r)
			}
		}
	}

	if changed == 0 {
		return LayerUnchanged, nil
	}
	if float32(changed)/float32(maxLen) > 0.30 {
		return LayerFullChange, nil
	}
	return LayerPartialChange, RemoveContained(MergeRects(dirty, 0.25))
}

func commandBounds(cmd gfx.Command) gfx.Rect {
	switch c := cmd.(type) {
	case gfx.FillRect:
		return c.Rect
	case gfx.StrokeRect:
		return c.Rect.Inset(-c.Stroke.Width/2, -c.Stroke.Width/2)
	case gfx.FillPath:
		return pathBounds(c.Path)
	case gfx.StrokePath:
		return pathBounds(c.Path).Inset(-c.Stroke.Width/2, -c.Stroke.Width/2)
	case gfx.DrawPolyline:
		return pointsBounds(c.Points).Inset(-c.Stroke.Width/2, -c.Stroke.Width/2)
	case gfx.DrawGlyphRun:
		return gfx.Rect{}
	case gfx.DrawSelectionRects:
		return rectUnionAll(c.Rects)
	case gfx.DrawImage:
		return c.DestRect
	default:
		return gfx.Rect{}
	}
}

func hasComplexTransforms(cmds []gfx.Command) bool {
	for _, cmd := range cmds {
		switch c := cmd.(type) {
		case gfx.PushTransform:
			if !c.Matrix.IsIdentity() {
				return true
			}
		}
	}
	return false
}

func MergeRects(rects []gfx.Rect, tolerance float32) []gfx.Rect {
	out := make([]gfx.Rect, 0, len(rects))
	for _, r := range rects {
		if !r.IsEmpty() {
			out = append(out, r)
		}
	}
	if len(out) <= 1 {
		return append([]gfx.Rect(nil), out...)
	}

	merged := true
	for merged {
		merged = false
		for i := 0; i < len(out); i++ {
			for j := i + 1; j < len(out); j++ {
				if r, ok := mergeRectPair(out[i], out[j], tolerance); ok {
					out[i] = r
					out = append(out[:j], out[j+1:]...)
					merged = true
					break
				}
			}
			if merged {
				break
			}
		}
	}

	return append([]gfx.Rect(nil), out...)
}

func RemoveContained(rects []gfx.Rect) []gfx.Rect {
	out := make([]gfx.Rect, 0, len(rects))
	for i, r := range rects {
		contained := false
		for j, other := range rects {
			if i == j {
				continue
			}
			if containsRect(other, r) {
				contained = true
				break
			}
		}
		if !contained {
			out = append(out, r)
		}
	}
	return append([]gfx.Rect(nil), out...)
}

func PropagateDirty(layers []render.Layer, perLayerDirty map[render.LayerID][]gfx.Rect) []gfx.Rect {
	var out []gfx.Rect
	for i, layer := range layers {
		dirty := perLayerDirty[layer.ID]
		out = append(out, dirty...)
		if len(dirty) == 0 {
			continue
		}
		for j := i + 1; j < len(layers); j++ {
			upper := layers[j]
			if upper.Opacity >= 1 {
				break
			}
			for _, r := range dirty {
				if rr := intersectRects(r, upper.Bounds); !rr.IsEmpty() {
					out = append(out, rr)
				}
			}
		}
	}
	return out
}

func layerDirtyMap(diffs map[render.LayerID]LayerDiff) map[render.LayerID][]gfx.Rect {
	out := make(map[render.LayerID][]gfx.Rect, len(diffs))
	for id, diff := range diffs {
		if len(diff.DirtyRects) == 0 {
			continue
		}
		out[id] = append([]gfx.Rect(nil), diff.DirtyRects...)
	}
	return out
}

func rectUnionAll(rects []gfx.Rect) gfx.Rect {
	if len(rects) == 0 {
		return gfx.Rect{}
	}
	out := rects[0]
	for _, r := range rects[1:] {
		out = unionRects(out, r)
	}
	return out
}

func mergeRectPair(a, b gfx.Rect, tolerance float32) (gfx.Rect, bool) {
	merged := unionRects(a, b)
	if merged.IsEmpty() {
		return gfx.Rect{}, false
	}
	areaA := rectArea(a)
	areaB := rectArea(b)
	areaU := rectArea(merged)
	if areaU <= 0 {
		return merged, true
	}
	waste := (areaU - areaA - areaB) / areaU
	if waste <= tolerance {
		return merged, true
	}
	return gfx.Rect{}, false
}

func rectArea(r gfx.Rect) float32 {
	if r.IsEmpty() {
		return 0
	}
	return r.Width() * r.Height()
}

func unionRects(a, b gfx.Rect) gfx.Rect {
	if a.IsEmpty() {
		return b
	}
	if b.IsEmpty() {
		return a
	}
	return gfx.Rect{
		Min: gfx.Point{X: minFloat32(a.Min.X, b.Min.X), Y: minFloat32(a.Min.Y, b.Min.Y)},
		Max: gfx.Point{X: maxFloat32(a.Max.X, b.Max.X), Y: maxFloat32(a.Max.Y, b.Max.Y)},
	}
}

func containsRect(outer, inner gfx.Rect) bool {
	if outer.IsEmpty() || inner.IsEmpty() {
		return false
	}
	return outer.Min.X <= inner.Min.X && outer.Min.Y <= inner.Min.Y && outer.Max.X >= inner.Max.X && outer.Max.Y >= inner.Max.Y
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
	return gfx.Rect{Min: gfx.Point{X: minX, Y: minY}, Max: gfx.Point{X: maxX, Y: maxY}}
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
