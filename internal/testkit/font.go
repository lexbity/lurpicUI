package testkit

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/fontdata"
	"codeburg.org/lexbit/lurpicui/internal/membudget"
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

// NewFontRegistry returns an empty test-owned registry with a real bounded
// text wallet. The caller owns font loading; cleanup releases its charges.
func NewFontRegistry(t testing.TB) *text.FontRegistry {
	t.Helper()
	profile := membudget.Standard
	limits := text.DefaultLimits(profile)
	wallet, err := membudget.NewWallet(membudget.DefaultPlan(profile).Caps[membudget.Text])
	if err != nil {
		t.Fatal(err)
	}
	registry, err := text.NewFontRegistry(wallet, limits.MaxFontFaces)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registry.Close() })
	return registry
}

func TestTextService(t testing.TB, registry *text.FontRegistry) *text.Service {
	t.Helper()
	service, err := text.NewService(registry, text.DefaultLimits(membudget.Standard))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	return service
}

func TestShapeSession(t testing.TB, registry *text.FontRegistry, scale float32) *text.ShapeSession {
	t.Helper()
	session, err := TestTextService(t, registry).AtScale(scale, nil)
	if err != nil {
		t.Fatal(err)
	}
	return session
}
