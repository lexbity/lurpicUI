package primitive

import (
	"fmt"
	"image"
	"math"
	"strings"

	"codeburg.org/lexbit/lurpicui/assets"
	csg "codeburg.org/lexbit/lurpicui/assets/schema/lurpic/csg"
	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	svgnorm "codeburg.org/lexbit/lurpicui/gfx/svg"
	"codeburg.org/lexbit/lurpicui/layout"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/theme"
)

// IconDensityBehavior describes how the authored icon size reacts to density.
type IconDensityBehavior uint8

const (
	IconDensityScaleWithDensity IconDensityBehavior = iota
	IconDensityLockLogicalSize
	IconDensityTouchAware
	IconDensitySnapToDevicePixels
)

// IconSource is the canonical icon source contract.
type IconSource interface {
	isIconSource()
}

// IconRef resolves through the runtime icon resolver.
type IconRef string

// IconSVG carries inline SVG source for fully authored icon geometry.
type IconSVG string

func (IconRef) isIconSource() {}
func (IconSVG) isIconSource() {}

// IconAssetPath resolves through the runtime asset manager and supports progressive LODs.
type IconAssetPath string

func (IconAssetPath) isIconSource() {}

const (
	IconMarkIDRoot    facet.MarkID = 1
	IconMarkIDContent facet.MarkID = 2
	IconMarkIDHit     facet.MarkID = 3
)

// Icon implements the primitive.icon standard mark.
type Icon struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole

	Source              IconSource
	Size                float32
	ColorSlot           theme.ColorToken
	DensityBehavior     IconDensityBehavior
	AccessibleLabel     string
	Decorative          bool
	HitPadding          float32
	PreserveAspectRatio svgnorm.SVGPreserveAspectRatio

	cachedSize      gfx.Size
	cachedSourceKey string
	cachedSource    iconResolvedSource
	cachedColor     gfx.Color
	cachedCommands  []gfx.Command
	cachedTouchPad  float32
}

type iconResolvedSourceKind uint8

const (
	iconSourceNone iconResolvedSourceKind = iota
	iconSourceAsset
	iconSourceManagedAsset
	iconSourceSVG
)

type iconResolvedSource struct {
	kind    iconResolvedSourceKind
	asset   runtimepkg.IconAsset
	managed assets.Handle
	doc     svgnorm.SVGDocument
	box     gfx.Rect
	key     string
}

var _ facet.FacetImpl = (*Icon)(nil)
var _ layout.AnchorExporter = (*Icon)(nil)

// NewIcon constructs a primitive.icon mark with canonical defaults.
func NewIcon(source IconSource) *Icon {
	i := &Icon{
		Facet:               facet.NewFacet(),
		Source:              source,
		ColorSlot:           theme.ColorText,
		DensityBehavior:     IconDensityScaleWithDensity,
		Decorative:          true,
		PreserveAspectRatio: defaultIconPreserveAspectRatio(),
	}
	i.layoutRole.Parent = facet.GroupParentContract{Kind: facet.GroupLayoutNone}
	i.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := i.measureSize(ctx, constraints)
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionClip,
			BelowMinHeight: facet.CompressionClip,
			AboveMaxWidth:  facet.ExpansionClip,
			AboveMaxHeight: facet.ExpansionClip,
		},
		Stretch: facet.StretchPolicy{
			Width:  facet.StretchNever,
			Height: facet.StretchNever,
		},
		Baseline: facet.BaselineNone,
	}
	i.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return i.measure(ctx, constraints)
	}
	i.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		i.layoutRole.ArrangedBounds = bounds
	}
	i.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := i.buildCommands(bounds, nil)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	i.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := i.buildCommands(i.layoutRole.ArrangedBounds, ctx.Runtime)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	i.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult {
		return i.hitTest(p)
	}
	i.AddRole(&i.layoutRole)
	i.AddRole(&i.renderRole)
	i.AddRole(&i.projectionRole)
	i.AddRole(&i.hitRole)
	return i
}

// Base satisfies facet.FacetImpl.
func (i *Icon) Base() *facet.Facet {
	i.Facet.BindImpl(i)
	return &i.Facet
}

// AccessibilityRole reports the semantic role required by the mark.
func (i *Icon) AccessibilityRole() string {
	if i == nil || i.Decorative {
		return "presentation"
	}
	return "img"
}

