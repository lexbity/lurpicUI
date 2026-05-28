package testkit

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var updateGolden = flag.Bool("update-golden", false, "regenerate golden images")
var goldenBaseDir string

// AssertGolden compares the surface against testdata/golden/<name>.png.
func AssertGolden(t reporter, surface *MemorySurface, name string) {
	t.Helper()
	baseDir := resolveGoldenBaseDir(t)
	if os.Getenv("TESTKIT_GOLDEN_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "golden baseDir=%s name=%s update=%v\n", baseDir, name, *updateGolden)
	}
	wantPath := filepath.Join(baseDir, name+".png")
	actualPath := filepath.Join(baseDir, name+"_actual.png")

	if err := os.MkdirAll(filepath.Dir(wantPath), 0o755); err != nil {
		t.Fatalf("mkdir golden: %v", err)
	}

	got := surface.Capture()
	if *updateGolden {
		if os.Getenv("TESTKIT_GOLDEN_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "golden update name=%s got=%x %s\n", name, imageDigest(got), firstPixelValue(got))
		}
		writePNGOrFail(t, wantPath, got)
		return
	}

	want, err := readPNG(wantPath)
	if err != nil {
		writePNGOrFail(t, wantPath, got)
		return
	}
	if !imagesClose(got, want, 2) {
		if os.Getenv("TESTKIT_GOLDEN_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "golden diff name=%s bounds=%v want=%x got=%x %s\n", name, got.Bounds(), imageDigest(want), imageDigest(got), firstPixelDiff(want, got))
		}
		writePNGOrFail(t, actualPath, got)
		t.Errorf("golden mismatch for %s", name)
	}
}

func resolveGoldenBaseDir(t reporter) string {
	if goldenBaseDir != "" {
		return goldenBaseDir
	}
	pcs := make([]uintptr, 16)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if frame.File != "" &&
			strings.HasSuffix(frame.File, "_test.go") &&
			!strings.Contains(frame.File, string(filepath.Separator)+"internal"+string(filepath.Separator)+"testkit"+string(filepath.Separator)) {
			return filepath.Join(filepath.Dir(frame.File), "testdata", "golden", platformSuffix)
		}
		if !more {
			break
		}
	}
	return filepath.Join("testdata", "golden", platformSuffix)
}

var platformSuffix = func() string {
	goos := os.Getenv("GOOS")
	if goos == "" {
		goos = runtime.GOOS
	}
	return goos
}()

// GoldenBaseDirForCaller exposes the resolved golden directory for diagnostics and tests.
func GoldenBaseDirForCaller() string {
	return resolveGoldenBaseDir(nil)
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
	if err := png.Encode(f, flattenOnWhite(img)); err != nil {
		t.Fatalf("encode golden: %v", err)
	}
}

func imagesClose(a, b image.Image, tol uint8) bool {
	a = flattenOnWhite(a)
	b = flattenOnWhite(b)
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

func imageDigest(img image.Image) [32]byte {
	if img == nil {
		return [32]byte{}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return sha256.Sum256(buf.Bytes())
}

func firstPixelDiff(a, b image.Image) string {
	a = flattenOnWhite(a)
	b = flattenOnWhite(b)
	if a == nil || b == nil {
		return "nil-image"
	}
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				return fmt.Sprintf("first-diff=(%d,%d) want=(%d,%d,%d,%d) got=(%d,%d,%d,%d)",
					x, y, br>>8, bg>>8, bb>>8, ba>>8, ar>>8, ag>>8, ab>>8, aa>>8)
			}
		}
	}
	return "no-pixel-diff"
}

func firstPixelValue(img image.Image) string {
	if img == nil {
		return "nil-image"
	}
	r, g, b, a := img.At(img.Bounds().Min.X, img.Bounds().Min.Y).RGBA()
	return fmt.Sprintf("first-pixel=(%d,%d,%d,%d)", r>>8, g>>8, b>>8, a>>8)
}

func flattenOnWhite(img image.Image) image.Image {
	if img == nil {
		return nil
	}
	bounds := img.Bounds()
	out := image.NewRGBA(bounds)
	draw.Draw(out, bounds, &image.Uniform{C: image.White}, image.Point{}, draw.Src)
	draw.Draw(out, bounds, img, bounds.Min, draw.Over)
	return out
}
