package testkit

import (
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

var updateGolden = flag.Bool("update-golden", false, "regenerate golden images")
var goldenBaseDir = filepath.Join("testdata", "golden")

// AssertGolden compares the surface against testdata/golden/<name>.png.
func AssertGolden(t reporter, surface *MemorySurface, name string) {
	t.Helper()
	wantPath := filepath.Join(goldenBaseDir, name+".png")
	actualPath := filepath.Join(goldenBaseDir, name+"_actual.png")

	if err := os.MkdirAll(filepath.Dir(wantPath), 0o755); err != nil {
		t.Fatalf("mkdir golden: %v", err)
	}

	got := surface.Capture()
	if *updateGolden {
		writePNGOrFail(t, wantPath, got)
		return
	}

	want, err := readPNG(wantPath)
	if err != nil {
		writePNGOrFail(t, wantPath, got)
		return
	}
	if !imagesClose(got, want, 2) {
		writePNGOrFail(t, actualPath, got)
		t.Errorf("golden mismatch for %s", name)
	}
}

func readPNG(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	out := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			out.Set(x, y, img.At(x, y))
		}
	}
	return out, nil
}

func writePNGOrFail(t reporter, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create golden: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode golden: %v", err)
	}
}

func imagesClose(a, b image.Image, tol uint8) bool {
	if !a.Bounds().Eq(b.Bounds()) {
		return false
	}
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if abs16(uint16(ar>>8), uint16(br>>8)) > uint16(tol) ||
				abs16(uint16(ag>>8), uint16(bg>>8)) > uint16(tol) ||
				abs16(uint16(ab>>8), uint16(bb>>8)) > uint16(tol) ||
				abs16(uint16(aa>>8), uint16(ba>>8)) > uint16(tol) {
				return false
			}
		}
	}
	return true
}

func abs16(a, b uint16) uint16 {
	if a > b {
		return a - b
	}
	return b - a
}