// AccessibleName reports the semantic name when the icon is not decorative.
func (i *Icon) AccessibleName() string {
	if i == nil || i.Decorative {
		return ""
	}
	if i.AccessibleLabel != "" {
		return i.AccessibleLabel
	}
	switch src := i.Source.(type) {
	case IconRef:
		if src != "" {
			return string(src)
		}
	case IconSVG:
		if strings.TrimSpace(string(src)) != "" {
			return "icon"
		}
	}
	return "icon"
}

// DiagnosticIcon returns the current icon-specific diagnostic snapshot.
func (i *Icon) DiagnosticIcon() diagnostics.IconSnapshot {
	if i == nil {
		return diagnostics.IconSnapshot{}
	}
	src := ""
	sourceKind := "missing"
	resolved := false
	cacheKey := i.cachedSourceKey
	if i.cachedSource.kind != iconSourceNone {
		resolved = true
		switch i.cachedSource.kind {
		case iconSourceAsset:
			sourceKind = "resolver"
			src = i.cachedSource.asset.SourceRef
			if src == "" && i.Source != nil {
				src = fmt.Sprint(i.Source)
			}
		case iconSourceSVG:
			sourceKind = "inline-svg"
			if i.Source != nil {
				src = fmt.Sprint(i.Source)
			}
		}
	}
	if src == "" && i.Source != nil {
		src = fmt.Sprint(i.Source)
		if sourceKind == "missing" {
			switch i.Source.(type) {
			case IconRef:
				sourceKind = "resolver"
			case IconSVG:
				sourceKind = "inline-svg"
			}
		}
	}
	bounds := i.layoutRole.ArrangedBounds
	if bounds.IsEmpty() {
		bounds = gfx.RectFromXYWH(0, 0, i.cachedSize.W, i.cachedSize.H)
	}
	return diagnostics.IconSnapshot{
		Source:          src,
		SourceKind:      sourceKind,
		Resolved:        resolved,
		Size:            i.cachedSize,
		Bounds:          bounds,
		ColorSlot:       iconColorSlotName(i.ColorSlot),
		DensityBehavior: iconDensityBehaviorName(i.DensityBehavior),
		PreserveAspect:  iconPreserveAspectName(i.PreserveAspectRatio),
		Decorative:      i.Decorative,
		AccessibleName:  i.AccessibleName(),
		CacheKey:        cacheKey,
		CommandCount:    len(i.cachedCommands),
		Missing:         !resolved,
	}
}

