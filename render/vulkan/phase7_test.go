//go:build linux && cgo

package vulkan

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/text"
)

func TestBackendSubmitGlyphRun_updatesGlyphAtlasAndPacketStats(t *testing.T) {
	if err := Shutdown(); err != nil {
		t.Skipf("Vulkan unavailable: %v", err)
	}
	defer func() {
		if err := Shutdown(); err != nil {
			t.Fatalf("final shutdown: %v", err)
		}
	}()
	if err := testResetRustState(); err != nil {
		t.Fatalf("reset rust state: %v", err)
	}

	reg := mustPhase7FontRegistry(t)
	fontData := mustPhase7TestFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	if err := reg.LoadFontBytes(fontData, "noto-regular"); err != nil {
		t.Fatalf("load font: %v", err)
	}
	face := reg.Resolve(text.TextStyle{Family: "Noto Sans", Size: 18})
	run := text.GlyphRun{
		Face:  face,
		Size:  18,
		Style: text.TextStyle{Family: "Noto Sans", Size: 18},
		Glyphs: []text.PositionedGlyph{
			{GlyphID: 65, X: 2.5, Y: 4.25, Advance: 8.5},
		},
	}

	backend := &Backend{}
	if err := backend.Initialize(nil); err != nil {
		t.Skipf("Vulkan unavailable: %v", err)
	}
	defer backend.Destroy()

	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 64, 64),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.DrawGlyphRun{
						Run:    run,
						Origin: gfx.Point{X: 10, Y: 12},
						Brush:  gfx.SolidBrush(gfx.Color{R: 1, A: 1}),
					},
				}},
			},
		},
	}

	if err := backend.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got, err := testLastBatchCount(); err != nil {
		t.Fatalf("last batch count: %v", err)
	} else if got != 1 {
		t.Fatalf("last batch count = %d, want 1", got)
	}
	if got, err := testLastCommandCount(); err != nil {
		t.Fatalf("last command count: %v", err)
	} else if got != 1 {
		t.Fatalf("last command count = %d, want 1", got)
	}
	if got, err := testGlyphAtlasCount(); err != nil {
		t.Fatalf("glyph atlas count: %v", err)
	} else if got != 1 {
		t.Fatalf("glyph atlas count = %d, want 1", got)
	}
}

func mustPhase7FontRegistry(t *testing.T) *text.FontRegistry {
	t.Helper()
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("NewFontRegistry: %v", err)
	}
	return reg
}

func mustPhase7TestFont(t *testing.T, rel string) []byte {
	t.Helper()
	path := mustPhase7TestFontPath(t, rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test font %q: %v", path, err)
	}
	return data
}

func mustPhase7TestFontPath(t *testing.T, rel string) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatalf("go env GOMODCACHE: %v", err)
	}
	return filepath.Join(string(bytes.TrimSpace(out)), rel)
}
