package svg

import (
	"fmt"
	"math"
	"sort"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/hashutil"
)

// SVGAnchorSet carries the canonical anchor names exported from a normalized SVG document.
type SVGAnchorSet map[string]gfx.Point

const (
	SVGAnchorBoundsCenter      = "bounds_center"
	SVGAnchorBoundsTopLeft     = "bounds_top_left"
	SVGAnchorBoundsTopRight    = "bounds_top_right"
	SVGAnchorBoundsBottomLeft  = "bounds_bottom_left"
	SVGAnchorBoundsBottomRight = "bounds_bottom_right"
)

// SVGFacet is the raw SVG contract helper for custom marks.
//
// It projects normalized SVG geometry without routing through primitive.icon.
// Callers may use it directly from custom mark implementations.
type SVGFacet struct {
	document            SVGDocument
	currentColor        gfx.Color
	preserveAspectRatio SVGPreserveAspectRatio
	definitions         map[string]SVGDefinition

	cachedKey      uint64
	cachedBounds   gfx.Rect
	cachedCommands []gfx.Command
}

// NewSVGFacet constructs an SVG facet helper for the supplied normalized document.
func NewSVGFacet(doc SVGDocument) *SVGFacet {
	f := &SVGFacet{
		currentColor: gfx.ColorFromRGBA8(0, 0, 0, 255),
	}
	f.SetDocument(doc)
	return f
}

// SetDocument replaces the normalized SVG document and invalidates cached projection state.
func (f *SVGFacet) SetDocument(doc SVGDocument) {
	if f == nil {
		return
	}
	f.document = doc
	f.preserveAspectRatio = doc.PreserveAspectRatio
	f.definitions = make(map[string]SVGDefinition, len(doc.Definitions))
	for _, def := range doc.Definitions {
		f.definitions[def.ID] = def
	}
	f.invalidate()
}

// Document returns the current normalized SVG document snapshot.
func (f *SVGFacet) Document() SVGDocument {
	if f == nil {
		return SVGDocument{}
	}
	return f.document
}

// SetCurrentColor updates the currentColor used by paints that inherit from the caller.
func (f *SVGFacet) SetCurrentColor(color gfx.Color) {
	if f == nil {
		return
	}
	f.currentColor = color
	f.invalidate()
}

// CurrentColor returns the currentColor used by the facet.
func (f *SVGFacet) CurrentColor() gfx.Color {
	if f == nil {
		return gfx.Color{}
	}
	return f.currentColor
}

// SetPreserveAspectRatio updates the fit policy used during projection.
func (f *SVGFacet) SetPreserveAspectRatio(par SVGPreserveAspectRatio) {
	if f == nil {
		return
	}
	f.preserveAspectRatio = par
	f.invalidate()
}

// PreserveAspectRatio returns the current fit policy.
func (f *SVGFacet) PreserveAspectRatio() SVGPreserveAspectRatio {
	if f == nil {
		return SVGPreserveAspectRatio{}
	}
	return f.preserveAspectRatio
}

// SourceBounds returns the SVG document's canonical local-space bounds.
func (f *SVGFacet) SourceBounds() gfx.Rect {
	if f == nil {
		return gfx.Rect{}
	}
	return sourceBounds(f.document)
}

// IntrinsicSize returns the document's unscaled intrinsic size.
func (f *SVGFacet) IntrinsicSize() gfx.Size {
	bounds := f.SourceBounds()
	return gfx.Size{W: bounds.Width(), H: bounds.Height()}
}

