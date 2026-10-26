package renderutil

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/canonical"
)

type RenderBatchDiffKind uint8

const (
	RenderBatchUnchanged RenderBatchDiffKind = iota
	RenderBatchPartialChange
	RenderBatchFullChange
	RenderBatchAdded
	RenderBatchRemoved
)

type RenderBatchDiff struct {
	Kind       RenderBatchDiffKind
	DirtyRects []gfx.Rect
}

type FrameDiff struct {
	RenderBatchs        map[render.RenderBatchID]RenderBatchDiff
	CompositeDirtyRects []gfx.Rect
}

type RenderBatchCache struct {
	RenderBatchs map[render.RenderBatchID]RenderBatchSnapshot
	order        []render.RenderBatchID
}

type RenderBatchSnapshot struct {
	Batch          render.FrameBatch
	commandBounds  []gfx.Rect
	commandDigests []string
	order          int
	complexMove    bool
}

func NewRenderBatchCache() *RenderBatchCache {
	return &RenderBatchCache{
		RenderBatchs: make(map[render.RenderBatchID]RenderBatchSnapshot),
	}
}

func (c *RenderBatchCache) Diff(frame *render.Frame) *FrameDiff {
	batchCount := 0
	if frame != nil {
		batchCount = frame.BatchCount()
	}
	diff := &FrameDiff{
		RenderBatchs: make(map[render.RenderBatchID]RenderBatchDiff, batchCount),
	}
	if c == nil || frame == nil {
		return diff
	}

	old := c.RenderBatchs
	seen := make(map[render.RenderBatchID]struct{}, batchCount)

	currentBatches := make([]render.FrameBatch, batchCount)
	for idx := 0; idx < batchCount; idx++ {
		RenderBatch, ok := frame.BatchAt(idx)
		if !ok {
			continue
		}
		currentBatches[idx] = RenderBatch
		seen[RenderBatch.ID] = struct{}{}
		snap, ok := old[RenderBatch.ID]
		if !ok {
			diff.RenderBatchs[RenderBatch.ID] = RenderBatchDiff{Kind: RenderBatchAdded, DirtyRects: []gfx.Rect{RenderBatch.Bounds}}
			diff.CompositeDirtyRects = append(diff.CompositeDirtyRects, RenderBatch.Bounds)
			continue
		}
		currentDigests, currentBounds, currentComplex := batchMetadata(frame, idx)

		if snap.order != idx || !rectEqual(snap.Batch.Bounds, RenderBatch.Bounds) || snap.Batch.Opacity != RenderBatch.Opacity {
			diff.RenderBatchs[RenderBatch.ID] = RenderBatchDiff{Kind: RenderBatchFullChange, DirtyRects: []gfx.Rect{unionRects(snap.Batch.Bounds, RenderBatch.Bounds)}}
			diff.CompositeDirtyRects = append(diff.CompositeDirtyRects, unionRects(snap.Batch.Bounds, RenderBatch.Bounds))
			continue
		}

		if stringSlicesEqual(snap.commandDigests, currentDigests) {
			diff.RenderBatchs[RenderBatch.ID] = RenderBatchDiff{Kind: RenderBatchUnchanged}
			continue
		}

		if snap.complexMove || currentComplex {
			diff.RenderBatchs[RenderBatch.ID] = RenderBatchDiff{Kind: RenderBatchFullChange, DirtyRects: []gfx.Rect{RenderBatch.Bounds}}
			diff.CompositeDirtyRects = append(diff.CompositeDirtyRects, RenderBatch.Bounds)
			continue
		}

		kind, dirty := detectPartialChange(snap.commandBounds, currentBounds, snap.commandDigests, currentDigests)
		if kind == RenderBatchPartialChange {
			diff.RenderBatchs[RenderBatch.ID] = RenderBatchDiff{Kind: kind, DirtyRects: dirty}
			diff.CompositeDirtyRects = append(diff.CompositeDirtyRects, dirty...)
			continue
		}

		diff.RenderBatchs[RenderBatch.ID] = RenderBatchDiff{Kind: RenderBatchFullChange, DirtyRects: []gfx.Rect{RenderBatch.Bounds}}
		diff.CompositeDirtyRects = append(diff.CompositeDirtyRects, RenderBatch.Bounds)
	}

	for id, snap := range old {
		if _, ok := seen[id]; ok {
			continue
		}
		diff.RenderBatchs[id] = RenderBatchDiff{Kind: RenderBatchRemoved, DirtyRects: []gfx.Rect{snap.Batch.Bounds}}
		diff.CompositeDirtyRects = append(diff.CompositeDirtyRects, snap.Batch.Bounds)
	}

	if batchCount > 0 {
		diff.CompositeDirtyRects = PropagateDirty(currentBatches, RenderBatchDirtyMap(diff.RenderBatchs))
		diff.CompositeDirtyRects = MergeRects(diff.CompositeDirtyRects, 0.25)
		diff.CompositeDirtyRects = RemoveContained(diff.CompositeDirtyRects)
	}

	return diff
}

