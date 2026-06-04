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
	"codeburg.org/lexbit/lurpicui/marks"
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

func (IconRef) isIconSource()  {}
func (IconSVG) isIconSource()  {}

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
	marks.Core

	Source              IconSource
	Size                marks.Binding[float32]
	ColorSlot           marks.Binding[theme.ColorToken]
	DensityBehavior     marks.Binding[IconDensityBehavior]
	AccessibleLabel     marks.Binding[string]
	Decorative          marks.Binding[bool]
	HitPadding          marks.Binding[float32]
	PreserveAspectRatio marks.Binding[svgnorm.SVGPreserveAspectRatio]

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
	kind     iconResolvedSourceKind
	asset    runtimepkg.IconAsset
	managed  assets.Handle
	drawable gfx.DrawableRef
	doc      svgnorm.SVGDocument
	box      gfx.Rect
	key      string
}

var _ facet.FacetImpl = (*Icon)(nil)
var _ layout.AnchorExporter = (*Icon)(nil)
var _ marks.Mark = (*Icon)(nil)

// NewIcon constructs a primitive.icon mark with canonical defaults.
func NewIcon(source IconSource) *Icon {
	i := &Icon{
		Source:              source,
		Size:                marks.Const[float32](0),
		ColorSlot:           marks.Const(theme.ColorText),
		DensityBehavior:     marks.Const(IconDensityScaleWithDensity),
		AccessibleLabel:     marks.Const(""),
		Decorative:          marks.Const(true),
		HitPadding:          marks.Const[float32](0),
		PreserveAspectRatio: marks.Const(defaultIconPreserveAspectRatio()),
	}
	i.Core.Facet = facet.NewFacet()
	i.AddBinding(i.Size)
	i.AddBinding(i.ColorSlot)
	i.AddBinding(i.DensityBehavior)
	i.AddBinding(i.AccessibleLabel)
	i.AddBinding(i.Decorative)
	i.AddBinding(i.HitPadding)
	i.AddBinding(i.PreserveAspectRatio)

	i.Layout.Parent = facet.GroupParentContract{Kind: facet.GroupLayoutNone}
	i.Layout.Child = facet.GroupChildContract{
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
	i.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return i.measure(ctx, constraints)
	}
	i.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		i.Layout.ArrangedBounds = bounds
	}
	i.Hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		return i.hitTest(p)
	}
	i.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return i.buildCommands(i.Layout.ArrangedBounds, ctx.Runtime)
	}
	i.RegisterRoles()
	return i
}

// Base satisfies facet.FacetImpl.
func (i *Icon) Base() *facet.Facet {
	i.Facet.BindImpl(i)
	return &i.Facet
}

// Descriptor satisfies marks.Mark.
func (i *Icon) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "primitive", TypeName: "icon"}
}

// AccessibilityRole reports the semantic role required by the mark.
func (i *Icon) AccessibilityRole() string {
	if i.Decorative.Get() {
		return "presentation"
	}
	return "img"
}

// AccessibleName reports the semantic name when the icon is not decorative.
func (i *Icon) AccessibleName() string {
	if i.Decorative.Get() {
		return ""
	}
	if label := i.AccessibleLabel.Get(); label != "" {
		return label
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
	bounds := i.Layout.ArrangedBounds
	if bounds.IsEmpty() {
		bounds = gfx.RectFromXYWH(0, 0, i.cachedSize.W, i.cachedSize.H)
	}
	return diagnostics.IconSnapshot{
		Source:          src,
		SourceKind:      sourceKind,
		Resolved:        resolved,
		Size:            i.cachedSize,
		Bounds:          bounds,
		ColorSlot:       iconColorSlotName(i.ColorSlot.Get()),
		DensityBehavior: iconDensityBehaviorName(i.DensityBehavior.Get()),
		PreserveAspect:  iconPreserveAspectName(i.PreserveAspectRatio.Get()),
		Decorative:      i.Decorative.Get(),
		AccessibleName:  i.AccessibleName(),
		CacheKey:        cacheKey,
		CommandCount:    len(i.cachedCommands),
		Missing:         !resolved,
	}
}

// ExportAnchors publishes the icon anchor set.
func (i *Icon) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	bounds := i.Layout.ArrangedBounds
	if bounds.IsEmpty() && !ctx.ResolvedLayer.Bounds.IsEmpty() {
		bounds = ctx.ResolvedLayer.Bounds
	}
	if bounds.IsEmpty() {
		return nil
	}
	return i.Core.DefaultAnchors(bounds, ctx)
}

