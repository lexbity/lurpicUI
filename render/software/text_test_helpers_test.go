package software

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/text"
)

func testShapeSession(t testing.TB, registry *text.FontRegistry, scale float32) *text.ShapeSession {
	t.Helper()
	wallet, err := membudget.NewWallet(membudget.DefaultPlan(membudget.Standard).Caps[membudget.Text])
	if err != nil {
		t.Fatal(err)
	}
	service, err := text.NewService(registry, wallet, text.DefaultLimits(membudget.Standard))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	session, err := service.AtScale(scale)
	if err != nil {
		t.Fatal(err)
	}
	return session
}
