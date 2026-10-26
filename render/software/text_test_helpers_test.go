package software

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/text"
)

func newSoftwareEmptyFontRegistry(t testing.TB) *text.FontRegistry {
	t.Helper()
	limits := text.DefaultLimits(membudget.Standard)
	wallet, err := membudget.NewWallet(membudget.DefaultPlan(membudget.Standard).Caps[membudget.Text])
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

func testShapeSession(t testing.TB, registry *text.FontRegistry, scale float32) *text.ShapeSession {
	t.Helper()
	service, err := text.NewService(registry, text.DefaultLimits(membudget.Standard))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	session, err := service.AtScale(scale, nil)
	if err != nil {
		t.Fatal(err)
	}
	return session
}
