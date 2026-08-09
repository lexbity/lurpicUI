package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// AnchorsData is the E3 exhibit descriptor.
type AnchorsData struct {
	fonts *text.FontRegistry
	theme theme.ResolvedContext
	ids   studioLayerIDs
}

// NewAnchorsData builds the E3 descriptor.
func NewAnchorsData(fonts *text.FontRegistry, themeCtx theme.ResolvedContext, ids studioLayerIDs) *AnchorsData {
	return &AnchorsData{fonts: fonts, theme: themeCtx, ids: ids}
}

func (p *AnchorsData) ID() ExhibitID { return ExhibitAnchors }
func (p *AnchorsData) Title() string { return "Anchored Overlays" }
func (p *AnchorsData) Build(_ *state.AppState) facet.FacetImpl {
	return NewAnchorsFacet(p.fonts, p.theme, p.ids)
}

// e3Trigger is the draggable anchor trigger: a circle that exports its bounds
// anchors (FR-anchor) and drags by updating a position store.
type e3Trigger struct {
	marks.Core

	pos       *store.ValueStore[gfx.Point]
	fill      gfx.Color
	dragStart *gfx.Point
	// host is the owning Anchors exhibit. The runtime's free layer re-positions
	// this trigger from its layer attachment even when the stage has hidden the
	// exhibit (arranged to empty bounds), so render and hit are gated on the
	// host's arranged bounds (F-inactive-layer-child): a hidden exhibit's
	// trigger must neither draw nor claim pointer hits.
	host *Anchors
}

func newE3Trigger(themeCtx theme.ResolvedContext, fonts *text.FontRegistry, pos *store.ValueStore[gfx.Point], host *Anchors) *e3Trigger {
	t := &e3Trigger{pos: pos, fill: gfx.ColorFromRGBA8(200, 80, 60, 255), host: host}
	t.Facet = facet.NewFacet()
	t.Layout.Child = facet.GroupChildContract{
		// Attached to the studio.trigger free layer (draggable position);
		// declared outside the base grid so its placement is unambiguous.
		SupportedPlacement: facet.SupportsFree | facet.SupportsAnchor,
	}
	t.Layout.OnMeasure = func(_ facet.MeasureContext, cst facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: cst.Constrain(gfx.Size{W: 56, H: 56})}
	}
	t.Layout.OnArrange = func(_ facet.ArrangeContext, bounds gfx.Rect) { t.Layout.ArrangedBounds = bounds }
	t.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		b := t.Layout.ArrangedBounds
		if b.IsEmpty() || t.hostHidden() {
			return nil
		}
		center := gfx.Point{X: b.Min.X + b.Width()*0.5, Y: b.Min.Y + b.Height()*0.5}
		return []gfx.Command{
			gfx.FillPath{Path: gfx.CirclePath(center, b.Width()*0.5), Brush: gfx.SolidBrush(t.fill)},
		}
	}
	t.Hit.OnHitTest = func(pt gfx.Point) facet.HitResult {
		if t.hostHidden() {
			return facet.HitResult{}
		}
		if !t.Layout.ArrangedBounds.IsEmpty() && t.Layout.ArrangedBounds.Contains(pt) {
			return facet.HitResult{Hit: true, Cursor: facet.CursorGrab}
		}
		return facet.HitResult{}
	}
	t.Input.OnPointer = func(e facet.PointerEvent) bool {
		switch e.Kind {
		case platform.PointerPress:
			if e.Button == platform.PointerLeft {
				t.dragStart = &gfx.Point{X: e.Position.X, Y: e.Position.Y}
				t.Invalidate(facet.DirtyProjection)
			}
		case platform.PointerMove:
			if t.dragStart != nil {
				p := t.pos.Get()
				dx := e.Position.X - t.dragStart.X
				dy := e.Position.Y - t.dragStart.Y
				t.pos.Set(gfx.Point{X: p.X + dx, Y: p.Y + dy})
				t.dragStart = &gfx.Point{X: e.Position.X, Y: e.Position.Y}
			}
		case platform.PointerRelease:
			t.dragStart = nil
		}
		return true
	}
	t.RegisterRoles()
	return t
}

// ExportAnchors publishes the standard bounds anchors plus a bottom-center
// anchor that the popovers reference with AnchorSide=Below.
func (t *e3Trigger) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	out := t.DefaultAnchors(t.Layout.ArrangedBounds, ctx)
	if out == nil {
		out = layout.AnchorSet{}
	}
	b := t.Layout.ArrangedBounds
	out["bounds_bottom_center"] = gfx.Point{X: (b.Min.X + b.Max.X) * 0.5, Y: b.Max.Y}
	return out
}

// hostHidden reports whether the owning exhibit is currently hidden by the
// stage (arranged to empty bounds). The runtime's free layer still positions
// this trigger from its layer attachment when hidden, so render and hit must be
// gated here rather than on the trigger's own bounds (F-inactive-layer-child).
func (t *e3Trigger) hostHidden() bool {
	if t == nil || t.host == nil {
		return false
	}
	role := t.host.Base().LayoutRole()
	return role == nil || role.ArrangedBounds.IsEmpty()
}

func (t *e3Trigger) Base() *facet.Facet             { t.BindImpl(t); return &t.Facet }
func (t *e3Trigger) OnAttach(_ facet.AttachContext) {}
func (t *e3Trigger) OnDetach()                      {}
func (t *e3Trigger) OnActivate()                    {}
func (t *e3Trigger) OnDeactivate()                  {}