// Project resolves the SVG document into command output for the supplied bounds.
func (f *SVGFacet) Project(bounds gfx.Rect) *gfx.CommandList {
	if f == nil || bounds.IsEmpty() {
		return nil
	}
	key := f.cacheKey(bounds)
	if key == f.cachedKey && len(f.cachedCommands) > 0 {
		return &gfx.CommandList{Commands: cloneCommands(f.cachedCommands)}
	}

	srcBounds := f.SourceBounds()
	if srcBounds.IsEmpty() {
		return nil
	}

	transform := fitTransform(srcBounds, bounds, f.preserveAspectRatio)
	scale := transformScale(transform)
	cmds := make([]gfx.Command, 0, len(f.document.Elements)*4)
	for _, el := range f.document.Elements {
		cmds = appendElementCommands(cmds, f.document, f.definitions, el, transform, scale, f.currentColor)
	}
	if len(cmds) == 0 {
		return nil
	}
	f.cachedKey = key
	f.cachedBounds = bounds
	f.cachedCommands = cloneCommands(cmds)
	return &gfx.CommandList{Commands: cmds}
}

// Anchors exports the canonical anchor set for the supplied bounds.
func (f *SVGFacet) Anchors(bounds gfx.Rect) SVGAnchorSet {
	if f == nil || bounds.IsEmpty() {
		return nil
	}
	return SVGAnchorSet{
		SVGAnchorBoundsCenter:      {X: (bounds.Min.X + bounds.Max.X) * 0.5, Y: (bounds.Min.Y + bounds.Max.Y) * 0.5},
		SVGAnchorBoundsTopLeft:     bounds.Min,
		SVGAnchorBoundsTopRight:    {X: bounds.Max.X, Y: bounds.Min.Y},
		SVGAnchorBoundsBottomLeft:  {X: bounds.Min.X, Y: bounds.Max.Y},
		SVGAnchorBoundsBottomRight: {X: bounds.Max.X, Y: bounds.Max.Y},
	}
}

// HitTest reports whether the given point falls within the projected bounds.
func (f *SVGFacet) HitTest(bounds gfx.Rect, p gfx.Point) bool {
	if f == nil || bounds.IsEmpty() {
		return false
	}
	return bounds.Contains(p)
}

func (f *SVGFacet) invalidate() {
	f.cachedKey = 0
	f.cachedBounds = gfx.Rect{}
	f.cachedCommands = nil
}

func (f *SVGFacet) cacheKey(bounds gfx.Rect) uint64 {
	b := hashutil.NewCacheKeyBuilder()
	b.WriteString("gfx/svg.SVGFacet")
	hashSVGDocument(&b, f.document)
	hashRect(&b, bounds)
	hashColor(&b, f.currentColor)
	b.WriteString(f.preserveAspectRatioKey())
	return b.Sum()
}

func (f *SVGFacet) preserveAspectRatioKey() string {
	return fmt.Sprintf("%d:%d", f.preserveAspectRatio.Align, f.preserveAspectRatio.MeetOrSlice)
}

func sourceBounds(doc SVGDocument) gfx.Rect {
	if !doc.ViewBox.IsEmpty() {
		return doc.ViewBox
	}
	if !doc.Bounds.IsEmpty() {
		return doc.Bounds
	}
	return gfx.Rect{}
}

func cloneCommands(cmds []gfx.Command) []gfx.Command {
	if len(cmds) == 0 {
		return nil
	}
	out := make([]gfx.Command, len(cmds))
	copy(out, cmds)
	return out
}

func hashSVGDocument(b *hashutil.CacheKeyBuilder, doc SVGDocument) {
	hashRect(b, doc.ViewBox)
	b.WriteFloat32(doc.Width)
	b.WriteFloat32(doc.Height)
	b.WriteUint8(uint8(doc.PreserveAspectRatio.Align))
	b.WriteUint8(uint8(doc.PreserveAspectRatio.MeetOrSlice))
	b.WriteUint64(uint64(len(doc.Definitions)))
	defs := append([]SVGDefinition(nil), doc.Definitions...)
	sort.Slice(defs, func(i, j int) bool { return defs[i].ID < defs[j].ID })
	for _, def := range defs {
		hashSVGDefinition(b, def)
	}
	b.WriteUint64(uint64(len(doc.Elements)))
	for _, el := range doc.Elements {
		hashSVGElement(b, el)
	}
	hashRect(b, doc.Bounds)
}

