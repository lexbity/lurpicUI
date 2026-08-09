package studio

import (
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// LayersData is the E2 exhibit descriptor.
type LayersData struct {
	fonts *text.FontRegistry
	theme theme.ResolvedContext
	ids   studioLayerIDs
}

// NewLayersData builds the E2 descriptor.
func NewLayersData(fonts *text.FontRegistry, themeCtx theme.ResolvedContext, ids studioLayerIDs) *LayersData {
	return &LayersData{fonts: fonts, theme: themeCtx, ids: ids}
}

func (p *LayersData) ID() ExhibitID { return ExhibitLayers }
func (p *LayersData) Title() string { return "Layers & Hit Routing" }
func (p *LayersData) Build(_ *state.AppState) facet.FacetImpl {
	return NewLayersFacet(p.fonts, p.theme, p.ids)
}

// e2Control is the base-layer control the modal scrim covers (FR-layers): a
// colored box that counts presses so the blocking/pass-through is observable.
type e2Control struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	hit    facet.HitRole
	input  facet.InputRole

	presses int
	color   gfx.Color
	label   string
	shaper  *text.Shaper
	style   text.TextStyle
}

func newE2Control(themeCtx theme.ResolvedContext, fonts *text.FontRegistry, label string) *e2Control {
	c := &e2Control{color: gfx.ColorFromRGBA8(60, 120, 220, 255), label: label}
	c.Facet = facet.NewFacet()
	if fonts != nil {
		c.shaper = text.NewShaper(fonts)
	}
	c.style = themeCtx.TextStyle(theme.TextLabelM)
	c.layout = facet.LayoutRole{ //lurpiclint:ignore LL001 -- bespoke exhibition leaf mark: a fixed-size base-layer control; no built-in layout container matches
		OnMeasure: func(_ facet.MeasureContext, cst facet.Constraints) facet.MeasureResult {
			return facet.MeasureResult{Size: cst.Constrain(gfx.Size{W: 160, H: 96})}
		},
		OnArrange: func(_ facet.ArrangeContext, bounds gfx.Rect) { c.layout.ArrangedBounds = bounds },
	}
	c.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			if bounds.IsEmpty() {
				return
			}
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(c.color)})
			if cmd := glyphLabel(bounds.Min.X+bounds.Width()*0.5, bounds.Min.Y+8, c.label, c.shaper, c.style, gfx.ColorFromRGBA8(255, 255, 255, 255)); cmd != nil {
				list.Commands = append(list.Commands, cmd)
			}
		},
	}
	c.hit = facet.HitRole{
		OnHitTest: func(pt gfx.Point) facet.HitResult {
			if !c.layout.ArrangedBounds.IsEmpty() && c.layout.ArrangedBounds.Contains(pt) {
				return facet.HitResult{Hit: true}
			}
			return facet.HitResult{}
		},
	}
	c.input = facet.InputRole{
		OnPointer: func(e facet.PointerEvent) bool {
			if e.Kind == platform.PointerPress && e.Button == platform.PointerLeft {
				c.presses++
				c.Invalidate(facet.DirtyProjection)
				return true
			}
			return true
		},
	}
	c.AddRole(&c.layout)
	c.AddRole(&c.render)
	c.AddRole(&c.hit)
	c.AddRole(&c.input)
	return c
}

func (c *e2Control) Base() *facet.Facet             { c.BindImpl(c); return &c.Facet }
func (c *e2Control) OnAttach(_ facet.AttachContext) {}
func (c *e2Control) OnDetach()                      {}
func (c *e2Control) OnActivate()                    {}
func (c *e2Control) OnDeactivate()                  {}

// overlayBox is a layered overlay (the modal scrim, the tooltip, anchored
// popovers): a colored box that fills its layer so the layer hit policy
// applies over its whole area. Its visibility gates both rendering and
// hit-testing (an invisible scrim does not block). It declares the V2 overlay
// contract: a resolved layer, a hit role, and a layer-dismissal scope.
type overlayBox struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	hit    facet.HitRole
	input  facet.InputRole

	// layer and dismissal are the overlay contract's layer/dismissal halves:
	// layer records the resolved layer (set during arrangement), dismissal
	// scopes the OnDismiss input to the owning exhibit's visibility store.
	layer     facet.LayerContext
	dismissal facet.DismissalScope
	onDismiss func()

	fill    gfx.Color
	label   string
	visible bool
	size    gfx.Size // zero = fill the layer
	shaper  *text.Shaper
	style   text.TextStyle
	textCl  gfx.Color
}

