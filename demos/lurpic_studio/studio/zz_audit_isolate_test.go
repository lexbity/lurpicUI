package studio

// Temporary: isolate the dash-column artifact — fresh shell starting on E2.

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestZZAuditIsolate(t *testing.T) {
	out := os.Getenv("LURPIC_AUDIT_OUT")
	if out == "" {
		t.Skip("LURPIC_AUDIT_OUT not set")
	}
	build := func(preset ExhibitID) (*Root, *testkit.Harness) {
		ctx := app.BuildContext{
			WindowSize:   gfx.Size{W: 1280, H: 800},
			ContentScale: 1,
			Theme:        theme.DefaultResolvedContext(),
		}
		root := NewRoot(ctx, nil, seedRows(t), nil)
		root.Shell().ActiveExhibit.Set(preset)
		h := testkit.NewStandardHarness(t, 1280, 800, root)
		h.RunFrame()
		h.RunFrame()
		return root, h
	}
	write := func(h *testkit.Harness, name string) {
		f, err := os.Create(filepath.Join(out, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, h.Surface().Capture()); err != nil {
			t.Fatal(err)
		}
	}

	_, h := build(ExhibitLayers)
	write(h, "iso_layers_only")

	_, h2 := build(ExhibitRealtime)
	write(h2, "iso_realtime_nofeed")

	// Palette via the real Ctrl+K path (Root focused).
	root3, h3 := build(ExhibitPlayground)
	h3.Runtime().SetFocus(root3)
	testkit.DriveKeyPress(h3, platform.KeyK, platform.ModControl)
	h3.RunFrame()
	h3.RunFrame()
	if !root3.Shell().CommandOpen.Get() {
		t.Log("ctrl+k did not set CommandOpen")
	}
	write(h3, "iso_palette_ctrlk")

	// Palette via the store directly.
	root4, h4 := build(ExhibitPlayground)
	root4.Shell().CommandOpen.Set(true)
	h4.RunFrame()
	h4.RunFrame()
	write(h4, "iso_palette_store")
}