func hashSVGDefinition(b *hashutil.CacheKeyBuilder, def SVGDefinition) {
	b.WriteString(def.ID)
	b.WriteUint8(uint8(def.Kind))
	if def.Gradient != nil {
		hashSVGGradient(b, def.Gradient)
	}
	if def.ClipPath != nil {
		hashSVGClipPath(b, def.ClipPath)
	}
}

func hashSVGGradient(b *hashutil.CacheKeyBuilder, grad *SVGGradient) {
	b.WriteString(grad.ID)
	b.WriteUint8(uint8(grad.Units))
	hashTransform(b, grad.Transform)
	hashPoint(b, grad.Start)
	hashPoint(b, grad.End)
	b.WriteUint64(uint64(len(grad.Stops)))
	for _, stop := range grad.Stops {
		b.WriteFloat32(stop.Offset)
		hashColor(b, stop.Color)
	}
}

func hashSVGClipPath(b *hashutil.CacheKeyBuilder, clip *SVGClipPath) {
	b.WriteString(clip.ID)
	b.WriteUint8(uint8(clip.Units))
	hashPath(b, clip.Path)
	hashRect(b, clip.Bounds)
}

func hashSVGElement(b *hashutil.CacheKeyBuilder, el SVGElement) {
	b.WriteString(el.ID)
	hashPath(b, el.Path)
	hashSVGPaint(b, el.Fill)
	hashSVGStroke(b, el.Stroke)
	b.WriteFloat32(el.Opacity)
	b.WriteUint8(uint8(el.FillRule))
	if el.ClipPath != nil {
		hashSVGClipPath(b, el.ClipPath)
	}
	hashRect(b, el.Bounds)
}

func hashSVGPaint(b *hashutil.CacheKeyBuilder, paint SVGPaint) {
	b.WriteUint8(uint8(paint.Kind))
	hashColor(b, paint.Color)
	b.WriteFloat32(paint.Opacity)
	if paint.Gradient != nil {
		hashSVGGradient(b, paint.Gradient)
	}
}

