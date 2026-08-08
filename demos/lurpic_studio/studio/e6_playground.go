package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// Playground is the E6 exhibit: the action/input/selection/navigation/
// feedback/status families in an interactive gallery. A navigation.tabs host
// switches between per-family playgrounds (the tabs mark's genuine home,
// F-tabs); each family tab is a scrollable list of cards, each card hosting one
// mark with a control that exercises a distinctive behavior of that mark
// (FR-playground, FR-coverage-distinct).
//
// The family bodies are hosted by the demo's bespoke scroll list (F-scroll-content:
// scroll_region draws its content without attaching it to the facet tree, so
// interactive content cannot live in one); the active body is arranged into the
// tabs' panel and the inactive bodies to zero bounds, exactly like the Stage
// gates its exhibits.
//
// The family builders live in the sibling files e6_action.go, e6_selection.go,
// e6_input.go, e6_navigation.go, e6_feedback.go, matching the P9 per-family
// file plan (F-exhibits-pkg: these live in the studio package, not a
// studios/exhibits subpackage, to avoid an import cycle with the stage).
type Playground struct {
	facet.Facet
	layout facet.LayoutRole

	tabs      *navigation.Tabs
	activeTab *store.ValueStore[int]

	actionFam *playActionFamily
	selectFam *playSelectFamily
	inputFam  *playInputFamily
	navFam    *playNavFamily
	feedback  *playFeedbackFamily
	statusFam *playStatusFamily

	bodies   []facet.FacetImpl //lurpiclint:ignore LL012 -- the family body facets are composition structure, not domain state (F-lint-hosts)
	cleanups []func()          //lurpiclint:ignore LL012 -- subscription cleanup handles are structural lifecycle state (F-lint-hosts)
}

// listGap is the vertical gap between playground cards.
const listGap = 8

// NewPlayground builds the E6 exhibit.
func NewPlayground(state *state.AppState) *Playground {
	e := &Playground{
		activeTab: store.NewValueStore(0),
		actionFam: newPlayActionFamily(),
		selectFam: newPlaySelectFamily(),
		inputFam:  newPlayInputFamily(),
		navFam:    newPlayNavFamily(),
		feedback:  newPlayFeedbackFamily(),
		statusFam: newPlayStatusFamily(),
	}

	e.bodies = []facet.FacetImpl{
		e.actionFam.scroll,
		e.selectFam.scroll,
		e.inputFam.scroll,
		e.navFam.scroll,
		e.feedback.scroll,
		e.statusFam.scroll,
	}
	e.tabs = navigation.NewTabs("Capability playground", []navigation.TabItem{
		{Key: "action", Label: "Action", Body: e.bodies[0]},
		{Key: "selection", Label: "Selection", Body: e.bodies[1]},
		{Key: "input", Label: "Input", Body: e.bodies[2]},
		{Key: "navigation", Label: "Navigation", Body: e.bodies[3]},
		{Key: "feedback", Label: "Feedback", Body: e.bodies[4]},
		{Key: "status", Label: "Status", Body: e.bodies[5]},
	}, e.activeTab)

	e.Facet = facet.NewFacet()
	e.AddChild(e.tabs.Base()) //lurpiclint:ignore LL021 -- E6 hosts a navigational tabs mark as its regular child, not an overlay (LL021 over-fires on field refs)
	// The family bodies are real facet-tree children so their cards are
	// projected and hit-tested by the runtime. The tabs mark arranges the
	// active body into its panel; the inactive bodies are arranged to zero
	// bounds below (the Stage gating idiom).
	for _, body := range e.bodies {
		if body != nil && body.Base() != nil {
			e.AddChild(body.Base()) //lurpiclint:ignore LL021 -- E6 hosts playground cards as regular children, not overlays (LL021 over-fires)
		}
	}

	e.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke exhibit host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			if role := e.tabs.Base().LayoutRole(); role != nil {
				role.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
			}
			return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
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
	return e
}

// arrange delegates to the tabs (which draws the strip and arranges the active
// body into its panel), then zeroes the inactive family bodies so only the
// active playground projects and hit-tests.
func (e *Playground) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if role := e.tabs.Base().LayoutRole(); role != nil {
		role.Arrange(ctx, bounds)
	}
	active := e.activeTab.Get()
	for i, body := range e.bodies {
		if body == nil || body.Base() == nil || body.Base().LayoutRole() == nil {
			continue
		}
		if i == active {
			continue
		}
		body.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
	}
}

func (e *Playground) OnAttach(ctx facet.AttachContext) {
	for _, wire := range []func() func(){
		e.actionFam.wire,
		e.selectFam.wire,
		e.inputFam.wire,
		e.navFam.wire,
		e.feedback.wire,
		e.statusFam.wire,
	} {
		if cleanup := wire(); cleanup != nil {
			e.cleanups = append(e.cleanups, cleanup)
		}
	}
	// Tab switching changes which family body is active, so it must re-lay the
	// host. The tabs mark invalidates its own local facet bits on ActiveIndex
	// change, but the runtime layout pass is gated on rt.dirtyFacets, which only
	// RuntimeServices.Invalidate populates (F-dirtylayout-routing). Routing the
	// store signal through the runtime re-measures and re-arranges the newly
	// active family body; the tabs mark then measures/arranges that body from
	// its panel bounds.
	tabID := e.activeTab.OnChange.Subscribe(func(signal.Change[int]) {
		invalidateLayout(e, ctx.Runtime, "playground.activeTab")
	})
	e.cleanups = append(e.cleanups, func() { e.activeTab.OnChange.Unsubscribe(tabID) })
}

// ActiveTab returns the tabs' active-index store.
func (e *Playground) ActiveTab() *store.ValueStore[int] { return e.activeTab }

// Tabs returns the family-switching tabs host.
func (e *Playground) Tabs() *navigation.Tabs { return e.tabs }

// Action returns the action family handles.
func (e *Playground) Action() *playActionFamily { return e.actionFam }

// Selection returns the selection family handles.
func (e *Playground) Selection() *playSelectFamily { return e.selectFam }

// Input returns the input family handles.
func (e *Playground) Input() *playInputFamily { return e.inputFam }

// Navigation returns the navigation family handles.
func (e *Playground) Navigation() *playNavFamily { return e.navFam }

// Feedback returns the feedback family handles.
func (e *Playground) Feedback() *playFeedbackFamily { return e.feedback }

// Status returns the status family handles.
func (e *Playground) Status() *playStatusFamily { return e.statusFam }

func (e *Playground) Base() *facet.Facet { e.BindImpl(e); return &e.Facet }
func (e *Playground) OnActivate()        {}
func (e *Playground) OnDeactivate()      {}

func (e *Playground) OnDetach() {
	for _, c := range e.cleanups {
		if c != nil {
			c()
		}
	}
	e.cleanups = nil
}

func (e *Playground) ID() ExhibitID                           { return ExhibitPlayground }
func (e *Playground) Title() string                           { return "Mark Playground" }
func (e *Playground) Build(s *state.AppState) facet.FacetImpl { return e }

// playgroundCard builds one playCard hosting one exercise control. The card is
// the demo's bespoke host (play_card.go, F-card-content) rather than the
// framework structure.Card, because the framework Card self-projects its
// content without attaching it to the facet tree and so cannot host interactive
// marks.
func playgroundCard(title string, children ...facet.FacetImpl) *playCard {
	return newPlayCard(title, children...)
}
