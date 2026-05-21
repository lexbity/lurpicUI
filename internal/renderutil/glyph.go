package renderutil

import (
	"math"

	"codeburg.org/lexbit/lurpicui/text"
)

type GlyphAtlasKey struct {
	FaceKey  uint64
	GlyphID  uint32
	SizeBits uint32
}

func GlyphSizeBits(run text.GlyphRun) uint32 {
	size := run.Size
	if size <= 0 {
		size = run.Style.Size
	}
	if size <= 0 {
		size = 14
	}
	return math.Float32bits(size)
}

func GlyphAtlasKeyFromRun(run text.GlyphRun, glyphID uint32) GlyphAtlasKey {
	return GlyphAtlasKey{
		FaceKey:  run.Face.CacheKey(),
		GlyphID:  glyphID,
		SizeBits: GlyphSizeBits(run),
	}
}
