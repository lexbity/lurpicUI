package ll014_good

type GoodOverlay struct {
	facet.Facet
	OverlayRole facet.OverlayRole
	HitRole     facet.HitRole
	Dismiss     facet.DismissalTrigger
}