// Anchors is the E3 facet: a draggable trigger whose exported anchors drive
// anchored popovers on the studio.anchored layer; moving the trigger re-
// resolves the anchor layer each frame so the popovers follow.
type Anchors struct {
	facet.Facet
	layout facet.LayoutRole

	pos      *store.ValueStore[gfx.Point]
	trigger  *e3Trigger
	popovers []*overlayBox //lurpiclint:ignore LL012 -- handles kept for anchor-tracking assertions in e3_anchors_test.go

	rt  facet.RuntimeServices
	ids studioLayerIDs
	cln func()
}

// NewAnchorsFacet builds the E3 facet.
func NewAnchorsFacet(fonts *text.FontRegistry, themeCtx theme.ResolvedContext, ids studioLayerIDs) *Anchors {
	e := &Anchors{
		pos: store.NewValueStore(gfx.Point{X: 240, Y: 160}),
		ids: ids,
	}
	e.Facet = facet.NewFacet()
	e.trigger = newE3Trigger(themeCtx, fonts, e.pos, e)
	e.popovers = []*overlayBox{
		newOverlayBox(gfx.ColorFromRGBA8(230, 130, 60, 255), "menu", themeCtx, fonts, gfx.Size{W: 120, H: 48}),
		newOverlayBox(gfx.ColorFromRGBA8(130, 90, 200, 255), "split", themeCtx, fonts, gfx.Size{W: 120, H: 48}),
		newOverlayBox(gfx.ColorFromRGBA8(255, 220, 90, 255), "tooltip", themeCtx, fonts, gfx.Size{W: 120, H: 48}),
	}

	e.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke exhibit host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			e.arrange(ctx, bounds)
		},
	}
	e.AddRole(&e.layout)
	return e
}

// Trigger returns the draggable trigger.
func (e *Anchors) Trigger() *e3Trigger { return e.trigger }

// Popovers returns the anchored popover boxes.
func (e *Anchors) Popovers() []*overlayBox { return append([]*overlayBox(nil), e.popovers...) }

func (e *Anchors) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	// Honor an empty/zero parent bounds: when the stage hides this exhibit
	// (inactive exhibits are arranged to gfx.Rect{}), the trigger must not
	// fall back to window coordinates — bounds.Min=(0,0) with the seed pos
	// would place it inside the active exhibit's area.
	if bounds.IsEmpty() {
		e.trigger.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
		return
	}
	pos := e.pos.Get()
	size := gfx.Size{W: 56, H: 56}
	// Prime the trigger's arranged bounds from the current position inside the
	// layout pass so the anchor cache resolves from this frame's position
	// (resolveLayerTree reads it right after the pass). The studio.trigger
	// free layer re-positions identically from the attachment placement.
	e.trigger.Base().LayoutRole().Arrange(facet.ArrangeContext{
		Runtime:     ctx.Runtime,
		Theme:       ctx.Theme,
		ParentGroup: e.trigger.Layout.Parent,
		ChildGroup:  e.trigger.Layout.Child,
		Placement: facet.Placement{
			Mode: facet.PlacementFree,
			Free: facet.FreePlacement{
				X: facet.ResolvedScalar(pos.X),
				Y: facet.ResolvedScalar(pos.Y),
			},
		},
	}, gfx.RectFromXYWH(bounds.Min.X+pos.X, bounds.Min.Y+pos.Y, size.W, size.H))
}

func freeTriggerAttachment(ids studioLayerIDs, pos gfx.Point) facet.Attachment {
	return facet.Attachment{
		LayerID: ids.trigger,
		Placement: facet.Placement{
			Mode: facet.PlacementFree,
			Free: facet.FreePlacement{
				X: facet.ResolvedScalar(pos.X),
				Y: facet.ResolvedScalar(pos.Y),
			},
		},
	}
}

func (e *Anchors) OnAttach(ctx facet.AttachContext) {
	e.rt = ctx.Runtime
	if rt, ok := e.rt.(*runtime.Runtime); ok && e.ids.trigger != 0 {
		// The trigger exports its anchors from the studio.trigger free layer;
		// the popovers sit on the anchored layer referencing the trigger's
		// bottom-center anchor (FR-anchor).
		rt.AddFacet(e, e.trigger, freeTriggerAttachment(e.ids, e.pos.Get()))
		for _, p := range e.popovers {
			rt.AddFacet(e, p, facet.Attachment{
				LayerID: e.ids.anchored,
				Placement: facet.Placement{
					Mode: facet.PlacementAnchor,
					Anchor: facet.AnchorPlacement{
						AnchorRef: facet.AnchorID("bounds_bottom_center"),
						Side:      facet.AnchorBelow,
						Gap:       8,
					},
				},
			})
		}
	}
	idPos := e.pos.OnChange.Subscribe(func(signal.Change[gfx.Point]) {
		// Moving the trigger re-positions the free layer attachment AND
		// re-arranges it; the anchor layer policy re-positions the popovers
		// from the re-exported anchors.
		if rt, ok := e.rt.(*runtime.Runtime); ok {
			rt.UpdateChildAttachment(e.trigger, freeTriggerAttachment(e.ids, e.pos.Get()))
			invalidateLayout(e, e.rt, "e3.trigger.drag")
		}
	})
	e.cln = func() {
		e.pos.OnChange.Unsubscribe(idPos)
	}
}

func (e *Anchors) OnDetach() {
	if e.cln != nil {
		e.cln()
		e.cln = nil
	}
}

func (e *Anchors) Base() *facet.Facet { e.BindImpl(e); return &e.Facet }
func (e *Anchors) OnActivate()        {}
func (e *Anchors) OnDeactivate()      {}

var _ layout.AnchorExporter = (*e3Trigger)(nil)