// SetSource updates the authored icon source.
func (i *Icon) SetSource(source IconSource) {
	if i == nil || i.Source == source {
		return
	}
	i.Source = source
	i.cachedSource = iconResolvedSource{}
	i.cachedSourceKey = ""
	i.cachedCommands = nil
	i.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetSize updates the authored icon size.
func (i *Icon) SetSize(size float32) {
	if i == nil || i.Size == size {
		return
	}
	i.Size = size
	i.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetColorSlot updates the theme color slot used for projection.
func (i *Icon) SetColorSlot(slot theme.ColorToken) {
	if i == nil || i.ColorSlot == slot {
		return
	}
	i.ColorSlot = slot
	i.invalidate(facet.DirtyProjection)
}

// SetDensityBehavior updates how the icon size responds to density.
func (i *Icon) SetDensityBehavior(behavior IconDensityBehavior) {
	if i == nil || i.DensityBehavior == behavior {
		return
	}
	i.DensityBehavior = behavior
	i.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetAccessibleName updates the accessible name and marks the icon semantic.
func (i *Icon) SetAccessibleName(name string) {
	if i == nil || i.AccessibleLabel == name {
		return
	}
	i.AccessibleLabel = name
	if name != "" {
		i.Decorative = false
	}
	i.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// SetDecorative updates the decorative flag.
func (i *Icon) SetDecorative(decorative bool) {
	if i == nil || i.Decorative == decorative {
		return
	}
	i.Decorative = decorative
	i.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// SetHitPadding updates the extra hit area around the icon bounds.
func (i *Icon) SetHitPadding(padding float32) {
	if i == nil || i.HitPadding == padding {
		return
	}
	i.HitPadding = padding
	i.invalidate(facet.DirtyHit)
}

// SetPreserveAspectRatio updates the fit mode used when projecting the source.
func (i *Icon) SetPreserveAspectRatio(par svgnorm.SVGPreserveAspectRatio) {
	if i == nil || i.PreserveAspectRatio == par {
		return
	}
	i.PreserveAspectRatio = par
	i.invalidate(facet.DirtyProjection)
}

// ExportAnchors publishes the icon anchor set.
func (i *Icon) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if i == nil {
		return nil
	}
	bounds := i.layoutRole.ArrangedBounds
	if bounds.IsEmpty() && !ctx.ResolvedLayer.Bounds.IsEmpty() {
		bounds = ctx.ResolvedLayer.Bounds
	}
	if bounds.IsEmpty() {
		return nil
	}
	return layout.AnchorSet{
		"bounds_center":       gfx.Point{X: (bounds.Min.X + bounds.Max.X) * 0.5, Y: (bounds.Min.Y + bounds.Max.Y) * 0.5},
		"bounds_top_left":     bounds.Min,
		"bounds_top_right":    gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y},
		"bounds_bottom_left":  gfx.Point{X: bounds.Min.X, Y: bounds.Max.Y},
		"bounds_bottom_right": gfx.Point{X: bounds.Max.X, Y: bounds.Max.Y},
	}
}

func (i *Icon) OnAttach(ctx facet.AttachContext) {}
func (i *Icon) OnActivate()                      {}
func (i *Icon) OnDeactivate()                    {}
func (i *Icon) OnDetach() {
	i.cachedSize = gfx.Size{}
	i.cachedSourceKey = ""
	i.cachedSource = iconResolvedSource{}
	i.cachedColor = gfx.Color{}
	i.cachedCommands = nil
	i.cachedTouchPad = 0
}

func (i *Icon) invalidate(flags facet.DirtyFlags) {
	if i == nil {
		return
	}
	i.Facet.Invalidate(flags)
}

func (i *Icon) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	size := i.resolveSize(ctx)
	i.cachedSize = size
	i.cachedTouchPad = i.computeTouchPadding(ctx, size)
	i.layoutRole.MeasuredSize = size
	i.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	_ = i.resolveSource(ctx.Runtime)
	return i.layoutRole.MeasuredResult
}

func (i *Icon) measureSize(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	result := i.measure(ctx, constraints)
	return result.Size
}

func (i *Icon) resolveSize(ctx facet.MeasureContext) gfx.Size {
	resolved := i.resolvedTheme(ctx)
	base := i.Size
	if base <= 0 {
		base = resolved.TokenSet().Spacing.IconSize
	}
	switch i.DensityBehavior {
	case IconDensityLockLogicalSize:
	case IconDensityTouchAware, IconDensityScaleWithDensity, IconDensitySnapToDevicePixels:
		base = resolved.Density.Scale(base)
	}
	switch i.DensityBehavior {
	case IconDensityTouchAware:
		touch := resolved.Density.Scale(resolved.TokenSet().Spacing.TouchTarget)
		if base < touch {
			base = touch
		}
	case IconDensitySnapToDevicePixels:
		scale := ctx.ContentScale
		if scale <= 0 {
			scale = 1
		}
		base = float32(math.Round(float64(base*scale))) / scale
	}
	if base < 0 {
		base = 0
	}
	return gfx.Size{W: base, H: base}
}

func (i *Icon) resolvedTheme(ctx facet.MeasureContext) theme.ResolvedContext {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	return resolved
}

func (i *Icon) styleTokens(runtime any) theme.Tokens {
	if runtime == nil {
		return theme.DefaultTokens()
	}
	type styleContextProvider interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if provider, ok := runtime.(styleContextProvider); ok {
		if store := theme.NearestStyleContext(provider, i.Base().ID()); store != nil {
			return store.Get().Tokens
		}
	}
	return theme.DefaultTokens()
}

func (i *Icon) resolveSource(runtime any) iconResolvedSource {
	if runtime == nil {
		if i.cachedSource.kind != iconSourceNone {
			return i.cachedSource
		}
		key, resolved, ok := i.resolveSourceWithoutRuntime()
		if !ok {
			return iconResolvedSource{}
		}
		if key == i.cachedSourceKey {
			return i.cachedSource
		}
		i.cachedSourceKey = key
		i.cachedSource = resolved
		i.cachedCommands = nil
		return resolved
	}
	key, resolved, ok := i.resolveSourceWithKey(runtime)
	if !ok {
		if i.cachedSource.kind != iconSourceNone {
			return i.cachedSource
		}
		return iconResolvedSource{}
	}
	if key == i.cachedSourceKey {
		return i.cachedSource
	}
	i.cachedSourceKey = key
	i.cachedSource = resolved
	i.cachedCommands = nil
	return resolved
}

func (i *Icon) resolveSourceWithoutRuntime() (string, iconResolvedSource, bool) {
	if i == nil || i.Source == nil {
		return "", iconResolvedSource{}, false
	}
	switch src := i.Source.(type) {
	case IconSVG:
		srcText := strings.TrimSpace(string(src))
		if srcText == "" {
			return "", iconResolvedSource{}, false
		}
		doc, err := svgnorm.ParseSVG([]byte(srcText))
		if err != nil {
			return "", iconResolvedSource{}, false
		}
		for _, el := range doc.Elements {
			if el.ClipPath != nil {
				return "", iconResolvedSource{}, false
			}
		}
		box := doc.ViewBox
		if box.IsEmpty() {
			box = doc.Bounds
		}
		key := fmt.Sprintf("svg:%x", hashString(srcText))
		return key, iconResolvedSource{kind: iconSourceSVG, doc: doc, box: box, key: key}, true
	default:
		return "", iconResolvedSource{}, false
	}
}

func (i *Icon) resolveSourceWithKey(runtime any) (string, iconResolvedSource, bool) {
	if i == nil || i.Source == nil {
		return "", iconResolvedSource{}, false
	}
	switch src := i.Source.(type) {
	case IconRef:
		if strings.TrimSpace(string(src)) == "" {
			return "", iconResolvedSource{}, false
		}
		type resolver interface {
			ResolveIcon(ref string) (runtimepkg.IconAsset, bool)
		}
		if runtime == nil {
			return "", iconResolvedSource{}, false
		}
		var asset runtimepkg.IconAsset
		var ok bool
		if provider, okProvider := runtime.(resolver); okProvider {
			asset, ok = provider.ResolveIcon(string(src))
		}
		if !ok {
			return "", iconResolvedSource{}, false
		}
		asset = asset.Clone()
		box := asset.ViewBox
		if box.IsEmpty() && len(asset.Path.Segments) > 0 {
			box = svgnorm.Bounds(asset.Path)
		}
		key := fmt.Sprintf("ref:%s:%d:%0.4f:%0.4f:%0.4f:%0.4f:%d", asset.SourceRef, asset.Revision, box.Min.X, box.Min.Y, box.Max.X, box.Max.Y, len(asset.Path.Segments))
		return key, iconResolvedSource{kind: iconSourceAsset, asset: asset, box: box, key: key}, true
	case IconAssetPath:
		path := strings.TrimSpace(string(src))
		if path == "" || runtime == nil {
			return "", iconResolvedSource{}, false
		}
		type assetManagerProvider interface {
			AssetManager() assets.Manager
		}
		type assetRegistryProvider interface {
			AssetRegistry() *assets.AssetRegistryStore
		}
		provider, okProvider := runtime.(assetManagerProvider)
		if !okProvider || provider.AssetManager() == nil {
			return "", iconResolvedSource{}, false
		}
		handle := provider.AssetManager().LoadSVG(path)
		if handle.IsZero() {
			return "", iconResolvedSource{}, false
		}
		key := fmt.Sprintf("asset:%s:%d", path, handle.AvailableLOD())
		box := gfx.RectFromXYWH(0, 0, 1, 1)
		if regProvider, okReg := runtime.(assetRegistryProvider); okReg {
			if reg := regProvider.AssetRegistry(); reg != nil {
				if entry := reg.Get(handle.ID); entry != nil {
					key = fmt.Sprintf("asset:%s:%d:%d", path, entry.EntryVersion, handle.AvailableLOD())
					if entry.LODHandles[0] != nil {
						if lod0, ok := entry.LODHandles[0].(*assets.DecodedSVGLOD0); ok && len(lod0.Data) > 0 {
							doc := csg.GetRootAsDocument(lod0.Data, 0)
							if bounds := csgDocumentBounds(doc); !bounds.IsEmpty() {
								box = bounds
							}
						}
					}
				}
			}
		}
		return key, iconResolvedSource{kind: iconSourceManagedAsset, managed: handle, box: box, key: key}, true
	case IconSVG:
		srcText := strings.TrimSpace(string(src))
		if srcText == "" {
			return "", iconResolvedSource{}, false
		}
		doc, err := svgnorm.ParseSVG([]byte(srcText))
		if err != nil {
			i.cachedSource = iconResolvedSource{}
			i.cachedCommands = nil
			return "", iconResolvedSource{}, false
		}
		for _, el := range doc.Elements {
			if el.ClipPath != nil {
				i.cachedSource = iconResolvedSource{}
				i.cachedCommands = nil
				return "", iconResolvedSource{}, false
			}
		}
		box := doc.ViewBox
		if box.IsEmpty() {
			box = doc.Bounds
		}
		key := fmt.Sprintf("svg:%x", hashString(srcText))
		return key, iconResolvedSource{kind: iconSourceSVG, doc: doc, box: box, key: key}, true
	default:
		return "", iconResolvedSource{}, false
	}
}

func (i *Icon) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if i == nil || bounds.IsEmpty() {
		return nil
	}
	resolved := i.resolveSource(runtime)
	if resolved.kind == iconSourceNone || resolved.box.IsEmpty() {
		return nil
	}
	color := i.resolveColor(runtime)
	if len(i.cachedCommands) > 0 && resolved.key == i.cachedSourceKey && bounds == i.layoutRole.ArrangedBounds && color == i.cachedColor {
		return append([]gfx.Command(nil), i.cachedCommands...)
	}
	i.cachedColor = color
	cmds := buildIconCommands(resolved, bounds, color, i.PreserveAspectRatio)
	if len(cmds) == 0 {
		return nil
	}
	i.cachedCommands = append([]gfx.Command(nil), cmds...)
	return cmds
}

func (i *Icon) resolveColor(runtime any) gfx.Color {
	tokens := i.styleTokens(runtime)
	return colorForToken(tokens, i.ColorSlot)
}

func buildIconCommands(src iconResolvedSource, target gfx.Rect, color gfx.Color, par svgnorm.SVGPreserveAspectRatio) []gfx.Command {
	transform := iconFitTransform(src.box, target, par)
	switch src.kind {
	case iconSourceAsset:
		if len(src.asset.Path.Segments) == 0 {
			return nil
		}
		return []gfx.Command{
			gfx.PushTransform{Matrix: transform},
			gfx.FillPath{Path: src.asset.Path, Brush: gfx.SolidBrush(color)},
			gfx.PopTransform{},
		}
	case iconSourceManagedAsset:
		return buildManagedAssetCommands(src.managed, target, color, transform)
	case iconSourceSVG:
		out := make([]gfx.Command, 0, len(src.doc.Elements)*4)
		for _, el := range src.doc.Elements {
			brush := gfx.SolidBrush(color)
			if el.Opacity > 0 && el.Opacity < 1 {
				brush.Color = brush.Color.WithAlpha(brush.Color.A * el.Opacity)
			}
			if el.Fill.Kind != svgnorm.SVGPaintNone {
				out = append(out, gfx.PushTransform{Matrix: transform})
				out = append(out, gfx.FillPath{Path: el.Path, Brush: brush})
				out = append(out, gfx.PopTransform{})
			}
			if el.Stroke != nil && el.Stroke.Width > 0 {
				strokeBrush := brush
				strokeBrush.Color = strokeBrush.Color.WithAlpha(strokeBrush.Color.A * el.Stroke.Paint.Opacity)
				out = append(out, gfx.PushTransform{Matrix: transform})
				out = append(out, gfx.StrokePath{
					Path:   el.Path,
					Stroke: convertStrokeStyle(*el.Stroke),
					Brush:  strokeBrush,
				})
				out = append(out, gfx.PopTransform{})
			}
		}
		return out
	default:
		return nil
	}
}

func buildManagedAssetCommands(handle assets.Handle, target gfx.Rect, color gfx.Color, transform gfx.Transform) []gfx.Command {
	if handle.IsZero() {
		return nil
	}
	reg := handle.Registry()
	if reg == nil {
		return nil
	}
	entry := reg.Get(handle.ID)
	lod := handle.AvailableLOD()
	switch lod {
	case 2:
		if entry != nil {
			if lod2, ok := entry.LODHandles[2].(*assets.DecodedSVGLOD2); ok {
				fill := gfx.ColorFromRGBA8(
					uint8(lod2.DominantColor),
					uint8(lod2.DominantColor>>8),
					uint8(lod2.DominantColor>>16),
					uint8(lod2.DominantColor>>24),
				)
				return []gfx.Command{
					gfx.PushTransform{Matrix: transform},
					gfx.FillRect{Rect: target, Brush: gfx.SolidBrush(fill)},
					gfx.PopTransform{},
				}
			}
		}
	case 1:
		if entry != nil {
			if lod1, ok := entry.LODHandles[1].(*assets.DecodedSVGLOD1); ok {
				img := rgbaFromBytes(lod1.RGBA, 32, 32)
				if img == nil {
					return nil
				}
				return []gfx.Command{
					gfx.PushTransform{Matrix: transform},
					gfx.DrawImage{Image: img, DestRect: target, SrcRect: gfx.RectFromXYWH(0, 0, 32, 32), Sampling: gfx.SamplingBilinear, Opacity: 1},
					gfx.PopTransform{},
				}
			}
		}
	case 0:
		if entry != nil {
			if lod0, ok := entry.LODHandles[0].(*assets.DecodedSVGLOD0); ok {
				return buildCSGCommands(lod0.Data, target, color, transform)
			}
		}
	}
	return nil
}

func rgbaFromBytes(src []byte, width, height int) *image.RGBA {
	if len(src) < width*height*4 {
		return nil
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, src[:width*height*4])
	return img
}

func buildCSGCommands(data []byte, target gfx.Rect, color gfx.Color, transform gfx.Transform) []gfx.Command {
	if len(data) == 0 {
		return nil
	}
	_ = target
	doc := csg.GetRootAsDocument(data, 0)
	out := []gfx.Command{gfx.PushTransform{Matrix: transform}}
	var shape csg.Shape
	for i := 0; i < doc.ShapesLength(); i++ {
		if !doc.Shapes(&shape, i) {
			continue
		}
		path := shapeToGFXPath(&shape)
		if len(path.Segments) == 0 {
			continue
		}
		out = append(out, gfx.FillPath{Path: path, Brush: gfx.SolidBrush(color)})
	}
	out = append(out, gfx.PopTransform{})
	return out
}

func csgDocumentBounds(doc *csg.Document) gfx.Rect {
	if doc == nil {
		return gfx.Rect{}
	}
	var bounds csg.Rect
	if doc.Bounds(&bounds) == nil {
		return gfx.Rect{}
	}
	var min, max csg.Vec2
	bounds.Min(&min)
	bounds.Max(&max)
	return gfx.RectFromXYWH(min.X(), min.Y(), max.X()-min.X(), max.Y()-min.Y())
}

func shapeToGFXPath(shape *csg.Shape) gfx.Path {
	if shape == nil {
		return gfx.Path{}
	}
	var path gfx.Path
	coordIndex := 0
	for i := 0; i < shape.VerbsLength(); i++ {
		verb := shape.Verbs(i)
		switch verb {
		case csg.VerbMoveTo:
			if coordIndex+1 >= shape.CoordsLength() {
				continue
			}
			path.Segments = append(path.Segments, gfx.PathSegment{
				Verb: gfx.PathMoveTo,
				Pts:  [3]gfx.Point{{X: shape.Coords(coordIndex), Y: shape.Coords(coordIndex + 1)}},
			})
			coordIndex += 2
		case csg.VerbLineTo:
			if coordIndex+1 >= shape.CoordsLength() {
				continue
			}
			path.Segments = append(path.Segments, gfx.PathSegment{
				Verb: gfx.PathLineTo,
				Pts:  [3]gfx.Point{{X: shape.Coords(coordIndex), Y: shape.Coords(coordIndex + 1)}},
			})
			coordIndex += 2
		case csg.VerbQuadTo:
			if coordIndex+3 >= shape.CoordsLength() {
				continue
			}
			path.Segments = append(path.Segments, gfx.PathSegment{
				Verb: gfx.PathQuadTo,
				Pts: [3]gfx.Point{
					{X: shape.Coords(coordIndex), Y: shape.Coords(coordIndex + 1)},
					{X: shape.Coords(coordIndex + 2), Y: shape.Coords(coordIndex + 3)},
				},
			})
			coordIndex += 4
		case csg.VerbCubicTo:
			if coordIndex+5 >= shape.CoordsLength() {
				continue
			}
			path.Segments = append(path.Segments, gfx.PathSegment{
				Verb: gfx.PathCubicTo,
				Pts: [3]gfx.Point{
					{X: shape.Coords(coordIndex), Y: shape.Coords(coordIndex + 1)},
					{X: shape.Coords(coordIndex + 2), Y: shape.Coords(coordIndex + 3)},
					{X: shape.Coords(coordIndex + 4), Y: shape.Coords(coordIndex + 5)},
				},
			})
			coordIndex += 6
		case csg.VerbClose:
			path.Segments = append(path.Segments, gfx.PathSegment{Verb: gfx.PathClose})
		}
	}
	return path
}

func convertStrokeStyle(st svgnorm.SVGStroke) gfx.StrokeStyle {
	return gfx.StrokeStyle{
		Width:      st.Width,
		Cap:        convertLineCap(st.Cap),
		Join:       convertLineJoin(st.Join),
		MiterLimit: st.MiterLimit,
		Dash:       append([]float32(nil), st.Dash...),
		DashOffset: st.DashOffset,
	}
}

func convertLineCap(cap gfx.LineCap) gfx.LineCap {
	switch cap {
	case gfx.LineCapRound:
		return gfx.LineCapRound
	case gfx.LineCapSquare:
		return gfx.LineCapSquare
	default:
		return gfx.LineCapButt
	}
}

func convertLineJoin(join gfx.LineJoin) gfx.LineJoin {
	switch join {
	case gfx.LineJoinRound:
		return gfx.LineJoinRound
	case gfx.LineJoinBevel:
		return gfx.LineJoinBevel
	default:
		return gfx.LineJoinMiter
	}
}

func iconFitTransform(srcBox, target gfx.Rect, par svgnorm.SVGPreserveAspectRatio) gfx.Transform {
	if target.IsEmpty() {
		return gfx.Identity()
	}
	if srcBox.IsEmpty() {
		return gfx.Translation(target.Min.X, target.Min.Y)
	}

	meet := true
	align := par.Align
	if align == svgnorm.SVGAspectRatioAlignUnspecified {
		align = svgnorm.SVGAspectRatioAlignXMidYMid
	}
	switch par.MeetOrSlice {
	case svgnorm.SVGMeetOrSliceSlice:
		meet = false
	case svgnorm.SVGMeetOrSliceMeet:
		meet = true
	}
	if align == svgnorm.SVGAspectRatioAlignNone {
		scaleX := target.Width() / srcBox.Width()
		scaleY := target.Height() / srcBox.Height()
		return gfx.Transform{
			A:  scaleX,
			D:  scaleY,
			TX: target.Min.X - srcBox.Min.X*scaleX,
			TY: target.Min.Y - srcBox.Min.Y*scaleY,
		}
	}
	scaleX := target.Width() / srcBox.Width()
	scaleY := target.Height() / srcBox.Height()
	scale := math.Min(float64(scaleX), float64(scaleY))
	if !meet {
		scale = math.Max(float64(scaleX), float64(scaleY))
	}
	scaledW := float32(scale) * srcBox.Width()
	scaledH := float32(scale) * srcBox.Height()
	var offsetX float32
	var offsetY float32
	switch align {
	case svgnorm.SVGAspectRatioAlignXMinYMin:
		offsetX = target.Min.X - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y - srcBox.Min.Y*float32(scale)
	case svgnorm.SVGAspectRatioAlignXMidYMin:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y - srcBox.Min.Y*float32(scale)
	case svgnorm.SVGAspectRatioAlignXMaxYMin:
		offsetX = target.Max.X - scaledW - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y - srcBox.Min.Y*float32(scale)
	case svgnorm.SVGAspectRatioAlignXMinYMid:
		offsetX = target.Min.X - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*float32(scale)
	case svgnorm.SVGAspectRatioAlignXMidYMid:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*float32(scale)
	case svgnorm.SVGAspectRatioAlignXMaxYMid:
		offsetX = target.Max.X - scaledW - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*float32(scale)
	case svgnorm.SVGAspectRatioAlignXMinYMax:
		offsetX = target.Min.X - srcBox.Min.X*float32(scale)
		offsetY = target.Max.Y - scaledH - srcBox.Min.Y*float32(scale)
	case svgnorm.SVGAspectRatioAlignXMidYMax:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*float32(scale)
		offsetY = target.Max.Y - scaledH - srcBox.Min.Y*float32(scale)
	case svgnorm.SVGAspectRatioAlignXMaxYMax:
		offsetX = target.Max.X - scaledW - srcBox.Min.X*float32(scale)
		offsetY = target.Max.Y - scaledH - srcBox.Min.Y*float32(scale)
	default:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*float32(scale)
	}
	return gfx.Transform{
		A:  float32(scale),
		D:  float32(scale),
		TX: offsetX,
		TY: offsetY,
	}
}

func (i *Icon) hitTest(p gfx.Point) facet.HitResult {
	if i == nil || i.Decorative || i.layoutRole.ArrangedBounds.IsEmpty() {
		return facet.HitResult{}
	}
	bounds := i.layoutRole.ArrangedBounds
	padding := i.effectiveHitPadding(bounds)
	if padding > 0 {
		bounds = bounds.Inset(-padding, -padding)
	}
	if !bounds.Contains(p) {
		return facet.HitResult{}
	}
	if i.layoutRole.ArrangedBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: IconMarkIDContent}
	}
	return facet.HitResult{Hit: true, MarkID: IconMarkIDHit}
}

