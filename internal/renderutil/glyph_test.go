package renderutil

import (
	"bytes"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/text"
)

func TestGlyphSizeBitsFallsBackToStyleThenDefault(t *testing.T) {
	if got := GlyphSizeBits(text.GlyphRun{}); got != math.Float32bits(text.DefaultStyle().Size) {
		t.Fatalf("empty run size bits = %d, want default size bits", got)
	}

	run := text.GlyphRun{Style: text.TextStyle{Size: 22}}
	if got := GlyphSizeBits(run); got != math.Float32bits(22) {
		t.Fatalf("style size bits = %d, want 22", got)
	}

	run.Size = 18
	if got := GlyphSizeBits(run); got != math.Float32bits(18) {
		t.Fatalf("explicit size bits = %d, want 18", got)
	}
}

func TestGlyphAtlasKeyFromRunStability(t *testing.T) {
	reg := mustFontRegistry(t)
	regular := mustReadTestFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	bold := mustReadTestFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Bold.ttf")
	if err := reg.LoadFontBytes(regular, "noto-regular"); err != nil {
		t.Fatalf("load regular: %v", err)
	}
	if err := reg.LoadFontBytes(bold, "noto-bold"); err != nil {
		t.Fatalf("load bold: %v", err)
	}
	regularFace := reg.Resolve(text.TextStyle{Family: "Noto Sans", Size: 14})
	boldFace := reg.Resolve(text.TextStyle{Family: "Noto Sans", Weight: text.WeightBold, Size: 14})
	run := text.GlyphRun{Face: regularFace, Style: text.TextStyle{Size: 14}}
	key := GlyphAtlasKeyFromRun(run, 65)
	if key.FaceKey == 0 || key.GlyphID != 65 || key.SizeBits == 0 {
		t.Fatalf("unexpected key: %#v", key)
	}

	if got := GlyphAtlasKeyFromRun(run, 65); got != key {
		t.Fatalf("key not stable across repeated calls: %#v vs %#v", got, key)
	}
	if got := GlyphAtlasKeyFromRun(text.GlyphRun{Face: regularFace, Style: text.TextStyle{Size: 18}}, 65); got.SizeBits == key.SizeBits {
		t.Fatalf("expected size to affect atlas key: %#v vs %#v", got, key)
	}
	if got := GlyphAtlasKeyFromRun(text.GlyphRun{Face: boldFace, Style: text.TextStyle{Size: 14}}, 65); got.FaceKey == key.FaceKey {
		t.Fatalf("expected face to affect atlas key: %#v vs %#v", got, key)
	}
	if got := GlyphAtlasKeyFromRun(run, 66); got.GlyphID == key.GlyphID {
		t.Fatalf("expected glyph ID to affect atlas key: %#v vs %#v", got, key)
	}
}

func mustFontRegistry(t *testing.T) *text.FontRegistry {
	t.Helper()
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("NewFontRegistry: %v", err)
	}
	return reg
}

func mustReadTestFont(t *testing.T, rel string) []byte {
	t.Helper()
	path := mustTestFontPath(t, rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test font %q: %v", path, err)
	}
	return data
}

func mustTestFontPath(t *testing.T, rel string) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatalf("go env GOMODCACHE: %v", err)
	}
	return filepath.Join(string(bytes.TrimSpace(out)), rel)
}
