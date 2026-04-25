package vulkan

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sync"

	"codeburg.org/lexbit/lurpicui/text"
	gotextrender "github.com/go-text/render"
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"golang.org/x/image/vector"
)

var _ = gotextrender.Renderer{}

type glyphKey struct {
	glyphID  uint32
	faceKey  uint64
	sizeBits uint32
}

type glyphEntry struct {
	bitmap  *image.Alpha
	offsetX float32
	offsetY float32
}

type glyphAtlas struct {
	mu             sync.Mutex
	entries        map[glyphKey]*glyphEntry
	rasterizeCount int
}

func newGlyphAtlas() *glyphAtlas {
	return &glyphAtlas{entries: make(map[glyphKey]*glyphEntry)}
}

func (a *glyphAtlas) getOrRasterize(run text.GlyphRun, glyph text.PositionedGlyph) *glyphEntry {
	if a == nil {
		return nil
	}
	size := run.Size
	if size <= 0 {
		size = run.Style.Size
	}
	if size <= 0 {
		size = 14
	}
	key := glyphKey{
		glyphID:  glyph.GlyphID,
		faceKey:  run.Face.CacheKey(),
		sizeBits: math.Float32bits(size),
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if entry := a.entries[key]; entry != nil {
		return entry
	}
	entry := rasterizeGlyphEntry(run, glyph, size)
	if entry == nil {
		return nil
	}
	a.entries[key] = entry
	a.rasterizeCount++
	return entry
}

func uploadGlyphRun(run text.GlyphRun) error {
	if run.Face.CacheKey() == 0 || len(run.Glyphs) == 0 {
		return nil
	}
	size := run.Size
	if size <= 0 {
		size = run.Style.Size
	}
	if size <= 0 {
		size = 14
	}
	sizeBits := math.Float32bits(size)
	for _, glyph := range run.Glyphs {
		if err := uploadGlyphEntry(run, glyph, sizeBits); err != nil {
			return err
		}
	}
	return nil
}

func uploadGlyphEntry(run text.GlyphRun, glyph text.PositionedGlyph, sizeBits uint32) error {
	entry := rasterizeGlyphEntry(run, glyph, math.Float32frombits(sizeBits))
	if entry == nil || entry.bitmap == nil {
		return nil
	}
	return UploadGlyph(
		run.Face.CacheKey(),
		glyph.GlyphID,
		sizeBits,
		entry.bitmap.Rect.Dx(),
		entry.bitmap.Rect.Dy(),
		entry.offsetX,
		entry.offsetY,
		glyph.Advance,
		append([]byte(nil), entry.bitmap.Pix...),
	)
}

func rasterizeGlyphEntry(run text.GlyphRun, glyph text.PositionedGlyph, size float32) *glyphEntry {
	goFace := run.Face.GoFace()
	if goFace == nil {
		return nil
	}
	gid := font.GID(glyph.GlyphID)
	data := goFace.GlyphData(gid)
	if data == nil {
		return nil
	}
	scale := size / float32(goFace.Upem())
	if scale <= 0 {
		scale = 1
	}

	switch gd := data.(type) {
	case font.GlyphOutline:
		return rasterizeOutlineGlyph(gd, scale)
	case font.GlyphSVG:
		return rasterizeOutlineGlyph(gd.Outline, scale)
	case font.GlyphBitmap:
		return rasterizeBitmapGlyph(gd, scale)
	default:
		return nil
	}
}

func rasterizeOutlineGlyph(outline font.GlyphOutline, scale float32) *glyphEntry {
	if len(outline.Segments) == 0 {
		return &glyphEntry{bitmap: image.NewAlpha(image.Rect(0, 0, 1, 1))}
	}
	minX, minY, maxX, maxY := outlineBounds(outline, scale)
	w := maxInt(1, int(math.Ceil(float64(maxX-minX)))+2)
	h := maxInt(1, int(math.Ceil(float64(maxY-minY)))+2)
	bmp := image.NewAlpha(image.Rect(0, 0, w, h))
	ras := vector.NewRasterizer(w, h)
	ras.DrawOp = draw.Src
	for _, seg := range outline.Segments {
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			ras.MoveTo(seg.Args[0].X*scale-minX+1, float32(h)-(seg.Args[0].Y*scale-minY+1))
		case ot.SegmentOpLineTo:
			ras.LineTo(seg.Args[0].X*scale-minX+1, float32(h)-(seg.Args[0].Y*scale-minY+1))
		case ot.SegmentOpQuadTo:
			ras.QuadTo(
				seg.Args[0].X*scale-minX+1, float32(h)-(seg.Args[0].Y*scale-minY+1),
				seg.Args[1].X*scale-minX+1, float32(h)-(seg.Args[1].Y*scale-minY+1),
			)
		case ot.SegmentOpCubeTo:
			ras.CubeTo(
				seg.Args[0].X*scale-minX+1, float32(h)-(seg.Args[0].Y*scale-minY+1),
				seg.Args[1].X*scale-minX+1, float32(h)-(seg.Args[1].Y*scale-minY+1),
				seg.Args[2].X*scale-minX+1, float32(h)-(seg.Args[2].Y*scale-minY+1),
			)
		}
	}
	ras.Draw(bmp, bmp.Bounds(), image.NewUniform(color.Alpha{A: 255}), image.Point{})
	return &glyphEntry{
		bitmap:  bmp,
		offsetX: minX - 1,
		offsetY: -maxY + 1,
	}
}

func rasterizeBitmapGlyph(gd font.GlyphBitmap, scale float32) *glyphEntry {
	w := maxInt(1, int(math.Ceil(float64(float32(gd.Width)*scale)))+2)
	h := maxInt(1, int(math.Ceil(float64(float32(gd.Height)*scale)))+2)
	bmp := image.NewAlpha(image.Rect(0, 0, w, h))
	if gd.Width <= 0 || gd.Height <= 0 {
		return &glyphEntry{bitmap: bmp}
	}
	if gd.Format == font.BlackAndWhite {
		for y := 0; y < h; y++ {
			sy := clampInt(int(float32(y-1)/scale), 0, gd.Height-1)
			for x := 0; x < w; x++ {
				sx := clampInt(int(float32(x-1)/scale), 0, gd.Width-1)
				idx := sy*gd.Width + sx
				if idx < 0 || idx >= gd.Width*gd.Height {
					continue
				}
				bit := gd.Data[idx/8] >> uint(7-(idx%8))
				if bit&1 == 1 {
					bmp.SetAlpha(x, y, color.Alpha{A: 255})
				}
			}
		}
		return &glyphEntry{bitmap: bmp}
	}
	return &glyphEntry{bitmap: bmp}
}

func outlineBounds(outline font.GlyphOutline, scale float32) (minX, minY, maxX, maxY float32) {
	first := true
	for _, seg := range outline.Segments {
		for _, pt := range seg.Args {
			x := pt.X * scale
			y := pt.Y * scale
			if first {
				minX, maxX = x, x
				minY, maxY = y, y
				first = false
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if first {
		return 0, 0, 0, 0
	}
	return
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