func (i *Icon) effectiveHitPadding(bounds gfx.Rect) float32 {
	padding := i.HitPadding
	if i.DensityBehavior == IconDensityTouchAware {
		if i.cachedTouchPad > padding {
			padding = i.cachedTouchPad
		}
	}
	if padding < 0 {
		padding = 0
	}
	return padding
}

func (i *Icon) computeTouchPadding(ctx facet.MeasureContext, size gfx.Size) float32 {
	if i == nil || i.DensityBehavior != IconDensityTouchAware {
		return 0
	}
	resolved := i.resolvedTheme(ctx)
	touch := resolved.Density.Scale(resolved.TokenSet().Spacing.TouchTarget)
	longest := size.W
	if size.H > longest {
		longest = size.H
	}
	if touch <= longest {
		return 0
	}
	return (touch - longest) * 0.5
}

func hashString(value string) uint64 {
	var h uint64 = 1469598103934665603
	for i := 0; i < len(value); i++ {
		h ^= uint64(value[i])
		h *= 1099511628211
	}
	return h
}

func defaultIconPreserveAspectRatio() svgnorm.SVGPreserveAspectRatio {
	return svgnorm.SVGPreserveAspectRatio{
		Align:       svgnorm.SVGAspectRatioAlignXMidYMid,
		MeetOrSlice: svgnorm.SVGMeetOrSliceMeet,
	}
}

