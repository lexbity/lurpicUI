package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// LayoutPolicies is the E4 exhibit: a live layout-policy playground. A split
// redistributes as the user toggles a pane's flex/fixed (switch), adjusts the
// fixed panes' width (slider), and adds/removes a fourth pane (button) — the
// same GallerySplit host the shell uses, driven dynamically through SetPanes.
type LayoutPolicies struct {
	facet.Facet
	layout facet.LayoutRole

	split *GallerySplit

	paneA, paneB, paneC, paneD *structure.Card

	// Control stores (the marks write these; E4 recomputes the panes).
	extraVisible *store.ValueStore[bool]
	flexFixed    *store.ValueStore[bool]
	paneMin      *store.ValueStore[float64]

	// Control marks, hosted in a horizontal control card.
	addButton  *action.Button
	minSlider  *selection.Slider
	flexSwitch *selection.Switch
	controls   *structure.Card

	cleanup func()
}

// NewLayoutPolicies builds the E4 exhibit.
func NewLayoutPolicies() *LayoutPolicies {
	e := &LayoutPolicies{
		paneA:        newPaneCard("Pane A"),
		paneB:        newPaneCard("Pane B"),
		paneC:        newPaneCard("Pane C"),
		paneD:        newPaneCard("Pane D"),
		extraVisible: store.NewValueStore(false),
		flexFixed:    store.NewValueStore(true),
		paneMin:      store.NewValueStore(120.0),
	}
	e.buildControls()

	// The split hosts all four pane facets as children; SetPanes (below)
	// chooses which are arranged.
	e.split = NewGallerySplit([]Pane{
		{Facet: e.paneA, FixedWidth: 120, MinWidth: 120},
		{Facet: e.paneB, Flex: 1, MinWidth: 80},
		{Facet: e.paneC, FixedWidth: 120, MinWidth: 120},
		{Facet: e.paneD, FixedWidth: 120, MinWidth: 120},
	}, dividerSize)

	e.Facet = facet.NewFacet()
	e.AddChild(e.split.Base())    //lurpiclint:ignore LL021 -- E4 hosts an action button; its children are not overlays (LL021 over-fires)
	e.AddChild(e.controls.Base()) //lurpiclint:ignore LL021 -- E4 hosts an action button; its children are not overlays (LL021 over-fires)

	e.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke exhibit host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return e.measure(ctx, c)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			e.arrange(ctx, bounds)
		},
	}
	e.layout.Child = linearChildContract(facet.StretchPolicy{
		Width:  facet.StretchAlways,
		Height: facet.StretchAlways,
	})
	e.AddRole(&e.layout)

	e.recomputePanes()
	return e
}

func (e *LayoutPolicies) buildControls() {
	e.addButton = action.NewButton(marks.Const("Toggle D"), marks.Const(uiinput.ButtonFilled))
	e.minSlider = selection.NewSlider("Min width", 40, 260, 10, e.paneMin)
	e.flexSwitch = selection.NewSwitch("B: flex", e.flexFixed)

	e.controls = structure.NewCard("Layout controls")
	e.controls.LayoutMode = marks.Const(structure.CardLayoutHorizontal)
	e.controls.ChildrenContent = []structure.CardChild{
		{Key: "add", Facet: e.addButton, Grid: facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1}},
		{Key: "min", Facet: e.minSlider, Grid: facet.GridPlacement{ColStart: 1, RowStart: 0, ColSpan: 1, RowSpan: 1}},
		{Key: "flex", Facet: e.flexSwitch, Grid: facet.GridPlacement{ColStart: 2, RowStart: 0, ColSpan: 1, RowSpan: 1}},
	}
}

// ExtraVisible reports whether the fourth pane is arranged.
func (e *LayoutPolicies) ExtraVisible() *store.ValueStore[bool] { return e.extraVisible }

// FlexFixed reports whether pane B is flex (true) or fixed (false).
func (e *LayoutPolicies) FlexFixed() *store.ValueStore[bool] { return e.flexFixed }

// PaneMin reports the fixed panes' width control.
func (e *LayoutPolicies) PaneMin() *store.ValueStore[float64] { return e.paneMin }

// Split returns the playground's split host.
func (e *LayoutPolicies) Split() *GallerySplit { return e.split }

// recomputePanes rebuilds the split's arranged pane list from the control
// stores: A and C are fixed at the slider width; B is flex or fixed per the
// switch; D (extra) is fixed when enabled.
func (e *LayoutPolicies) recomputePanes() {
	width := float32(e.paneMin.Get())
	panes := []Pane{
		{Facet: e.paneA, FixedWidth: width, MinWidth: width},
	}
	if e.flexFixed.Get() {
		panes = append(panes, Pane{Facet: e.paneB, Flex: 1, MinWidth: 80})
	} else {
		panes = append(panes, Pane{Facet: e.paneB, FixedWidth: width, MinWidth: width})
	}
	panes = append(panes, Pane{Facet: e.paneC, FixedWidth: width, MinWidth: width})
	if e.extraVisible.Get() {
		panes = append(panes, Pane{Facet: e.paneD, FixedWidth: width, MinWidth: width})
	}
	e.split.SetPanes(panes)
}

func (e *LayoutPolicies) measure(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
	if role := e.split.Base().LayoutRole(); role != nil {
		role.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
	}
	// Measure the controls with an unbounded height so the control card
	// reports its content height instead of flex-filling the whole stage.
	if role := e.controls.Base().LayoutRole(); role != nil {
		role.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: c.MaxSize.W}})
	}
	return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
}

func (e *LayoutPolicies) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	controlsH := e.controls.Base().LayoutRole().MeasuredSize.H
	splitH := bounds.Height() - controlsH
	if splitH < 1 {
		splitH = 1
	}
	e.split.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), splitH))
	e.controls.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y+splitH, bounds.Width(), controlsH))
}

func (e *LayoutPolicies) OnAttach(ctx facet.AttachContext) {
	btnID := e.addButton.Activated.Subscribe(func(signal.Unit) {
		e.extraVisible.Set(!e.extraVisible.Get())
	})
	extraID := e.extraVisible.OnChange.Subscribe(func(signal.Change[bool]) { e.recomputePanes() })
	minID := e.paneMin.OnChange.Subscribe(func(signal.Change[float64]) { e.recomputePanes() })
	flexID := e.flexFixed.OnChange.Subscribe(func(signal.Change[bool]) { e.recomputePanes() })
	e.cleanup = func() {
		e.addButton.Activated.Unsubscribe(btnID)
		e.extraVisible.OnChange.Unsubscribe(extraID)
		e.paneMin.OnChange.Unsubscribe(minID)
		e.flexFixed.OnChange.Unsubscribe(flexID)
	}
}

func (e *LayoutPolicies) OnDetach() {
	if e.cleanup != nil {
		e.cleanup()
		e.cleanup = nil
	}
}

func (e *LayoutPolicies) ID() ExhibitID                         { return ExhibitPolicies }
func (e *LayoutPolicies) Title() string                         { return "Layout Policies" }
func (e *LayoutPolicies) Build(*state.AppState) facet.FacetImpl { return e }

func (e *LayoutPolicies) Base() *facet.Facet { e.BindImpl(e); return &e.Facet }
func (e *LayoutPolicies) OnActivate()        {}
func (e *LayoutPolicies) OnDeactivate()      {}