func newOverlayBox(fill gfx.Color, label string, themeCtx theme.ResolvedContext, fonts *text.FontRegistry, size ...gfx.Size) *overlayBox {
	o := &overlayBox{fill: fill, label: label}
	if len(size) > 0 {
		o.size = size[0]
	}
	o.Facet = facet.NewFacet()
	if fonts != nil {
		o.shaper = text.NewShaper(fonts)
	}
	o.style = themeCtx.TextStyle(theme.TextLabelS)
	o.textCl = themeCtx.Color(theme.ColorText)
	o.layout = facet.LayoutRole{ //lurpiclint:ignore LL001 -- bespoke share leaf mark: a fill/layer box sized by its layer policy
		Child: facet.GroupChildContract{
			SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
			Stretch: facet.StretchPolicy{
				Width:  facet.StretchAlways,
				Height: facet.StretchAlways,
			},
		},
		OnMeasure: func(_ facet.MeasureContext, cst facet.Constraints) facet.MeasureResult {
			if o.size.W > 0 || o.size.H > 0 {
				return facet.MeasureResult{Size: cst.Constrain(o.size)}
			}
			return facet.MeasureResult{Size: cst.MaxSize}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			o.layout.ArrangedBounds = bounds
			o.layer = ctx.Layer
		},
	}
	o.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			if bounds.IsEmpty() || !o.visible {
				return
			}
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(o.fill)})
			if cmd := glyphLabel(bounds.Min.X+8, bounds.Min.Y+8, o.label, o.shaper, o.style, o.textCl); cmd != nil {
				list.Commands = append(list.Commands, cmd)
			}
		},
	}
	o.hit = facet.HitRole{
		OnHitTest: func(pt gfx.Point) facet.HitResult {
			if o.visible && o.layer.ID != 0 && !o.layout.ArrangedBounds.IsEmpty() && o.layout.ArrangedBounds.Contains(pt) {
				return facet.HitResult{Hit: true}
			}
			return facet.HitResult{}
		},
	}
	o.input = facet.InputRole{
		OnDismiss: func(e facet.DismissEvent) bool {
			if o == nil || !o.visible || !o.dismissal.Enabled {
				return false
			}
			if o.dismissal.Triggers&(1<<uint(e.Trigger)) == 0 {
				return false
			}
			if o.onDismiss != nil {
				o.onDismiss()
			}
			return true
		},
	}
	o.AddRole(&o.layout)
	o.AddRole(&o.render)
	o.AddRole(&o.hit)
	o.AddRole(&o.input)
	return o
}

// SetDismissal configures the overlay's V2 layer-dismissal contract: the scope
// selects the dismissal triggers and onDismiss fires when a permitted dismissal
// event reaches this overlay.
func (o *overlayBox) SetDismissal(scope facet.DismissalScope, onDismiss func()) {
	if o == nil {
		return
	}
	o.dismissal = scope
	o.onDismiss = onDismiss
}

func (o *overlayBox) Base() *facet.Facet             { o.BindImpl(o); return &o.Facet }
func (o *overlayBox) OnAttach(_ facet.AttachContext) {}
func (o *overlayBox) OnDetach()                      {}
func (o *overlayBox) OnActivate()                    {}
func (o *overlayBox) OnDeactivate()                  {}

