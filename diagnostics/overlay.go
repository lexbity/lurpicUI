package diagnostics

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
)

// Overlay draws a visual debug RenderBatch over the render output.
// Activated by Toggle — no recompilation needed.
type Overlay struct {
	active     bool
	showBounds bool
	showDirty  bool
	showHitMap bool

	boundsColor    gfx.Color
	dirtyColor     gfx.Color
	hitColor       gfx.Color
	anchorColor    gfx.Color
	anchorHotColor gfx.Color
	linkColor      gfx.Color
	timingColor    gfx.Color

	lastAnchorVersions map[facet.FacetID]uint64
}

// NewOverlay constructs an inactive overlay with default debug colors.
func NewOverlay() *Overlay {
	return &Overlay{
		boundsColor:        gfx.ColorFromRGBA8(80, 120, 220, 80),
		dirtyColor:         gfx.ColorFromRGBA8(220, 40, 40, 60),
		hitColor:           gfx.ColorFromRGBA8(40, 180, 60, 100),
		anchorColor:        gfx.ColorFromRGBA8(60, 200, 220, 180),
		anchorHotColor:     gfx.ColorFromRGBA8(255, 240, 100, 220),
		linkColor:          gfx.ColorFromRGBA8(180, 180, 180, 160),
		timingColor:        gfx.ColorFromRGBA8(220, 200, 40, 200),
		lastAnchorVersions: make(map[facet.FacetID]uint64),
	}
}

// Toggle cycles: off → bounds → bounds+dirty → bounds+dirty+hitmap → off.
func (o *Overlay) Toggle() {
	if o == nil {
		return
	}
	switch {
	case !o.active:
		o.active = true
		o.showBounds = true
		o.showDirty = false
		o.showHitMap = false
	case o.showBounds && !o.showDirty:
		o.showDirty = true
	case o.showBounds && o.showDirty && !o.showHitMap:
		o.showHitMap = true
	default:
		o.active = false
		o.showBounds = false
		o.showDirty = false
		o.showHitMap = false
	}
}

// IsActive reports whether the overlay is currently rendering.
func (o *Overlay) IsActive() bool {
	return o != nil && o.active
}

// Inject adds an overlay RenderBatch to the top of frame.
// No-op when inactive or frame is nil.
func (o *Overlay) Inject(frame *render.Frame, inspector *Inspector, hitProbe *HitProbe, stats FrameStats) {
	if o == nil || !o.active || frame == nil {
		return
	}

	var surfaceSize gfx.Size
	for _, l := range frame.RenderBatchs {
		if l.Bounds.Max.X > surfaceSize.W {
			surfaceSize.W = l.Bounds.Max.X
		}
		if l.Bounds.Max.Y > surfaceSize.H {
			surfaceSize.H = l.Bounds.Max.Y
		}
	}

	var list gfx.CommandList
	if o.showBounds && inspector != nil {
		inspector.Walk(func(_ int, info FacetInfo) {
			o.drawFacetBounds(&list, info)
			if o.showDirty && info.DirtyFlags != 0 {
				o.drawDirtyHighlight(&list, info)
			}
			o.drawLayerSummary(&list, info)
			if snap, ok := inspector.AnchorSnapshot(info.ID); ok {
				o.drawAnchorSnapshot(&list, inspector, snap, surfaceSize)
			}
		})
	}
	if o.showHitMap && hitProbe != nil {
		o.drawHitRegions(&list, hitProbe, surfaceSize)
	}
	o.drawTimingBar(&list, stats, surfaceSize)

	frame.RenderBatchs = append(frame.RenderBatchs, render.RenderBatch{
		Bounds:   gfx.Rect{Max: gfx.Point{X: surfaceSize.W, Y: surfaceSize.H}},
		Opacity:  1.0,
		Commands: list,
	})
	frame.Layers = append(frame.Layers, render.LayeredBatch{
		RenderOrder: int(^uint(0) >> 1),
		Batches: []render.RenderBatch{
			{
				Bounds:   gfx.Rect{Max: gfx.Point{X: surfaceSize.W, Y: surfaceSize.H}},
				Opacity:  1.0,
				Commands: list,
			},
		},
	})
}

