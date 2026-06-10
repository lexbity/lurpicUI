package ll014_bad

// OverlayHost is an overlay-like facet missing layer, hit, and dismissal.
type OverlayHost struct {
	facet.Facet
	content facet.FacetImpl
}