func (c *RenderBatchCache) Update(frame *render.Frame) {
	if c == nil {
		return
	}
	if frame == nil {
		c.RenderBatchs = make(map[render.RenderBatchID]RenderBatchSnapshot)
		c.order = c.order[:0]
		return
	}
	c.RenderBatchs = make(map[render.RenderBatchID]RenderBatchSnapshot, frame.BatchCount())
	c.order = c.order[:0]
	for idx := 0; idx < frame.BatchCount(); idx++ {
		RenderBatch, ok := frame.BatchAt(idx)
		if !ok {
			continue
		}
		commandDigests, commandBounds, complexMove := batchMetadata(frame, idx)
		snap := RenderBatchSnapshot{
			Batch:          RenderBatch,
			commandBounds:  commandBounds,
			commandDigests: commandDigests,
			order:          idx,
			complexMove:    complexMove,
		}
		c.RenderBatchs[RenderBatch.ID] = snap
		c.order = append(c.order, RenderBatch.ID)
	}
}

func detectPartialChange(oldBounds, newBounds []gfx.Rect, oldDigests, newDigests []string) (RenderBatchDiffKind, []gfx.Rect) {
	maxLen := len(oldDigests)
	if len(newDigests) > maxLen {
		maxLen = len(newDigests)
	}
	if maxLen == 0 {
		return RenderBatchUnchanged, nil
	}

	var dirty []gfx.Rect
	changed := 0
	for i := 0; i < maxLen; i++ {
		if i < len(oldDigests) && i < len(newDigests) && oldDigests[i] == newDigests[i] {
			continue
		}
		changed++
		if i < len(oldBounds) {
			if r := oldBounds[i]; !r.IsEmpty() {
				dirty = append(dirty, r)
			}
		}
		if i < len(newBounds) {
			if r := newBounds[i]; !r.IsEmpty() {
				dirty = append(dirty, r)
			}
		}
	}

	if changed == 0 {
		return RenderBatchUnchanged, nil
	}
	if float32(changed)/float32(maxLen) > 0.30 {
		return RenderBatchFullChange, nil
	}
	return RenderBatchPartialChange, RemoveContained(MergeRects(dirty, 0.25))
}

func batchMetadata(frame *render.Frame, index int) (digests []string, bounds []gfx.Rect, complex bool) {
	digests = make([]string, 0)
	bounds = make([]gfx.Rect, 0)
	visitOK := frame.VisitBatchCommands(index, func(command gfx.CommandView) bool {
		digest, err := canonical.DigestCommand(command)
		if err != nil {
			panic(fmt.Errorf("renderutil: canonical command contract: %w", err))
		}
		digests = append(digests, digest)
		bounds = append(bounds, commandBounds(command))
		if transform, ok := command.Scalars().(gfx.PushTransform); ok && !transform.Matrix.IsIdentity() {
			complex = true
		}
		return true
	})
	if !visitOK {
		panic("renderutil: released frame command page")
	}
	return digests, bounds, complex
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func commandBounds(cmd gfx.CommandView) gfx.Rect {
	switch c := cmd.Scalars().(type) {
	case gfx.FillRect:
		return c.Rect
	case gfx.StrokeRect:
		return c.Rect.Inset(-c.Stroke.Width/2, -c.Stroke.Width/2)
	case gfx.FillPath:
		return pathBounds(cmd.Path())
	case gfx.StrokePath:
		return pathBounds(cmd.Path()).Inset(-c.Stroke.Width/2, -c.Stroke.Width/2)
	case gfx.DrawPolyline:
		return pointsBounds(cmd.Points()).Inset(-c.Stroke.Width/2, -c.Stroke.Width/2)
	case gfx.DrawGlyphRun:
		return gfx.Rect{}
	case gfx.DrawSelectionRects:
		return rectUnionAll(cmd.Rects())
	case gfx.DrawImage:
		return c.DestRect
	default:
		return gfx.Rect{}
	}
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

func PropagateDirty(RenderBatchs []render.FrameBatch, perRenderBatchDirty map[render.RenderBatchID][]gfx.Rect) []gfx.Rect {
	var out []gfx.Rect
	for i, RenderBatch := range RenderBatchs {
		dirty := perRenderBatchDirty[RenderBatch.ID]
		out = append(out, dirty...)
		if len(dirty) == 0 {
			continue
		}
		for j := i + 1; j < len(RenderBatchs); j++ {
			upper := RenderBatchs[j]
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

func RenderBatchDirtyMap(diffs map[render.RenderBatchID]RenderBatchDiff) map[render.RenderBatchID][]gfx.Rect {
	out := make(map[render.RenderBatchID][]gfx.Rect, len(diffs))
	for id, diff := range diffs {
		if len(diff.DirtyRects) == 0 {
			continue
		}
		out[id] = append([]gfx.Rect(nil), diff.DirtyRects...)
	}
	return out
}

func rectUnionAll(rects gfx.Values[gfx.Rect]) gfx.Rect {
	if rects.Len() == 0 {
		return gfx.Rect{}
	}
	out := rects.At(0)
	for i := 1; i < rects.Len(); i++ {
		r := rects.At(i)
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

func pathBounds(path gfx.PathView) gfx.Rect {
	var bounds gfx.Rect
	first := true
	for seg := range path.Segments.All() {
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

func pointsBounds(pts gfx.Values[gfx.Point]) gfx.Rect {
	if pts.Len() == 0 {
		return gfx.Rect{}
	}
	bounds := gfx.Rect{Min: pts.At(0), Max: pts.At(0)}
	for i := 1; i < pts.Len(); i++ {
		p := pts.At(i)
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
