package basic

import (
	"fmt"
	"image"
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

// FitMode selects how image content maps into the declared bounds.
type FitMode uint8

const (
	FitContain FitMode = iota
	FitCover
	FitFill
	FitNone
)

// ImageSource abstracts a decoded image source.
type ImageSource interface {
	Image() *image.RGBA
	IntrinsicSize() gfx.Size
	CacheKey() string
}

// RGBAImageSource adapts a decoded RGBA image.
type RGBAImageSource struct {
	ImageRef *image.RGBA
	Key      string
}

func (s RGBAImageSource) Image() *image.RGBA { return s.ImageRef }
func (s RGBAImageSource) IntrinsicSize() gfx.Size {
	if s.ImageRef == nil {
		return gfx.Size{}
	}
	return gfx.Size{W: float32(s.ImageRef.Bounds().Dx()), H: float32(s.ImageRef.Bounds().Dy())}
}
func (s RGBAImageSource) CacheKey() string {
	if s.Key != "" {
		return s.Key
	}
	if s.ImageRef == nil {
		return ""
	}
	return "rgba"
}

// Image is a primitive authored image mark.
type Image struct {
	ID      string
	Source  ImageSource
	Bounds  BoundsProps
	Fit     FitMode
	Clip    bool
	Opacity float32
	Tint    *theme.Material
	Tx      TransformProps

	base           primitiveFacet
	once           sync.Once
	layoutRole     *facet.LayoutRole
	viewportRole   *facet.ViewportRole
	projectionRole *facet.ProjectionRole
	hitRole        *facet.HitRole
}

func init() {
	registerPrimitiveDescriptor(marks.Descriptor{
		Family:            marks.FamilyBasic,
		ConstructionClass: marks.ConstructionPrimitive,
		Type:              marks.TypeName("basic:image"),
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

func (i *Image) Base() *facet.Facet               { i.ensureInit(); return i.base.Base() }
func (i *Image) Descriptor() marks.Descriptor     { i.ensureInit(); return i.base.Descriptor() }
func (i *Image) AuthoredID() string               { return i.ID }
func (i *Image) OnAttach(ctx facet.AttachContext) { i.syncRoles() }
func (i *Image) OnDetach()                        {}
func (i *Image) OnActivate()                      {}
func (i *Image) OnDeactivate()                    {}

func (i *Image) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	i.ensureInit()
	fit := i.resolveFit()
	anchors := layout.AnchorSet{
		"bounds-center": {X: fit.content.Min.X + fit.content.Width()/2, Y: fit.content.Min.Y + fit.content.Height()/2},
		"top-left":      {X: fit.content.Min.X, Y: fit.content.Min.Y},
		"top-right":     {X: fit.content.Max.X, Y: fit.content.Min.Y},
		"bottom-right":  {X: fit.content.Max.X, Y: fit.content.Max.Y},
		"bottom-left":   {X: fit.content.Min.X, Y: fit.content.Max.Y},
	}
	transform := normalizeTransform(i.Tx.Transform)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, anchors)
}

func (i *Image) HitTest(world gfx.Point) bool {
	i.ensureInit()
	inv, ok := inverseTransform(i.Tx)
	if !ok {
		return false
	}
	return i.hitTestLocal(inv.TransformPoint(world))
}

func (i *Image) ensureInit() {
	i.once.Do(func() {
		i.base.descriptor = marks.Descriptor{Family: marks.FamilyBasic, ConstructionClass: marks.ConstructionPrimitive, Type: marks.TypeName("basic:image"), HitTestable: true, AnchorExporting: true}
		i.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			fit := i.resolveFit()
			return gfx.Size{W: fit.content.Width(), H: fit.content.Height()}
		}}
		i.viewportRole = &facet.ViewportRole{Transform: normalizeTransform(i.Tx.Transform)}
		i.projectionRole = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return i.project(ctx) }}
		i.hitRole = &facet.HitRole{OnHitTest: func(pt gfx.Point) facet.HitResult {
			if i.hitTestLocal(pt) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		attachPrimitiveRoles(&i.base, i.layoutRole, i.viewportRole, i.projectionRole, i.hitRole)
		syncLayout(i.layoutRole, i.localBounds())
		syncViewport(i.viewportRole, normalizeTransform(i.Tx.Transform))
	})
}

