package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

// TestGolden_shell pins the rendered gallery shell (chrome, 3-pane split,
// status bar) so layout regressions are caught by pixel diff. The software
// backend and the shared NotoSans font keep it deterministic (NFR).
func TestGolden_shell(t *testing.T) {
	scope := store.NewScope()
	ctx := app.BuildContext{
		StoreScope:   scope,
		WindowSize:   gfx.Size{W: 1280, H: 800},
		ContentScale: 1,
		Theme:        theme.DefaultResolvedContext(),
		FontRegistry: testkit.TestFontRegistry(t),
	}
	root := NewRoot(ctx, nil, seedRows(t), nil)
	h := testkit.NewStandardHarness(t, scope, 1280, 800, root)
	h.RunFrame()
	testkit.AssertGolden(t, h.Surface(), "shell")
}