func hashSVGStroke(b *hashutil.CacheKeyBuilder, stroke *SVGStroke) {
	if stroke == nil {
		b.WriteUint8(0)
		return
	}
	b.WriteUint8(1)
	hashSVGPaint(b, stroke.Paint)
	b.WriteFloat32(stroke.Width)
	b.WriteUint8(uint8(stroke.Cap))
	b.WriteUint8(uint8(stroke.Join))
	b.WriteFloat32(stroke.MiterLimit)
	b.WriteUint64(uint64(len(stroke.Dash)))
	for _, dash := range stroke.Dash {
		b.WriteFloat32(dash)
	}
	b.WriteFloat32(stroke.DashOffset)
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

func hashTransform(b *hashutil.CacheKeyBuilder, t gfx.Transform) {
	b.WriteFloat32(t.A)
	b.WriteFloat32(t.B)
	b.WriteFloat32(t.C)
	b.WriteFloat32(t.D)
	b.WriteFloat32(t.TX)
	b.WriteFloat32(t.TY)
}

func hashColor(b *hashutil.CacheKeyBuilder, c gfx.Color) {
	b.WriteFloat32(c.R)
	b.WriteFloat32(c.G)
	b.WriteFloat32(c.B)
	b.WriteFloat32(c.A)
}

func appendElementCommands(dst []gfx.Command, doc SVGDocument, defs map[string]SVGDefinition, el SVGElement, transform gfx.Transform, scale float32, currentColor gfx.Color) []gfx.Command {
	if len(el.Path.Segments) == 0 {
		return dst
	}
	if el.Fill.Kind == SVGPaintNone && (el.Stroke == nil || el.Stroke.Width <= 0) {
		return dst
	}

	dst = append(dst, gfx.PushTransform{Matrix: transform})
	clipRect, hasClip := clipRectForElement(el, transform)
	if hasClip {
		dst = append(dst, gfx.PushClipRect{Rect: clipRect})
	}
	if el.Opacity > 0 && el.Opacity < 1 {
		dst = append(dst, gfx.PushOpacity{Alpha: el.Opacity})
	}

	if fill, ok := brushForPaint(doc, defs, el.Fill, el.Bounds, transform, scale, currentColor); ok {
		dst = append(dst, gfx.FillPath{Path: el.Path, Brush: fill})
	}
	if el.Stroke != nil && el.Stroke.Width > 0 {
		if stroke, ok := brushForPaint(doc, defs, el.Stroke.Paint, el.Bounds, transform, scale, currentColor); ok {
			style := strokeStyleFor(*el.Stroke, scale)
			dst = append(dst, gfx.StrokePath{Path: el.Path, Stroke: style, Brush: stroke})
		}
	}

	if el.Opacity > 0 && el.Opacity < 1 {
		dst = append(dst, gfx.PopOpacity{})
	}
	if hasClip {
		dst = append(dst, gfx.PopClip{})
	}
	dst = append(dst, gfx.PopTransform{})
	return dst
}

func clipRectForElement(el SVGElement, transform gfx.Transform) (gfx.Rect, bool) {
	if el.ClipPath == nil || el.ClipPath.Bounds.IsEmpty() {
		return gfx.Rect{}, false
	}
	return transform.TransformRect(el.ClipPath.Bounds), true
}

func brushForPaint(doc SVGDocument, defs map[string]SVGDefinition, paint SVGPaint, bounds gfx.Rect, transform gfx.Transform, scale float32, currentColor gfx.Color) (gfx.Brush, bool) {
	switch paint.Kind {
	case SVGPaintUnset, SVGPaintNone:
		return gfx.Brush{}, false
	case SVGPaintCurrentColor:
		return gfx.SolidBrush(scaleColor(currentColor, paint.Opacity)), true
	case SVGPaintColor:
		return gfx.SolidBrush(scaleColor(paint.Color, paint.Opacity)), true
	case SVGPaintLinearGradient:
		if paint.Gradient == nil {
			return gfx.Brush{}, false
		}
		def, ok := defs[paint.Gradient.ID]
		if !ok || def.Gradient == nil {
			return gfx.Brush{}, false
		}
		return gradientBrush(def.Gradient, bounds, transform, scale, paint.Opacity), true
	default:
		return gfx.Brush{}, false
	}
}

func gradientBrush(grad *SVGGradient, bounds gfx.Rect, transform gfx.Transform, scale float32, opacity float32) gfx.Brush {
	if grad == nil || len(grad.Stops) == 0 {
		return gfx.Brush{}
	}
	start := grad.Start
	end := grad.End
	if grad.Units == SVGGradientUnitsObjectBoundingBox {
		start = gfx.Point{
			X: bounds.Min.X + bounds.Width()*start.X,
			Y: bounds.Min.Y + bounds.Height()*start.Y,
		}
		end = gfx.Point{
			X: bounds.Min.X + bounds.Width()*end.X,
			Y: bounds.Min.Y + bounds.Height()*end.Y,
		}
	}
	start = grad.Transform.TransformPoint(start)
	end = grad.Transform.TransformPoint(end)
	start = transform.TransformPoint(start)
	end = transform.TransformPoint(end)
	stops := make([]gfx.GradientStop, len(grad.Stops))
	for i, stop := range grad.Stops {
		stops[i] = gfx.GradientStop{
			Offset: stop.Offset,
			Color:  scaleColor(stop.Color, opacity),
		}
	}
	return gfx.LinearGradientBrush(start, end, stops)
}

func strokeStyleFor(st SVGStroke, scale float32) gfx.StrokeStyle {
	out := gfx.StrokeStyle{
		Width:      st.Width * scale,
		Cap:        svgLineCap(st.Cap),
		Join:       svgLineJoin(st.Join),
		MiterLimit: st.MiterLimit,
		DashOffset: st.DashOffset * scale,
	}
	if len(st.Dash) > 0 {
		out.Dash = make([]float32, len(st.Dash))
		for i, dash := range st.Dash {
			out.Dash[i] = dash * scale
		}
	}
	return out
}

func svgLineCap(cap gfx.LineCap) gfx.LineCap {
	switch cap {
	case gfx.LineCapRound:
		return gfx.LineCapRound
	case gfx.LineCapSquare:
		return gfx.LineCapSquare
	default:
		return gfx.LineCapButt
	}
}

func svgLineJoin(join gfx.LineJoin) gfx.LineJoin {
	switch join {
	case gfx.LineJoinRound:
		return gfx.LineJoinRound
	case gfx.LineJoinBevel:
		return gfx.LineJoinBevel
	default:
		return gfx.LineJoinMiter
	}
}

func scaleColor(c gfx.Color, opacity float32) gfx.Color {
	if opacity <= 0 {
		return gfx.Color{}
	}
	if opacity >= 1 {
		return c
	}
	return gfx.Color{
		R: c.R * opacity,
		G: c.G * opacity,
		B: c.B * opacity,
		A: c.A * opacity,
	}
}

func fitTransform(srcBox, target gfx.Rect, par SVGPreserveAspectRatio) gfx.Transform {
	if target.IsEmpty() {
		return gfx.Identity()
	}
	if srcBox.IsEmpty() {
		return gfx.Translation(target.Min.X, target.Min.Y)
	}

	meet := true
	align := par.Align
	if align == SVGAspectRatioAlignUnspecified {
		align = SVGAspectRatioAlignXMidYMid
	}
	switch par.MeetOrSlice {
	case SVGMeetOrSliceSlice:
		meet = false
	case SVGMeetOrSliceMeet:
		meet = true
	}
	scaleX := target.Width() / srcBox.Width()
	scaleY := target.Height() / srcBox.Height()
	if align == SVGAspectRatioAlignNone {
		return gfx.Transform{
			A:  scaleX,
			D:  scaleY,
			TX: target.Min.X - srcBox.Min.X*scaleX,
			TY: target.Min.Y - srcBox.Min.Y*scaleY,
		}
	}
	scale := math.Min(float64(scaleX), float64(scaleY))
	if !meet {
		scale = math.Max(float64(scaleX), float64(scaleY))
	}
	scaledW := float32(scale) * srcBox.Width()
	scaledH := float32(scale) * srcBox.Height()
	var offsetX float32
	var offsetY float32
	switch align {
	case SVGAspectRatioAlignXMinYMin:
		offsetX = target.Min.X - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y - srcBox.Min.Y*float32(scale)
	case SVGAspectRatioAlignXMidYMin:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y - srcBox.Min.Y*float32(scale)
	case SVGAspectRatioAlignXMaxYMin:
		offsetX = target.Max.X - scaledW - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y - srcBox.Min.Y*float32(scale)
	case SVGAspectRatioAlignXMinYMid:
		offsetX = target.Min.X - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*float32(scale)
	case SVGAspectRatioAlignXMidYMid:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*float32(scale)
	case SVGAspectRatioAlignXMaxYMid:
		offsetX = target.Max.X - scaledW - srcBox.Min.X*float32(scale)
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*float32(scale)
	case SVGAspectRatioAlignXMinYMax:
		offsetX = target.Min.X - srcBox.Min.X*float32(scale)
		offsetY = target.Max.Y - scaledH - srcBox.Min.Y*float32(scale)
	case SVGAspectRatioAlignXMidYMax:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*float32(scale)
		offsetY = target.Max.Y - scaledH - srcBox.Min.Y*float32(scale)
	case SVGAspectRatioAlignXMaxYMax:
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

func transformScale(t gfx.Transform) float32 {
	det := t.A*t.D - t.B*t.C
	if det <= 0 {
		return 1
	}
	return float32(math.Sqrt(float64(det)))
}
