package cook

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	gfxsvg "codeburg.org/lexbit/lurpicui/gfx/svg"
)

func TestCompileSVGLOD1Rasterizes32x32WithAA(t *testing.T) {
	src := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20">
		<circle cx="10" cy="10" r="7" fill="#ff0000"/>
	</svg>`)

	doc, err := gfxsvg.ParseSVG(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	lod1 := compileSVGLOD1(doc)
	if len(lod1) != 32*32*4 {
		t.Fatalf("unexpected lod1 byte length: %d", len(lod1))
	}

	img := &image.RGBA{
		Pix:    lod1,
		Stride: 32 * 4,
		Rect:   image.Rect(0, 0, 32, 32),
	}
	zero, full, partial := 0, 0, 0
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			switch {
			case a == 0:
				zero++
			case a == 0xffff:
				full++
			default:
				partial++
			}
		}
	}
	if zero == 0 || full == 0 || partial == 0 {
		t.Fatalf("expected transparent, solid, and antialiased pixels; got zero=%d full=%d partial=%d", zero, full, partial)
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Fatalf("expected transparent corner alpha, got %d", a)
	}
	if _, _, _, a := img.At(31, 0).RGBA(); a != 0 {
		t.Fatalf("expected transparent corner alpha, got %d", a)
	}

	dir := t.TempDir()
	out := filepath.Join(dir, "lod1.png")
	f, err := os.Create(out)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encode png: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close png: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("expected png file to exist: %v", err)
	}
}
