package reactive

import (
	"codeburg.org/lexbit/lurpicui/scale"
	"codeburg.org/lexbit/lurpicui/store"
)

// ZoomController implements semantic zoom by applying scale.ZoomDomain and
// scale.PanDomain transforms to a domain ValueStore. The controller operates
// in data-space: callers convert screen coordinates to data values via
// scale.Invert before calling Zoom.
type ZoomController struct {
	domain *store.ValueStore[[2]float64]
}

// NewZoomController creates a ZoomController that mutates the given domain
// store on zoom/pan operations.
func NewZoomController(domain *store.ValueStore[[2]float64]) *ZoomController {
	return &ZoomController{domain: domain}
}

// Zoom zooms the domain around focal by factor.
// factor > 1 zooms in, 0 < factor < 1 zooms out.
func (zc *ZoomController) Zoom(focal, factor float64) {
	d := zc.domain.Get()
	lo, hi := scale.ZoomDomain(d[0], d[1], focal, factor)
	zc.domain.Set([2]float64{lo, hi})
}

// Pan pans the domain by delta in data-space units.
func (zc *ZoomController) Pan(delta float64) {
	d := zc.domain.Get()
	lo, hi := scale.PanDomain(d[0], d[1], delta)
	zc.domain.Set([2]float64{lo, hi})
}
