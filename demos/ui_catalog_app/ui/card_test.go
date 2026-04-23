package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
)

func TestCardFacet_Creation(t *testing.T) {
	th := theme.Default()
	shaper := text.NewShaper(nil)
	shaper.SetContentScale(1.0)

	entry := &model.CatalogEntry{
		ID:          "basic.rect",
		DisplayName: "Rectangle",
		Family:      model.FamilyBasic,
		Coverage:    model.CoverageImplemented,
	}

	card := NewCardFacet(th, shaper, entry)
	if card == nil {
		t.Fatal("NewCardFacet returned nil")
	}

	if card.Entry() != entry {
		t.Error("Card.Entry() does not match provided entry")
	}
}

func TestCardFacet_Measure(t *testing.T) {
	th := theme.Default()
	shaper := text.NewShaper(nil)
	shaper.SetContentScale(1.0)

	entry := &model.CatalogEntry{
		ID:       "basic.rect",
		Coverage: model.CoverageImplemented,
	}

	card := NewCardFacet(th, shaper, entry)
	size := card.layout.OnMeasure(facet.Constraints{})

	if size.W != cardWidth {
		t.Errorf("Card width = %v, want %v", size.W, cardWidth)
	}
	if size.H != cardHeight {
		t.Errorf("Card height = %v, want %v", size.H, cardHeight)
	}
}

func TestCardFacet_SetOnClick(t *testing.T) {
	th := theme.Default()
	shaper := text.NewShaper(nil)
	shaper.SetContentScale(1.0)

	entry := &model.CatalogEntry{
		ID:       "basic.rect",
		Coverage: model.CoverageImplemented,
	}

	card := NewCardFacet(th, shaper, entry)

	card.SetOnClick(func() {
		t.Log("Click handler called")
	})

	// Verify the callback is set (would be called via input role)
	if card.onClick == nil {
		t.Error("SetOnClick did not set the callback")
	}
}

func TestCardFacet_TruncateText(t *testing.T) {
	th := theme.Default()
	shaper := newTestShaper(t)

	entry := &model.CatalogEntry{
		ID:       "basic.rect",
		Coverage: model.CoverageImplemented,
	}

	card := NewCardFacet(th, shaper, entry)

	// Short text should not be truncated (within reasonable width)
	short := card.truncateText("short", 100, theme.TextBodyS)
	if short != "short" {
		t.Errorf("Short text was truncated: %q", short)
	}

	// Very long text should be truncated
	// With nil font, the shaper wraps unpredictably, so we test truncation behavior differently
	longText := "this.is.a.very.long.entry.identifier.that.exceeds.the.card.width"
	truncated := card.truncateText(longText, 40, theme.TextBodyS)

	// Verify truncation occurred (length reduced and ends with ellipsis)
	if len(truncated) >= len(longText) {
		t.Error("Long text was not truncated - length not reduced")
	}

	// Check that it ends with ellipsis (accounting for possible newlines in wrapped text)
	trimmed := truncated
	if len(trimmed) > 3 {
		// Handle case where shaper wraps text and adds newlines
		trimmed = strings.ReplaceAll(trimmed, "\n", "")
	}
	if !strings.HasSuffix(trimmed, "...") {
		t.Errorf("Truncated text missing ellipsis: %q", truncated)
	}
}

func newTestShaper(t *testing.T) *text.Shaper {
	t.Helper()
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("NewFontRegistry: %v", err)
	}
	for _, path := range []string{
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
		"/usr/share/fonts/Adwaita/AdwaitaSans-Regular.ttf",
	} {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := reg.LoadFontFile(filepath.Clean(path)); err != nil {
			t.Fatalf("LoadFontFile %s: %v", path, err)
		}
		shaper := text.NewShaper(reg)
		shaper.SetContentScale(1.0)
		return shaper
	}
	t.Skip("no usable font found for card truncation test")
	return nil
}

func TestCardFacet_Constants(t *testing.T) {
	// Verify card dimensions are reasonable
	if cardWidth <= 0 {
		t.Error("cardWidth must be positive")
	}
	if cardHeight <= 0 {
		t.Error("cardHeight must be positive")
	}
	if cardMargin < 0 {
		t.Error("cardMargin must be non-negative")
	}

	// Verify card aspect ratio is reasonable (wider than tall)
	if cardWidth <= cardHeight {
		t.Logf("Warning: cardWidth (%v) <= cardHeight (%v)", cardWidth, cardHeight)
	}
}