func (i *Icon) OnAttach(ctx facet.AttachContext) { i.Core.OnAttach() }
func (i *Icon) OnDetach() {
	i.Core.OnDetach()
	i.cachedSize = gfx.Size{}
	i.cachedSourceKey = ""
	i.cachedSource = iconResolvedSource{}
	i.cachedColor = gfx.Color{}
	i.cachedCommands = nil
	i.cachedTouchPad = 0
}
func (i *Icon) OnActivate()   { i.Core.OnActivate() }
func (i *Icon) OnDeactivate() { i.Core.OnDeactivate() }

func (i *Icon) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	size := i.resolveSize(ctx)
	i.cachedSize = size
	i.cachedTouchPad = i.computeTouchPadding(ctx, size)
	i.Layout.MeasuredSize = size
	i.Layout.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	_ = i.resolveSource(ctx.Runtime)
	return i.Layout.MeasuredResult
}

func (i *Icon) measureSize(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	result := i.measure(ctx, constraints)
	return result.Size
}

func (i *Icon) resolveSize(ctx facet.MeasureContext) gfx.Size {
	resolved := i.resolvedTheme(ctx)
	base := i.Size.Get()
	if base <= 0 {
		base = resolved.TokenSet().Spacing.IconSize
	}
	switch i.DensityBehavior.Get() {
	case IconDensityLockLogicalSize:
	case IconDensityTouchAware, IconDensityScaleWithDensity, IconDensitySnapToDevicePixels:
		base = resolved.Density.Scale(base)
	}
	switch i.DensityBehavior.Get() {
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
	if i.Source == nil {
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
	if i.Source == nil {
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

		var ref gfx.DrawableRef
		refOK := false
		if rt, ok := runtime.(assets.Runtime); ok {
			ref, refOK = assets.ResolveDrawable(rt, handle, assets.AssetTypeSVG)
		}

		box := gfx.RectFromXYWH(0, 0, 1, 1)
		key := ""
		if regProvider, okReg := runtime.(assetRegistryProvider); okReg {
			if reg := regProvider.AssetRegistry(); reg != nil {
				if entry := reg.Get(handle.ID); entry != nil {
					if refOK {
						key = ref.Key
					} else {
						lod := handle.AvailableLOD()
						key = fmt.Sprintf("asset:%s:%d:%d", handle.ID.String(), entry.EntryVersion, lod)
					}
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
		if key == "" {
			return "", iconResolvedSource{}, false
		}
		return key, iconResolvedSource{kind: iconSourceManagedAsset, managed: handle, drawable: ref, box: box, key: key}, true
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
	if bounds.IsEmpty() {
		return nil
	}
	resolved := i.resolveSource(runtime)
	if resolved.kind == iconSourceNone || resolved.box.IsEmpty() {
		return nil
	}
	color := i.resolveColor(runtime)
	if len(i.cachedCommands) > 0 && resolved.key == i.cachedSourceKey && bounds == i.Layout.ArrangedBounds && color == i.cachedColor {
		return append([]gfx.Command(nil), i.cachedCommands...)
	}
	i.cachedColor = color
	cmds := buildIconCommands(resolved, bounds, color, i.PreserveAspectRatio.Get())
	if len(cmds) == 0 {
		return nil
	}
	i.cachedCommands = append([]gfx.Command(nil), cmds...)
	return cmds
}

func (i *Icon) resolveColor(runtime any) gfx.Color {
	tokens := i.styleTokens(runtime)
	return colorForToken(tokens, i.ColorSlot.Get())
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
		return buildManagedAssetCommands(src.drawable, src.managed, target, color, transform)
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

func buildManagedAssetCommands(ref gfx.DrawableRef, handle assets.Handle, target gfx.Rect, color gfx.Color, transform gfx.Transform) []gfx.Command {
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
		if ref.Kind == gfx.DrawableVector && ref.Vector != nil {
			if lod0, ok := ref.Vector.(*assets.DecodedSVGLOD0); ok {
				return buildCSGCommands(lod0.Data, target, color, transform)
			}
		}
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
	if i.Decorative.Get() || i.Layout.ArrangedBounds.IsEmpty() {
		return facet.HitResult{}
	}
	bounds := i.Layout.ArrangedBounds
	padding := i.effectiveHitPadding(bounds)
	if padding > 0 {
		bounds = bounds.Inset(-padding, -padding)
	}
	if !bounds.Contains(p) {
		return facet.HitResult{}
	}
	if i.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: IconMarkIDContent}
	}
	return facet.HitResult{Hit: true, MarkID: IconMarkIDHit}
}

func (i *Icon) effectiveHitPadding(bounds gfx.Rect) float32 {
	padding := i.HitPadding.Get()
	if i.DensityBehavior.Get() == IconDensityTouchAware {
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
	if i.DensityBehavior.Get() != IconDensityTouchAware {
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
