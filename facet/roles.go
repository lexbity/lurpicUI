package facet

import (
	"sync/atomic"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout/space"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/text"
)

var nextRenderBatchID atomic.Uint64

// Constraints is the shared layout constraint type.
type Constraints = space.Constraints

// RenderRole participates in command collection inside the resolved layer contract.
type RenderRole struct {
	RenderBatchID gfx.RenderBatchCacheID
	OnCollect     func(list *gfx.CommandList, bounds gfx.Rect)
}

// Collect appends draw commands for the given bounds.
func (r *RenderRole) Collect(bounds gfx.Rect) *gfx.CommandList {
	if r == nil {
		return nil
	}
	var list gfx.CommandList
	if r.OnCollect != nil {
		r.OnCollect(&list, bounds)
	}
	return &list
}

// MarkID identifies a hit-test mark.
type MarkID uint64

// CursorShape describes the cursor requested by a hit result.
type CursorShape uint8

const (
	CursorDefault CursorShape = iota
	CursorPointer
	CursorText
	CursorCrosshair
	CursorGrab
	CursorGrabbing
	CursorResize
	CursorNotAllowed
)

// HitResult is returned from hit tests.
type HitResult struct {
	Hit    bool
	MarkID MarkID
	Cursor CursorShape
}

// HitRole participates in hit testing inside the resolved layer contract.
type HitRole struct {
	OnHitTest func(p gfx.Point) HitResult
}

// HitTest runs the hit-test callback.
func (r *HitRole) HitTest(p gfx.Point) HitResult {
	if r == nil || r.OnHitTest == nil {
		return HitResult{}
	}
	return r.OnHitTest(p)
}

// PointerEvent is delivered to facets during pointer routing.
type PointerEvent struct {
	Kind      platform.PointerEventKind
	Position  gfx.Point
	ScreenPos gfx.Point
	Button    platform.PointerButton
	Modifiers platform.ModifierKeys
	MarkID    MarkID
}

// TouchEvent is delivered to facets during touch routing.
type TouchEvent struct {
	SequenceID  uint64
	Phase       platform.TouchPhase
	Position    gfx.Point
	ScreenPos   gfx.Point
	StartPos    gfx.Point
	ScreenStart gfx.Point
	Pressure    float32
	MarkID      MarkID
}

// ScrollEvent is delivered to facets during scroll routing.
type ScrollEvent struct {
	Position  gfx.Point
	DeltaX    float32
	DeltaY    float32
	Precise   bool
	Modifiers platform.ModifierKeys
}

// KeyEvent is delivered to facets during key routing.
type KeyEvent struct {
	Kind      platform.KeyEventKind
	Key       platform.Key
	Modifiers platform.ModifierKeys
}

// TextEvent is delivered to facets during text input routing.
type TextEvent struct {
	Text      string
	Composing bool
}

// DismissEvent is delivered when a layer is dismissed by an outside interaction.
type DismissEvent struct {
	Trigger    DismissalTrigger
	ScreenPos  gfx.Point
	HitFacetID FacetID
	HitMarkID  MarkID
	HitLayerID LayerID
	HitOrder   int
}

// InputRole participates in direct input handling inside the resolved layer contract.
type InputRole struct {
	OnPointer func(e PointerEvent) bool
	OnTouch   func(e TouchEvent) bool
	OnScroll  func(e ScrollEvent) bool
	OnKey     func(e KeyEvent) bool
	OnText    func(e TextEvent) bool
	OnDismiss func(e DismissEvent) bool

	// SuppressSyntheticPointer opts out of touch-to-pointer fallback for this facet.
	SuppressSyntheticPointer bool
}

// FocusRole participates in keyboard focus management inside the resolved layer contract.
type FocusRole struct {
	Focusable     func() bool
	OnFocusGained func()
	OnFocusLost   func()
	TabIndex      int
}

// ViewportRole defines an authored local transform inside the resolved layer contract.
type ViewportRole struct {
	Transform   gfx.Transform
	WorldBounds gfx.Rect
}

// ProjectionLayer carries the resolved spatial context for projection.
type ProjectionLayer struct {
	LayerID       LayerID
	Bounds        gfx.Rect
	Transform     gfx.Transform
	ClipRect      gfx.Rect
	CoordSpace    uint8
	RenderOrder   int
	HitPolicy     uint8
	ClipPolicy    ClipPolicy
	Dismissal     DismissalScope
	FocusTrap     bool
	FocusRestore  FocusRestoreMode
	RecipeVersion uint64
}

// LayerToLocal converts a point in layer space back into authored local space.
func (v *ViewportRole) LayerToLocal(layerPt gfx.Point) (localPt gfx.Point, ok bool) {
	if v == nil {
		return gfx.Point{}, false
	}
	inv, ok := v.Transform.Inverse()
	if !ok {
		return gfx.Point{}, false
	}
	return inv.TransformPoint(layerPt), true
}

// LocalToLayer converts an authored local point into layer space.
func (v *ViewportRole) LocalToLayer(localPt gfx.Point) gfx.Point {
	if v == nil {
		return gfx.Point{}
	}
	return v.Transform.TransformPoint(localPt)
}

// SetPanZoom updates the transform from pan and zoom.
func (v *ViewportRole) SetPanZoom(pan gfx.Point, zoom float32) {
	if v == nil {
		return
	}
	v.Transform = gfx.Transform{
		A:  zoom,
		D:  zoom,
		TX: pan.X,
		TY: pan.Y,
	}
}