// SetVisible toggles the overlay's rendering and hit-testing.
func (o *overlayBox) SetVisible(visible bool) {
	if o == nil {
		return
	}
	o.visible = visible
	o.Invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// IsVisible reports whether the overlay is rendering/hittable.
func (o *overlayBox) IsVisible() bool { return o != nil && o.visible }

// Layers is the E2 facet: a base-layer control beneath a modal scrim
// (HitBlockBelow) that blocks it, a tooltip (HitPassThrough) that lets the
// drag through, and an auto-dismissing toast that never steals input.
type Layers struct {
	facet.Facet
	layout facet.LayoutRole
	tick   facet.TickRole

	control  *e2Control
	scrim    *overlayBox
	tooltip  *overlayBox
	toast    *overlayBox
	controls *structure.Card

	modalOpen  *store.ValueStore[bool]
	tooltipOn  *store.ValueStore[bool]
	toastOn    *store.ValueStore[bool]
	toastUntil time.Time

	rt  facet.RuntimeServices
	ids studioLayerIDs
	cln func()
}

// NewLayersFacet builds the E2 facet.
func NewLayersFacet(fonts *text.FontRegistry, themeCtx theme.ResolvedContext, ids studioLayerIDs) *Layers {
	e := &Layers{
		modalOpen: store.NewValueStore(false),
		tooltipOn: store.NewValueStore(true),
		toastOn:   store.NewValueStore(false),
		ids:       ids,
	}
	e.Facet = facet.NewFacet()

	e.control = newE2Control(themeCtx, fonts, "covered control")
	e.scrim = newOverlayBox(gfx.ColorFromRGBA8(0, 0, 0, 120), "modal (HitBlockBelow)", themeCtx, fonts)
	e.tooltip = newOverlayBox(gfx.ColorFromRGBA8(255, 220, 90, 60), "tooltip (HitPassThrough)", themeCtx, fonts)
	e.toast = newOverlayBox(gfx.ColorFromRGBA8(90, 180, 120, 255), "toast", themeCtx, fonts)
	e.buildControls()

	// The layered overlays declare their layer contract at construction via
	// facet.AttachLayer (V2 overlay mounting); attachLayers later pins them to
	// the studio custom layers by LayerID. The content-plane control and card
	// stay regular children: the toast is a content notice the host arranges
	// into the corner, so it never intercepts the layer input policies.
	facet.AttachLayer(e, e.scrim, facet.LayerAttachment{
		ZPriority: 100,
		Dismissal: facet.DismissalScope{
			Enabled:  true,
			Triggers: facet.DismissalTriggerSetPointer | facet.DismissalTriggerSetKey,
		},
	})
	facet.AttachLayer(e, e.tooltip, facet.LayerAttachment{ZPriority: 50})
	e.scrim.SetDismissal(facet.DismissalScope{
		Enabled:  true,
		Triggers: facet.DismissalTriggerSetPointer | facet.DismissalTriggerSetKey,
	}, func() { e.modalOpen.Set(false) })
	e.tooltip.SetDismissal(facet.DismissalScope{
		Enabled:  true,
		Triggers: facet.DismissalTriggerSetPointer,
	}, func() { e.tooltipOn.Set(false) })
	e.AddChild(e.control.Base())
	e.AddChild(e.toast.Base())
	e.AddChild(e.controls.Base())

	e.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke exhibit host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			if role := e.controls.Base().LayoutRole(); role != nil {
				role.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: c.MaxSize.W}})
			}
			return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			e.arrange(ctx, bounds)
		},
	}
	e.tick = facet.TickRole{OnTick: func(dt time.Duration) {
		if e.toastOn.Get() && time.Now().After(e.toastUntil) {
			e.toastOn.Set(false)
		}
	}}
	e.AddRole(&e.layout)
	e.AddRole(&e.tick)
	return e
}

// Control returns the covered base control (for the FR-layers tests).
func (e *Layers) Control() *e2Control { return e.control }

// ModalOpen returns the modal open/close store.
func (e *Layers) ModalOpen() *store.ValueStore[bool] { return e.modalOpen }

// TooltipOn returns the tooltip visibility store.
func (e *Layers) TooltipOn() *store.ValueStore[bool] { return e.tooltipOn }

// ToastOn returns the toast visibility store.
func (e *Layers) ToastOn() *store.ValueStore[bool] { return e.toastOn }

func (e *Layers) buildControls() {
	modalSwitch := selection.NewSwitch("Modal", e.modalOpen)
	tooltipSwitch := selection.NewSwitch("Tooltip", e.tooltipOn)
	toastButton := action.NewButton(marks.Const("Show toast"), marks.Const(uiinput.ButtonFilled))
	toastButton.Activated.Subscribe(func(signal.Unit) {
		e.toastOn.Set(true)
		e.toastUntil = time.Now().Add(2 * time.Second)
	})

	e.controls = structure.NewCard("Layers")
	e.controls.GridColumns = marks.Const(3)
	e.controls.GridRows = marks.Const(1)
	e.controls.ChildrenContent = []structure.CardChild{
		{Key: "modal", Facet: modalSwitch, Grid: facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1}},
		{Key: "tooltip", Facet: tooltipSwitch, Grid: facet.GridPlacement{ColStart: 1, RowStart: 0, ColSpan: 1, RowSpan: 1}},
		{Key: "toast", Facet: toastButton, Grid: facet.GridPlacement{ColStart: 2, RowStart: 0, ColSpan: 1, RowSpan: 1}},
	}
}

