package studio

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/structure"
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
// The family bodies are standard structure.scroll_region marks hosting
// structure.card compositions (RX-2 P2: both marks attach their content as
// real facet-tree children, so the interactive playground marks are projected
// and hit-tested by the runtime); the active body is arranged into the tabs'
// panel and the inactive bodies to zero bounds, exactly like the Stage gates
// its exhibits.
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

	bodies   []facet.FacetImpl
	cleanups []func()
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
	e.AddChild(e.tabs.Base())
	// The family bodies are real facet-tree children so their cards are
	// projected and hit-tested by the runtime. The tabs mark arranges the
	// active body into its panel; the inactive bodies are arranged to zero
	// bounds below (the Stage gating idiom).
	for _, body := range e.bodies {
		if body != nil && body.Base() != nil {
			e.AddChild(body.Base())
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
	// Tab switching changes which family body is active — a content change
	// (the body set is unchanged; only the active body's arrangement changes).
	// Route it through the RX-1 FR-3 propagation entry point so the host
	// re-measures and re-arranges the newly active family body; the tabs mark
	// then measures/arranges that body from its panel bounds.
	tabID := e.activeTab.OnChange.Subscribe(func(signal.Change[int]) {
		layout.PropagateContentDirty(e, ctx.Runtime, "playground.activeTab", facet.DirtyLayout|facet.DirtyProjection)
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

// playgroundCard builds one structure.Card hosting one exercise control: the
// title text spans the top row and the exercise mark(s) share the body row in
// equal columns (RX-2 P2: the Card attaches content as real tree children, so
// the marks inside receive input).
func playgroundCard(title string, children ...facet.FacetImpl) *structure.Card {
	card := structure.NewCard(title)
	n := len(children)
	if n == 0 {
		n = 1
	}
	card.GridColumns = marks.Const(n)
	card.GridRows = marks.Const(2)
	content := []structure.CardChild{{
		Key:   "title",
		Facet: primitive.NewText(marks.Const(title)),
		Grid:  facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: n, RowSpan: 1},
	}}
	for i, child := range children {
		if child == nil {
			continue
		}
		content = append(content, structure.CardChild{
			Key:   fmt.Sprintf("body%d", i),
			Facet: child,
			Grid:  facet.GridPlacement{ColStart: i, RowStart: 1, ColSpan: 1, RowSpan: 1},
		})
	}
	card.ChildrenContent = content
	return card
}

// newPlayScroll builds the family body: a structure.scroll_region stacking the
// family's cards with the shared playground gap (the scroll region hosts its
// children as real tree members — RX-2 P2).
func newPlayScroll(gap float32, cards ...facet.FacetImpl) *structure.ScrollRegion {
	sr := structure.NewScrollRegion("Playground")
	sr.Gap = marks.Const(gap)
	kids := make([]structure.ScrollRegionChild, 0, len(cards))
	for i, c := range cards {
		if c == nil {
			continue
		}
		kids = append(kids, structure.ScrollRegionChild{Facet: c, MarkID: facet.MarkID(100 + i)})
	}
	sr.SetChildren(kids)
	return sr
}
