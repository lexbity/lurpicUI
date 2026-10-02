package feedback

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Scrim is a hit-blocking dim overlay (RX-2 FR-5c): it fills its arranged
// bounds with a themed dim color, blocks hits to content beneath it, and
// emits Dismissed when the user presses on it (tap-outside dismissal) or when
// a permitted facet.DismissalScope event reaches it. An empty arrangement
// renders and hits nothing, so the owner gates visibility by arrangement (the
// RX-1 hosting contract) or by mounting it as a layer child.
//
// The dim color resolves from the theme's OnSurface token at 40% alpha unless
// overridden through the Color binding.
type Scrim struct {
	marks.Core

	// Color overrides the dim color. The zero color resolves to the theme's
	// OnSurface token at 0.4 alpha.
	Color marks.Binding[gfx.Color]

	// Dismissed fires when a press lands on the scrim or a permitted
	// dismissal event reaches it. The owner closes the overlays it owns.
	Dismissed signal.Signal[signal.Unit]

	// dismissal scopes the facet.DismissEvent input path (the layer
	// system's dismissal delivery).
	dismissal facet.DismissalScope

	cachedColor gfx.Color
}

var _ facet.FacetImpl = (*Scrim)(nil)
var _ marks.Mark = (*Scrim)(nil)

// NewScrim constructs a feedback.scrim mark with canonical defaults.
func NewScrim() *Scrim {
	s := &Scrim{
		Color:     marks.Const(gfx.Color{}),
		Dismissed: signal.NewSignal[signal.Unit]("scrim_dismissed"),
	}
	s.Facet = facet.NewFacet()
	s.AddBinding(s.Color)

	// The scrim is a leaf that fills whatever its host or layer arranges.
	s.Layout.Parent = facet.GroupParentContract{Kind: facet.GroupLayoutNone, Overflow: facet.OverflowClip}
	s.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsLinear | facet.SupportsAnchor | facet.SupportsFree,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := s.measure(ctx, constraints).Size
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionClip,
			BelowMinHeight: facet.CompressionClip,
			AboveMaxWidth:  facet.ExpansionClip,
			AboveMaxHeight: facet.ExpansionClip,
		},
		Stretch: facet.StretchPolicy{
			Width:  facet.StretchWhenParentRequests,
			Height: facet.StretchWhenParentRequests,
		},
		Baseline: facet.BaselineNone,
	}
	s.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return s.measure(ctx, constraints)
	}
	s.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		s.Layout.ArrangedBounds = bounds
	}
	s.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return s.buildCommands(s.Layout.ArrangedBounds)
	}
	s.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return s.hitTest(p) }
	s.Input.OnPointer = func(e facet.PointerEvent) bool { return s.onPointer(e) }
	s.Input.OnDismiss = func(e facet.DismissEvent) bool { return s.onDismiss(e) }
	s.RegisterRoles()
	return s
}

// SetDismissal scopes the facet.DismissEvent input path: when the layer
// system delivers a permitted dismissal trigger, the scrim emits Dismissed.
func (s *Scrim) SetDismissal(scope facet.DismissalScope) {
	if s == nil {
		return
	}
	s.dismissal = scope
}

// Base satisfies facet.FacetImpl.
func (s *Scrim) Base() *facet.Facet {
	s.BindImpl(s)
	return &s.Facet
}

// Descriptor satisfies marks.Mark.
func (s *Scrim) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: markTypeFeedback, TypeName: "scrim"}
}

// Children is a facet.ChildSource conformance stub; the scrim is a leaf.
//
// nolint:LL031
func (s *Scrim) Children() []facet.GroupChild { return nil }

// AccessibilityRole reports the semantic role.
func (s *Scrim) AccessibilityRole() string { return "dialog" }

// measure fills the available space (the scrim always covers its host area)
// and caches the themed dim color for projection.
func (s *Scrim) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	if resolved, ok := ctx.Theme.(theme.ResolvedContext); ok {
		c := resolved.Color(theme.ColorText)
		c.A = 0.4
		s.cachedColor = c
	}
	s.Layout.MeasuredSize = constraints.Constrain(constraints.MaxSize)
	s.Layout.MeasuredResult = facet.MeasureResult{
		Size:        s.Layout.MeasuredSize,
		Intrinsic:   facet.IntrinsicSize{Min: s.Layout.MeasuredSize, Preferred: s.Layout.MeasuredSize, Max: s.Layout.MeasuredSize},
		Constraints: constraints,
	}
	return s.Layout.MeasuredResult
}

func (s *Scrim) OnAttach(ctx facet.AttachContext) { s.Core.OnAttach(ctx) }
func (s *Scrim) OnActivate()                      { s.Core.OnActivate() }
func (s *Scrim) OnDeactivate()                    { s.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (s *Scrim) OnDetach() {
	s.Core.OnDetach()
	s.cachedColor = gfx.Color{}
}

// buildCommands dims the arranged bounds. Empty bounds contribute nothing
// (the projection gate also prunes them).
func (s *Scrim) buildCommands(bounds gfx.Rect) []gfx.Command {
	if s == nil || bounds.IsEmpty() {
		return nil
	}
	color := s.resolveColor()
	if color.A <= 0 {
		return nil
	}
	return []gfx.Command{gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(color)}}
}

// resolveColor resolves the dim color: the binding when set, otherwise the
// theme's OnSurface token at 0.4 alpha (cached from the last measure).
func (s *Scrim) resolveColor() gfx.Color {
	if c := s.Color.Get(); c != (gfx.Color{}) {
		return c
	}
	if s.cachedColor != (gfx.Color{}) {
		return s.cachedColor
	}
	resolved := theme.DefaultResolvedContext()
	c := resolved.Color(theme.ColorText)
	c.A = 0.4
	return c
}

func (s *Scrim) hitTest(p gfx.Point) facet.HitResult {
	if s == nil || s.Layout.ArrangedBounds.IsEmpty() || !s.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	return facet.HitResult{Hit: true}
}

func (s *Scrim) onPointer(e facet.PointerEvent) bool {
	if s == nil || e.Button != platform.PointerLeft {
		return false
	}
	if !s.Layout.ArrangedBounds.Contains(e.Position) {
		return false
	}
	if e.Kind == platform.PointerPress {
		// A press on the scrim is "outside" the overlays it fronts: dismiss.
		s.Dismissed.Emit(signal.Unit{})
		return true
	}
	// Consume hover/release so content beneath never sees it.
	return true
}

func (s *Scrim) onDismiss(e facet.DismissEvent) bool {
	if s == nil || !s.dismissal.Enabled {
		return false
	}
	if s.dismissal.Triggers&(1<<uint(e.Trigger)) == 0 {
		return false
	}
	s.Dismissed.Emit(signal.Unit{})
	return true
}