func iconColorSlotName(slot theme.ColorToken) string {
	switch slot {
	case theme.ColorBackground:
		return "background"
	case theme.ColorSurface:
		return "surface"
	case theme.ColorSurfaceVariant:
		return "surface-variant"
	case theme.ColorPrimary:
		return "primary"
	case theme.ColorOnPrimary:
		return "on-primary"
	case theme.ColorText:
		return "text"
	case theme.ColorTextSecondary:
		return "text-secondary"
	case theme.ColorTextDisabled:
		return "text-disabled"
	case theme.ColorBorder:
		return "border"
	case theme.ColorBorderStrong:
		return "border-strong"
	case theme.ColorSelection:
		return "selection"
	case theme.ColorCaret:
		return "caret"
	case theme.ColorError:
		return "error"
	case theme.ColorSuccess:
		return "success"
	case theme.ColorWarning:
		return "warning"
	default:
		return fmt.Sprintf("token(%d)", slot)
	}
}

func iconDensityBehaviorName(b IconDensityBehavior) string {
	switch b {
	case IconDensityScaleWithDensity:
		return "scale-with-density"
	case IconDensityLockLogicalSize:
		return "lock-logical-size"
	case IconDensityTouchAware:
		return "touch-aware"
	case IconDensitySnapToDevicePixels:
		return "snap-to-device-pixels"
	default:
		return fmt.Sprintf("density(%d)", b)
	}
}

func iconPreserveAspectName(par svgnorm.SVGPreserveAspectRatio) string {
	switch par.Align {
	case svgnorm.SVGAspectRatioAlignNone:
		return "none"
	case svgnorm.SVGAspectRatioAlignXMinYMin:
		return "xMinYMin"
	case svgnorm.SVGAspectRatioAlignXMidYMin:
		return "xMidYMin"
	case svgnorm.SVGAspectRatioAlignXMaxYMin:
		return "xMaxYMin"
	case svgnorm.SVGAspectRatioAlignXMinYMid:
		return "xMinYMid"
	case svgnorm.SVGAspectRatioAlignXMidYMid:
		return "xMidYMid"
	case svgnorm.SVGAspectRatioAlignXMaxYMid:
		return "xMaxYMid"
	case svgnorm.SVGAspectRatioAlignXMinYMax:
		return "xMinYMax"
	case svgnorm.SVGAspectRatioAlignXMidYMax:
		return "xMidYMax"
	case svgnorm.SVGAspectRatioAlignXMaxYMax:
		return "xMaxYMax"
	default:
		return "xMidYMid"
	}
}
