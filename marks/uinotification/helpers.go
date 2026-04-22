package uinotification

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

func clampInt(v, min, max int) int {
	return markutil.ClampInt(v, min, max)
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func dialogWidth() float32 {
	return markutil.RegularUINotificationBaseline().Dialog.MinWidth.Regular + 100
}

func dialogHeight() float32 {
	return markutil.RegularUINotificationBaseline().Dialog.MinWidth.Regular - 40
}

func snackbarWidth() float32 {
	return markutil.RegularUINotificationBaseline().Snackbar.MinHeight.Regular*4 + 144
}

func snackbarHeight() float32 {
	return markutil.RegularUINotificationBaseline().Snackbar.MinHeight.Regular + 12
}

func progressLinearHeight() float32 {
	return markutil.RegularUINotificationBaseline().Progress.LinearThickness.Regular + 8
}

func progressCircularSize() float32 {
	return markutil.RegularUINotificationBaseline().Progress.CircularStroke.Regular * 12
}

func progressLinearWidth() float32 {
	return markutil.RegularUINotificationBaseline().Progress.LinearThickness.Regular * 60
}

func attachChildMarks(parent *facet.Facet, children []marks.Mark) {
	if parent == nil {
		return
	}
	for _, child := range children {
		if child == nil {
			continue
		}
		impl, ok := child.(facet.FacetImpl)
		if !ok {
			panic("marks/uinotification: child mark does not implement facet.FacetImpl")
		}
		parent.AddChild(impl.Base())
	}
}

func boundsAnchors(bounds gfx.Rect) layout.AnchorSet {
	if bounds.IsEmpty() {
		return nil
	}
	return layout.AnchorSet{
		"bounds-center": {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y + bounds.Height()/2},
		"top-left":      {X: bounds.Min.X, Y: bounds.Min.Y},
		"top-right":     {X: bounds.Max.X, Y: bounds.Min.Y},
		"bottom-right":  {X: bounds.Max.X, Y: bounds.Max.Y},
		"bottom-left":   {X: bounds.Min.X, Y: bounds.Max.Y},
	}
}
