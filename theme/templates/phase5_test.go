package templates

import "testing"

func TestPreviewMatrix_coversAllShipThemesAndDensities(t *testing.T) {
	cards := PreviewMatrix()
	if got := len(cards); got != 9 {
		t.Fatalf("preview matrix size = %d", got)
	}

	seen := map[string]int{}
	for _, card := range cards {
		seen[card.ThemeName+"/"+card.Density.String()]++
		if card.Control.H <= 0 || card.Input.H <= 0 || card.Navigation.H <= 0 || card.Notification.H <= 0 || card.Chart.H <= 0 {
			t.Fatalf("preview card has invalid geometry: %#v", card)
		}
	}

	for _, key := range []string{
		"uneNuit/compact",
		"uneNuit/regular",
		"uneNuit/touchspread",
		"sythique/compact",
		"sythique/regular",
		"sythique/touchspread",
		"notes/compact",
		"notes/regular",
		"notes/touchspread",
	} {
		if seen[key] != 1 {
			t.Fatalf("missing preview card for %s: %v", key, seen)
		}
	}
}

func TestPreviewMatrix_densityScalingIsMonotonic(t *testing.T) {
	cards := PreviewMatrix(Notes())
	var compact, regular, touchspread PreviewCard
	for _, card := range cards {
		switch card.Density {
		case DensityCompact:
			compact = card
		case DensityRegular:
			regular = card
		case DensityTouchspread:
			touchspread = card
		}
	}
	if !(compact.Control.H < regular.Control.H && regular.Control.H < touchspread.Control.H) {
		t.Fatalf("control heights are not monotonic: compact=%v regular=%v touch=%v", compact.Control.H, regular.Control.H, touchspread.Control.H)
	}
	if !(compact.Navigation.H < regular.Navigation.H && regular.Navigation.H < touchspread.Navigation.H) {
		t.Fatalf("tab heights are not monotonic: compact=%v regular=%v touch=%v", compact.Navigation.H, regular.Navigation.H, touchspread.Navigation.H)
	}
	if !(compact.Notification.W < regular.Notification.W && regular.Notification.W < touchspread.Notification.W) {
		t.Fatalf("dialog widths are not monotonic: compact=%v regular=%v touch=%v", compact.Notification.W, regular.Notification.W, touchspread.Notification.W)
	}
}
