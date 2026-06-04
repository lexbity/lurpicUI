package testkit

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/fontdata"
	"codeburg.org/lexbit/lurpicui/text"
)

// TestFontBytes returns the canonical embedded test font (NotoSans-Regular).
// Never nil or empty.
func TestFontBytes() []byte {
	return fontdata.TestFontBytes()
}

// TestFontBoldBytes returns the embedded bold variant (NotoSans-Bold).
// Never nil or empty.
func TestFontBoldBytes() []byte {
	return fontdata.TestFontBoldBytes()
}

// TestFontRegistry returns a FontRegistry loaded with the canonical test font
// (NotoSans-Regular). Fatals the test if the font cannot be loaded; never
// returns a nil registry.
func TestFontRegistry(t testing.TB) *text.FontRegistry {
	t.Helper()
	return fontdata.TestFontRegistry(t)
}