func (i *Image) syncRoles() {
	syncLayout(i.layoutRole, i.localBounds())
	syncViewport(i.viewportRole, normalizeTransform(i.Tx.Transform))
}

func (i *Image) localBounds() gfx.Rect {
	return i.resolveFit().content
}

func (i *Image) project(ctx facet.ProjectionContext) *gfx.CommandList {
	src := i.image()
	if src == nil {
		return &gfx.CommandList{}
	}
	fit := i.resolveFit()
	var list gfx.CommandList
	if i.Clip {
		list.Add(gfx.PushClipRect{Rect: fit.hit})
		defer list.Add(gfx.PopClip{})
	}
	if i.Opacity <= 0 {
		return &list
	}
	list.Add(gfx.DrawImage{
		Image:    src,
		DestRect: fit.content,
		SrcRect:  gfx.RectFromXYWH(0, 0, float32(src.Bounds().Dx()), float32(src.Bounds().Dy())),
		Sampling: gfx.SamplingBilinear,
		Opacity:  i.Opacity,
	})
	return &list
}

func (i *Image) hitTestLocal(pt gfx.Point) bool {
	fit := i.resolveFit()
	if i.Clip {
		return fit.hit.Contains(pt)
	}
	return fit.content.Contains(pt)
}

func (i *Image) image() *image.RGBA {
	if i.Source == nil {
		return nil
	}
	return i.Source.Image()
}

func (i *Image) resolveFit() fitRect {
	src := i.image()
	bounds := i.Bounds.Rect()
	if bounds.IsEmpty() && src != nil {
		bounds = gfx.RectFromXYWH(0, 0, float32(src.Bounds().Dx()), float32(src.Bounds().Dy()))
	}
	key := i.fitCacheKey()
	return cachedImageFit(key, func() fitRect {
		if src == nil {
			return fitRect{content: bounds, hit: bounds}
		}
		srcW := float32(src.Bounds().Dx())
		srcH := float32(src.Bounds().Dy())
		if srcW <= 0 || srcH <= 0 {
			return fitRect{content: bounds, hit: bounds}
		}
		switch i.Fit {
		case FitFill:
			return fitRect{content: bounds, hit: bounds}
		case FitNone:
			content := gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, srcW, srcH)
			return fitRect{content: content, hit: hitRectFor(i.Clip, bounds, content)}
		case FitCover:
			scale := max(bounds.Width()/srcW, bounds.Height()/srcH)
			w := srcW * scale
			h := srcH * scale
			content := centeredRect(bounds, w, h)
			return fitRect{content: content, hit: hitRectFor(i.Clip, bounds, content)}
		default:
			scale := min(bounds.Width()/srcW, bounds.Height()/srcH)
			w := srcW * scale
			h := srcH * scale
			content := centeredRect(bounds, w, h)
			return fitRect{content: content, hit: hitRectFor(i.Clip, bounds, content)}
		}
	})
}

func (i *Image) fitCacheKey() string {
	srcKey := ""
	if i.Source != nil {
		srcKey = i.Source.CacheKey()
	}
	return srcKey + "|" + i.BoundsKey()
}

func (i *Image) BoundsKey() string {
	return fmt.Sprintf("%g,%g,%g,%g|%d|%t|%g", i.Bounds.X, i.Bounds.Y, i.Bounds.W, i.Bounds.H, i.Fit, i.Clip, i.Opacity)
}

func centeredRect(bounds gfx.Rect, w, h float32) gfx.Rect {
	x := bounds.Min.X + (bounds.Width()-w)/2
	y := bounds.Min.Y + (bounds.Height()-h)/2
	return gfx.RectFromXYWH(x, y, w, h)
}

func hitRectFor(clipped bool, bounds, content gfx.Rect) gfx.Rect {
	if clipped {
		return bounds
	}
	return content
}
