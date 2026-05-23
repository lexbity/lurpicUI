package cook

import (
	"bytes"
	"context"
	"fmt"
	"image"
	imagedraw "image/draw"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"

	xdraw "golang.org/x/image/draw"
)

// ImageCompiler invokes basisu to produce three KTX2 LODs at 100%, 50%, and 25%.
type ImageCompiler struct {
	// BasisuPath optionally overrides the basisu binary path. When empty, PATH lookup is used.
	BasisuPath string
	// TempDir optionally overrides the parent directory used for staging files.
	TempDir string
}

// Extensions reports the handled source file extensions.
func (c *ImageCompiler) Extensions() []string {
	return []string{".png", ".jpg", ".jpeg"}
}

// Compile decodes src, rescales it to three target resolutions, and runs basisu for each LOD.
func (c *ImageCompiler) Compile(src []byte, target Platform) ([]CompiledLOD, error) {
	_ = target

	srcImg, _, err := image.Decode(bytesReader(src))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	basisuPath, err := c.resolveBasisuPath()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		level int
		data  []byte
		err   error
	}

	workDir, err := os.MkdirTemp(c.tempRoot(), "image-compile-*")
	if err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	scales := []struct {
		level int
		scale float64
	}{
		{level: 0, scale: 1.0},
		{level: 1, scale: 0.5},
		{level: 2, scale: 0.25},
	}

	results := make(chan result, len(scales))
	var wg sync.WaitGroup
	for _, spec := range scales {
		spec := spec
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := c.compileImageLOD(ctx, basisuPath, workDir, srcImg, spec.level, spec.scale)
			results <- result{level: spec.level, data: data, err: err}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	lods := make([]CompiledLOD, len(scales))
	var firstErr error
	for res := range results {
		if res.err != nil && firstErr == nil {
			firstErr = res.err
			cancel()
		}
		if res.err == nil {
			lods[res.level] = CompiledLOD{Level: res.level, Data: res.data}
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}

	sort.Slice(lods, func(i, j int) bool { return lods[i].Level < lods[j].Level })
	return lods, nil
}

func (c *ImageCompiler) compileImageLOD(ctx context.Context, basisuPath, workDir string, src image.Image, level int, scale float64) ([]byte, error) {
	lodDir, err := os.MkdirTemp(workDir, fmt.Sprintf("lod-%d-*", level))
	if err != nil {
		return nil, fmt.Errorf("create lod dir: %w", err)
	}
	defer os.RemoveAll(lodDir)

	resized := resizeImage(src, scale)
	inputPath := filepath.Join(lodDir, "input.png")
	outputPath := filepath.Join(lodDir, "output.ktx2")

	inFile, err := os.Create(inputPath)
	if err != nil {
		return nil, fmt.Errorf("create input file: %w", err)
	}
	if err := png.Encode(inFile, resized); err != nil {
		inFile.Close()
		return nil, fmt.Errorf("encode input png: %w", err)
	}
	if err := inFile.Close(); err != nil {
		return nil, fmt.Errorf("close input file: %w", err)
	}

	args := []string{
		inputPath,
		"-output_file", outputPath,
		"-tex_type", "2d",
	}
	cmd := exec.CommandContext(ctx, basisuPath, args...)
	cmd.Dir = lodDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("basisu level %d: %w: %s", level, err, string(out))
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read output file: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("basisu produced empty output for level %d", level)
	}
	return append([]byte(nil), data...), nil
}

func (c *ImageCompiler) resolveBasisuPath() (string, error) {
	if c.BasisuPath != "" {
		return c.BasisuPath, nil
	}
	path, err := exec.LookPath("basisu")
	if err != nil {
		return "", fmt.Errorf("look up basisu: %w", err)
	}
	return path, nil
}

func (c *ImageCompiler) tempRoot() string {
	if c.TempDir != "" {
		return c.TempDir
	}
	return ""
}

func resizeImage(src image.Image, scale float64) *image.RGBA {
	bounds := src.Bounds()
	w := scaledDimension(bounds.Dx(), scale)
	h := scaledDimension(bounds.Dy(), scale)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if w == bounds.Dx() && h == bounds.Dy() {
		imagedraw.Draw(dst, dst.Bounds(), src, bounds.Min, imagedraw.Src)
		return dst
	}
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, imagedraw.Over, nil)
	return dst
}

func scaledDimension(v int, scale float64) int {
	if v <= 0 {
		return 1
	}
	s := int(float64(v) * scale)
	if s < 1 {
		s = 1
	}
	return s
}

func bytesReader(src []byte) *bytes.Reader {
	return bytes.NewReader(src)
}