// InjectOverlay is called by the runtime after frame assembly to inject the overlay.
// It satisfies the runtime's overlayInjector interface when Overlay is active.
func (h *Hook) InjectOverlay(frame *render.Frame, stats FrameStats) {
	if h == nil || h.Overlay == nil || !h.Overlay.IsActive() {
		return
	}
	probe := h.HitProbe
	if h.HitProbeSource != nil {
		probe = h.HitProbeSource.HitProbe()
	}
	h.Overlay.Inject(frame, h.Inspector, probe, stats)
}

func (o *Overlay) drawFacetBounds(list *gfx.CommandList, info FacetInfo) {
	list.Add(gfx.StrokeRect{
		Rect:   info.ArrangedBounds,
		Stroke: gfx.DefaultStroke(1),
		Brush:  gfx.SolidBrush(o.boundsColor),
	})
}

func (o *Overlay) drawDirtyHighlight(list *gfx.CommandList, info FacetInfo) {
	list.Add(gfx.FillRect{
		Rect:  info.ArrangedBounds,
		Brush: gfx.SolidBrush(o.dirtyColor),
	})
}

func (o *Overlay) drawLayerSummary(list *gfx.CommandList, info FacetInfo) {
	for _, layer := range info.Layers {
		list.Add(gfx.StrokeRect{
			Rect:   layer.Bounds,
			Stroke: gfx.DefaultStroke(1),
			Brush:  gfx.SolidBrush(o.anchorColor),
		})
	}
}

func (o *Overlay) drawAnchorSnapshot(list *gfx.CommandList, inspector *Inspector, snap AnchorSnapshot, _ gfx.Size) {
	if inspector == nil || len(snap.Entries) == 0 {
		return
	}
	hot := o.lastAnchorVersions[snap.ParentID] != snap.Version
	if hot {
		o.lastAnchorVersions[snap.ParentID] = snap.Version
	}
	anchorBrush := o.anchorColor
	if hot {
		anchorBrush = o.anchorHotColor
	}
	for _, entry := range snap.Entries {
		center := entry.Position
		cross := gfx.RectFromXYWH(center.X-2, center.Y-2, 4, 4)
		list.Add(gfx.FillRect{
			Rect:  cross,
			Brush: gfx.SolidBrush(anchorBrush),
		})
		for _, childID := range entry.Children {
			childInfo, ok := inspector.Find(childID)
			if !ok {
				continue
			}
			childCenter := gfx.Point{
				X: childInfo.ArrangedBounds.Min.X + childInfo.ArrangedBounds.Width()/2,
				Y: childInfo.ArrangedBounds.Min.Y + childInfo.ArrangedBounds.Height()/2,
			}
			list.Add(gfx.DrawPolyline{
				Points: []gfx.Point{center, childCenter},
				Stroke: gfx.DefaultStroke(1),
				Brush:  gfx.SolidBrush(o.linkColor),
			})
		}
	}
}

func (o *Overlay) drawHitRegions(list *gfx.CommandList, probe *HitProbe, surfaceSize gfx.Size) {
	if probe == nil || probe.hitMap == nil {
		return
	}
	for _, entry := range probe.hitMap.Entries() {
		for _, region := range entry.Regions {
			bounds := entry.Transform.TransformRect(region.Bounds)
			list.Add(gfx.StrokeRect{
				Rect:   bounds,
				Stroke: gfx.DefaultStroke(1),
				Brush:  gfx.SolidBrush(o.hitColor),
			})
		}
	}
}

func (o *Overlay) drawTimingBar(list *gfx.CommandList, stats FrameStats, surfaceSize gfx.Size) {
	const barHeight = 4.0
	const targetMs = 16.67
	totalMs := float32(stats.LayoutDuration.Milliseconds()) +
		float32(stats.ProjectDuration.Milliseconds()) +
		float32(stats.RenderDuration.Milliseconds())
	if totalMs <= 0 || surfaceSize.W <= 0 {
		return
	}
	filled := (totalMs / targetMs) * surfaceSize.W
	if filled > surfaceSize.W {
		filled = surfaceSize.W
	}
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(0, surfaceSize.H-barHeight, filled, barHeight),
		Brush: gfx.SolidBrush(o.timingColor),
	})
}