func (e *Layers) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	// Honor an empty/zero parent bounds: when the stage hides this exhibit
	// (inactive exhibits arranged to gfx.Rect{}), the controls must not fall
	// back to window-relative coordinates — bounds.Min=(0,0) with the
	// content-centering math places the covered control at (-80,-48,160,96),
	// whose bottom-right extends inside the window and steals clicks from the
	// chrome's title bar (F-inactive-layer-child, same class as Anchors).
	if bounds.IsEmpty() {
		e.control.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
		e.toast.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
		e.controls.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
		e.scrim.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
		e.tooltip.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
		return
	}
	controlsH := e.controls.Base().LayoutRole().MeasuredSize.H
	if controlsH < 1 {
		controlsH = 1
	}
	content := gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), bounds.Height()-controlsH)
	if content.IsEmpty() {
		content = bounds
	}

	// The covered control sits in the middle of the content area.
	ctrlSize := gfx.Size{W: 160, H: 96}
	e.control.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(
		content.Min.X+(content.Width()-ctrlSize.W)*0.5,
		content.Min.Y+(content.Height()-ctrlSize.H)*0.5,
		ctrlSize.W, ctrlSize.H))

	// The toast slides in at the bottom-right corner; it has no hit role, so
	// it never steals input.
	if e.toastOn.Get() {
		tSize := gfx.Size{W: 140, H: 44}
		e.toast.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(
			content.Max.X-tSize.W-12, content.Max.Y-tSize.H-12,
			tSize.W, tSize.H))
	} else {
		e.toast.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
	}

	e.controls.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, content.Max.Y, bounds.Width(), controlsH))

	// The scrim and tooltip fill their layers (the layer arrangement owns
	// their bounds), so only their visibility flags need invalidation here.
	e.scrim.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
	e.tooltip.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
}

// syncOverlayLayers mounts or unmounts the overlay facets on their layers so
// the overlay contract stays in sync with the visibility flags. Hit regions
// are bounds-derived, so an invisible overlay left on a block-below layer
// would still cover and block the base layer (a closed modal would swallow
// the scrim's hit); unmounting (LayerID 0) lets the layer arrangement skip it
// and the host reset its bounds, keeping the layer hit policies (HitBlockBelow
// / HitPassThrough) in force only while the overlay is on screen.
func (e *Layers) syncOverlayLayers(rt *runtime.Runtime) {
	if rt == nil {
		return
	}
	// Visible overlays span the full layer grid so their hit regions cover the
	// content area and the layer hit policy applies everywhere.
	full := facet.Placement{
		Mode: facet.PlacementGrid,
		Grid: facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 5, RowSpan: 5},
	}
	mount := func(box *overlayBox, lid facet.LayerID) {
		if box == nil || !box.IsVisible() || lid == 0 {
			rt.UpdateChildAttachment(box, facet.Attachment{})
			return
		}
		rt.UpdateChildAttachment(box, facet.Attachment{LayerID: lid, Placement: full})
	}
	mount(e.scrim, e.ids.modal)
	mount(e.tooltip, e.ids.tooltip)
}

func (e *Layers) OnAttach(ctx facet.AttachContext) {
	e.rt = ctx.Runtime
	rt, _ := e.rt.(*runtime.Runtime)
	e.scrim.SetVisible(e.modalOpen.Get())
	e.tooltip.SetVisible(e.tooltipOn.Get())
	e.syncOverlayLayers(rt)
	sync := func() {
		e.syncOverlayLayers(rt)
		e.Invalidate(facet.DirtyProjection)
	}
	idModal := e.modalOpen.OnChange.Subscribe(func(signal.Change[bool]) {
		e.scrim.SetVisible(e.modalOpen.Get())
		sync()
	})
	idTooltip := e.tooltipOn.OnChange.Subscribe(func(signal.Change[bool]) {
		e.tooltip.SetVisible(e.tooltipOn.Get())
		sync()
	})
	idToast := e.toastOn.OnChange.Subscribe(func(signal.Change[bool]) {
		e.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	})
	e.cln = func() {
		e.modalOpen.OnChange.Unsubscribe(idModal)
		e.tooltipOn.OnChange.Unsubscribe(idTooltip)
		e.toastOn.OnChange.Unsubscribe(idToast)
	}
}

func (e *Layers) OnDetach() {
	if e.cln != nil {
		e.cln()
		e.cln = nil
	}
}

func (e *Layers) Base() *facet.Facet { e.BindImpl(e); return &e.Facet }
func (e *Layers) OnActivate()        {}
func (e *Layers) OnDeactivate()      {}
