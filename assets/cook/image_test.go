package cook

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageCompilerCompile(t *testing.T) {
	basisuPath := buildBasisuFixture(t)
	workDir := t.TempDir()

	compiler := &ImageCompiler{
		BasisuPath: basisuPath,
		TempDir:    workDir,
	}
	if got := compiler.Extensions(); len(got) != 3 || got[0] != ".png" || got[1] != ".jpg" || got[2] != ".jpeg" {
		t.Fatalf("unexpected extensions: %v", got)
	}

	src := makeTestImage(t, 16, 8)
	lods, err := compiler.Compile(src, PlatformLinux)
	if err != nil {
		t.Fatalf("compile image: %v", err)
	}
	if len(lods) != 3 {
		t.Fatalf("unexpected lod count: %d", len(lods))
	}
	if lods[0].Level != 0 || lods[1].Level != 1 || lods[2].Level != 2 {
		t.Fatalf("unexpected lod levels: %+v", lods)
	}

	wantDims := []string{"16x8", "8x4", "4x2"}
	for i, want := range wantDims {
		if !bytes.Contains(lods[i].Data, []byte(want)) {
			t.Fatalf("lod %d missing dimension marker %q in %q", i, want, string(lods[i].Data))
		}
	}
	if !(len(lods[0].Data) > len(lods[1].Data) && len(lods[1].Data) > len(lods[2].Data)) {
		t.Fatalf("unexpected lod sizes: %d %d %d", len(lods[0].Data), len(lods[1].Data), len(lods[2].Data))
	}

	if entries, err := os.ReadDir(workDir); err != nil {
		t.Fatalf("read work dir: %v", err)
	} else if len(entries) != 0 {
		t.Fatalf("expected work dir cleanup, found %d entries", len(entries))
	}
}

func buildBasisuFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	src := filepath.Join(dir, "basisu.go")
	bin := filepath.Join(dir, "basisu")
	code := `package main

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
)

func main() {
	var input, output string
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-output_file":
			i++
			if i >= len(os.Args) {
				fmt.Fprintln(os.Stderr, "missing output_file")
				os.Exit(2)
			}
			output = os.Args[i]
		case "-tex_type":
			i++
		default:
			if len(os.Args[i]) > 0 && os.Args[i][0] != '-' && input == "" {
				input = os.Args[i]
			}
		}
	}
	if input == "" || output == "" {
		fmt.Fprintln(os.Stderr, "missing input or output")
		os.Exit(2)
	}
	f, err := os.Open(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b := img.Bounds()
	payload := append([]byte(fmt.Sprintf("KTX2:%dx%d\n", b.Dx(), b.Dy())), bytes.Repeat([]byte{byte(b.Dx() + b.Dy())}, b.Dx()*b.Dy())...)
	if err := os.WriteFile(output, payload, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatalf("write helper source: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v: %s", err, string(out))
	}
	return bin
}

func makeTestImage(t *testing.T, w, h int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x * 16),
				G: uint8(y * 24),
				B: uint8((x + y) * 8),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode source image: %v", err)
	}
	return buf.Bytes()
}

func TestImageCompilerCompileRequiresBasisu(t *testing.T) {
	compiler := &ImageCompiler{}
	if _, err := compiler.resolveBasisuPath(); err == nil {
		t.Fatal("expected basisu lookup failure without binary")
	} else if !strings.Contains(err.Error(), "basisu") {
		t.Fatalf("unexpected error: %v", err)
	}
}