// ProjectionContext provides the minimal inputs for projection within a resolved layer.
// Layer is authoritative when populated; helper methods fall back to the local
// bounds and viewport transform only when projection is invoked without a
// resolved runtime snapshot.
type ProjectionContext struct {
	Bounds        gfx.Rect
	Viewport      *ViewportRole
	Runtime       RuntimeServices
	Layer         ProjectionLayer
	ContentScale  float32
	InputModality InputModality
}

// ProjectionRole consumes the resolved layer contract and participates in projection output collection.
type ProjectionRole struct {
	OnProject   func(ctx ProjectionContext) *gfx.CommandList
	OnJobResult func(result job.AnyResult)
}

// Project collects commands for the supplied projection context.
func (r *ProjectionRole) Project(ctx ProjectionContext) *gfx.CommandList {
	if r == nil || r.OnProject == nil {
		return nil
	}
	return r.OnProject(ctx)
}

// TextSelectionGeometry is the selection geometry produced by TextRole.
type TextSelectionGeometry struct {
	CaretRect      gfx.Rect
	SelectionRects []gfx.Rect
	CaretVisible   bool
}

// TextRole participates in selection geometry collection.
type TextRole struct {
	Layout        *text.TextLayout
	Selection     text.TextRange
	CaretPosition text.TextPosition
	CaretVisible  bool
	IMEEnabled    bool
}

// CollectSelectionGeometry computes selection and caret geometry from the current text layout.
func (r *TextRole) CollectSelectionGeometry() *TextSelectionGeometry {
	if r == nil || r.Layout == nil {
		return nil
	}
	out := &TextSelectionGeometry{
		CaretVisible: r.CaretVisible,
	}
	if sel := r.Selection.Normalized(); !sel.IsEmpty() {
		rects := r.Layout.SelectionRects(sel)
		if len(rects) > 0 {
			out.SelectionRects = make([]gfx.Rect, 0, len(rects))
			for _, rect := range rects {
				out.SelectionRects = append(out.SelectionRects, gfx.Rect{
					Min: gfx.Point{X: rect.Min.X, Y: rect.Min.Y},
					Max: gfx.Point{X: rect.Max.X, Y: rect.Max.Y},
				})
			}
		}
	}
	if r.CaretVisible {
		rect := r.Layout.CaretRect(r.CaretPosition)
		out.CaretRect = gfx.Rect{
			Min: gfx.Point{X: rect.Min.X, Y: rect.Min.Y},
			Max: gfx.Point{X: rect.Max.X, Y: rect.Max.Y},
		}
	}
	return out
}

// TickRole receives per-frame updates.
type TickRole struct {
	OnTick func(dt time.Duration)
	active bool
}

// RequestTick keeps the role active for the next frame.
func (r *TickRole) RequestTick() {
	if r == nil {
		return
	}
	r.active = true
}

// IsActive reports whether the role requested another tick.
func (r *TickRole) IsActive() bool {
	return r != nil && r.active
}

// Reset clears the active flag. The runtime calls this once per frame.
func (r *TickRole) Reset() {
	if r == nil {
		return
	}
	r.active = false
}

func (r *RenderRole) onAttach(f *Facet) {
	if r.RenderBatchID == 0 {
		r.RenderBatchID = gfx.RenderBatchCacheID(nextRenderBatchID.Add(1))
	}
}
func (r *RenderRole) onActivate(f *Facet)   {}
func (r *RenderRole) onDeactivate(f *Facet) {}
func (r *RenderRole) onDispose(f *Facet) {
	r.OnCollect = nil
}

func (r *HitRole) onAttach(f *Facet) {
	if r.OnHitTest == nil {
		panic("facet: HitRole requires OnHitTest")
	}
}
func (r *HitRole) onActivate(f *Facet)   {}
func (r *HitRole) onDeactivate(f *Facet) {}
func (r *HitRole) onDispose(f *Facet) {
	r.OnHitTest = nil
}

func (r *InputRole) onAttach(f *Facet)     {}
func (r *InputRole) onActivate(f *Facet)   {}
func (r *InputRole) onDeactivate(f *Facet) {}
func (r *InputRole) onDispose(f *Facet) {
	r.OnPointer = nil
	r.OnScroll = nil
	r.OnKey = nil
	r.OnText = nil
	r.OnDismiss = nil
}

func (r *FocusRole) onAttach(f *Facet)     {}
func (r *FocusRole) onActivate(f *Facet)   {}
func (r *FocusRole) onDeactivate(f *Facet) {}
func (r *FocusRole) onDispose(f *Facet) {
	r.Focusable = nil
	r.OnFocusGained = nil
	r.OnFocusLost = nil
}

func (r *ViewportRole) onAttach(f *Facet)     {}
func (r *ViewportRole) onActivate(f *Facet)   {}
func (r *ViewportRole) onDeactivate(f *Facet) {}
func (r *ViewportRole) onDispose(f *Facet)    {}

func (r *ProjectionRole) onAttach(f *Facet) {
	if r.OnProject == nil {
		panic("facet: ProjectionRole requires OnProject")
	}
}
func (r *ProjectionRole) onActivate(f *Facet)   {}
func (r *ProjectionRole) onDeactivate(f *Facet) {}
func (r *ProjectionRole) onDispose(f *Facet) {
	r.OnProject = nil
	r.OnJobResult = nil
}

func (r *TextRole) onAttach(f *Facet)     {}
func (r *TextRole) onActivate(f *Facet)   {}
func (r *TextRole) onDeactivate(f *Facet) {}
func (r *TextRole) onDispose(f *Facet) {
	r.Layout = nil
}

func (r *TickRole) onAttach(f *Facet)     {}
func (r *TickRole) onActivate(f *Facet)   {}
func (r *TickRole) onDeactivate(f *Facet) {}
func (r *TickRole) onDispose(f *Facet) {
	r.OnTick = nil
	r.active = false
}
