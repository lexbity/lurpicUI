package vulkan

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"math"

	"codeburg.org/lexbit/lurpicui/internal/renderutil"
	"codeburg.org/lexbit/lurpicui/text"
	gotextrender "github.com/go-text/render"
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	_ "golang.org/x/image/tiff"
	"golang.org/x/image/vector"
)

var _ = gotextrender.Renderer{}

type glyphEntry struct {
	bitmap  *image.Alpha
	offsetX float32
	offsetY float32
}

func uploadGlyphRun(run text.GlyphRun) error {
	if run.Face.CacheKey() == 0 || len(run.Glyphs) == 0 {
		return nil
	}
	sizeBits := renderutil.GlyphSizeBits(run)
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
		extents, ok := goFace.GlyphExtents(gid)
		return rasterizeOutlineGlyph(gd, extents, ok, scale)
	case font.GlyphSVG:
		extents, ok := goFace.GlyphExtents(gid)
		return rasterizeOutlineGlyph(gd.Outline, extents, ok, scale)
	case font.GlyphBitmap:
		extents, ok := goFace.GlyphExtents(gid)
		return rasterizeBitmapGlyph(gd, extents, ok, scale)
	default:
		return nil
	}
}

func rasterizeOutlineGlyph(outline font.GlyphOutline, extents font.GlyphExtents, hasExtents bool, scale float32) *glyphEntry {
	if len(outline.Segments) == 0 {
		return &glyphEntry{bitmap: image.NewAlpha(image.Rect(0, 0, 1, 1))}
	}
	minX, minY, maxX, maxY := outlineBounds(outline, scale)
	originX := float32(math.Floor(float64(minX)))
	originY := float32(math.Floor(float64(minY)))
	w := maxInt(1, int(math.Ceil(float64(maxX)))-int(originX)+2)
	h := maxInt(1, int(math.Ceil(float64(maxY)))-int(originY)+2)
	bmp := image.NewAlpha(image.Rect(0, 0, w, h))
	ras := vector.NewRasterizer(w, h)
	ras.DrawOp = draw.Src
	for _, seg := range outline.Segments {
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			ras.MoveTo(seg.Args[0].X*scale-originX+1, float32(h)-(seg.Args[0].Y*scale-originY+1))
		case ot.SegmentOpLineTo:
			ras.LineTo(seg.Args[0].X*scale-originX+1, float32(h)-(seg.Args[0].Y*scale-originY+1))
		case ot.SegmentOpQuadTo:
			ras.QuadTo(
				seg.Args[0].X*scale-originX+1, float32(h)-(seg.Args[0].Y*scale-originY+1),
				seg.Args[1].X*scale-originX+1, float32(h)-(seg.Args[1].Y*scale-originY+1),
			)
		case ot.SegmentOpCubeTo:
			ras.CubeTo(
				seg.Args[0].X*scale-originX+1, float32(h)-(seg.Args[0].Y*scale-originY+1),
				seg.Args[1].X*scale-originX+1, float32(h)-(seg.Args[1].Y*scale-originY+1),
				seg.Args[2].X*scale-originX+1, float32(h)-(seg.Args[2].Y*scale-originY+1),
			)
		}
	}
	ras.Draw(bmp, bmp.Bounds(), image.NewUniform(color.Alpha{A: 255}), image.Point{})
	entry := &glyphEntry{bitmap: bmp}
	entry.offsetX = originX - 1
	entry.offsetY = -originY - float32(h) + 1
	return entry
}

func rasterizeBitmapGlyph(gd font.GlyphBitmap, extents font.GlyphExtents, hasExtents bool, scale float32) *glyphEntry {
	if gd.Width <= 0 || gd.Height <= 0 {
		return &glyphEntry{bitmap: image.NewAlpha(image.Rect(0, 0, 1, 1))}
	}

	bmp := image.NewAlpha(image.Rect(0, 0, gd.Width, gd.Height))
	switch gd.Format {
	case font.BlackAndWhite:
		if len(gd.Data) < ((gd.Width*gd.Height)+7)/8 {
			return &glyphEntry{bitmap: bmp}
		}
		for y := 0; y < gd.Height; y++ {
			for x := 0; x < gd.Width; x++ {
				idx := y*gd.Width + x
				bit := gd.Data[idx/8] >> uint(7-(idx%8))
				if bit&1 == 1 {
					bmp.SetAlpha(x, y, color.Alpha{A: 255})
				}
			}
		}
	default:
		src, _, err := image.Decode(bytes.NewReader(gd.Data))
		if err != nil {
			return &glyphEntry{bitmap: bmp}
		}
		bounds := src.Bounds()
		if bounds.Empty() {
			return &glyphEntry{bitmap: bmp}
		}
		bmp = image.NewAlpha(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				bmp.SetAlpha(x-bounds.Min.X, y-bounds.Min.Y, color.Alpha{A: alphaFromColor(src.At(x, y))})
			}
		}
	}

	entry := &glyphEntry{bitmap: bmp}
	if gd.Outline != nil {
		minX, _, _, maxY := outlineBounds(*gd.Outline, scale)
		entry.offsetX = minX
		entry.offsetY = -maxY
		return entry
	}
	if hasExtents {
		entry.offsetX = extents.XBearing * scale
		entry.offsetY = -extents.YBearing * scale
	}
	return entry
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

func alphaFromColor(c color.Color) uint8 {
	_, _, _, a := c.RGBA()
	if a != 0 {
		return uint8(a >> 8) //nolint:gosec // integer overflow conversion
	}
	r, g, b, _ := c.RGBA()
	gray := (r*299 + g*587 + b*114) / 1000
	return uint8(gray >> 8) //nolint:gosec // integer overflow conversion
}
