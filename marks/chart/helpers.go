package chart

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/internal/markutil"
	"codeburg.org/lexbit/lurpicui/theme"
)

func registerDescriptor(d marks.Descriptor) {
	marks.RegisterDescriptor(d)
}

func syncLayout(layoutRole *facet.LayoutRole, bounds gfx.Rect) {
	markutil.SyncLayout(layoutRole, bounds)
}

func syncViewport(viewport *facet.ViewportRole, transform gfx.Transform) {
	markutil.SyncViewport(viewport, transform)
}

func fillColor(m theme.Material, fallback gfx.Color) gfx.Color {
	return markutil.FillColor(m, fallback)
}

func strokeColor(m theme.Material, fallback gfx.Color) gfx.Color {
	return markutil.StrokeColor(m, fallback)
}

func strokeStyle(stroke theme.MaterialStroke) gfx.StrokeStyle {
	return markutil.StrokeStyle(stroke)
}

func transformAnchors(tx gfx.Transform, anchors layout.AnchorSet) layout.AnchorSet {
	if len(anchors) == 0 {
		return nil
	}
	out := make(layout.AnchorSet, len(anchors))
	for id, pt := range anchors {
		out[id] = tx.TransformPoint(pt)
	}
	return out
}
