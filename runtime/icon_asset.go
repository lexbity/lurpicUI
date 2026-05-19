package runtime

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/hashutil"
	"codeburg.org/lexbit/lurpicui/theme"
)

// IconAsset describes a resolved vector icon asset.
//
// IconAsset values are treated as immutable snapshots. Callers should copy or
// clone the returned value before retaining it across resolver boundaries.
type IconAsset struct {
	SourceRef string
	Revision  uint64
	Path      gfx.Path
	ViewBox   gfx.Rect
}

// IconCacheInputs captures the projection inputs that affect icon cache keys.
type IconCacheInputs struct {
	ThemeVersion        uint64
	ColorSlot           theme.ColorToken
	Density             theme.DensityScale
	ContentScale        float32
	PreserveAspectRatio string
}

// IconResolver resolves icon asset references for icon-bearing marks.
type IconResolver interface {
	ResolveIcon(ref string) (IconAsset, bool)
}

// NewIconAsset constructs an immutable icon asset snapshot.
func NewIconAsset(sourceRef string, revision uint64, path gfx.Path, viewBox gfx.Rect) IconAsset {
	return IconAsset{
		SourceRef: sourceRef,
		Revision:  revision,
		Path:      clonePath(path),
		ViewBox:   viewBox,
	}
}

// Clone returns a deep copy of the icon asset.
func (a IconAsset) Clone() IconAsset {
	a.Path = clonePath(a.Path)
	return a
}

// IsZero reports whether the asset carries any usable geometry.
func (a IconAsset) IsZero() bool {
	return a.SourceRef == "" && a.Revision == 0 && len(a.Path.Segments) == 0 && a.ViewBox.IsEmpty()
}

// CacheKey computes a stable cache key for the asset and the supplied inputs.
func (a IconAsset) CacheKey(inputs IconCacheInputs) uint64 {
	b := hashutil.NewCacheKeyBuilder()
	b.WriteString("runtime.IconAsset")
	b.WriteString(a.SourceRef)
	b.WriteUint64(a.Revision)
	hashPath(&b, a.Path)
	hashRect(&b, a.ViewBox)
	b.WriteUint64(inputs.ThemeVersion)
	b.WriteUint8(uint8(inputs.ColorSlot))
	b.WriteString(string(inputs.Density.ID))
	b.WriteFloat32(inputs.Density.Factor)
	b.WriteFloat32(inputs.ContentScale)
	b.WriteString(inputs.PreserveAspectRatio)
	return b.Sum()
}

func clonePath(path gfx.Path) gfx.Path {
	if len(path.Segments) == 0 {
		return gfx.Path{}
	}
	segments := make([]gfx.PathSegment, len(path.Segments))
	copy(segments, path.Segments)
	return gfx.Path{Segments: segments}
}

func hashPath(b *hashutil.CacheKeyBuilder, path gfx.Path) {
	b.WriteUint64(uint64(len(path.Segments)))
	for _, seg := range path.Segments {
		b.WriteUint8(uint8(seg.Verb))
		for _, p := range seg.Pts {
			hashPoint(b, p)
		}
	}
}

func hashPoint(b *hashutil.CacheKeyBuilder, p gfx.Point) {
	b.WriteFloat32(p.X)
	b.WriteFloat32(p.Y)
}

func hashRect(b *hashutil.CacheKeyBuilder, r gfx.Rect) {
	hashPoint(b, r.Min)
	hashPoint(b, r.Max)
}
