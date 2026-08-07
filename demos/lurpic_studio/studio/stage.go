package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// Stage is the exhibit stage: exactly one active exhibit among many.
//
// F-stage resolved: the stage uses visibility-gating — only the active child
// is measured and arranged; inactive exhibits are arranged to zero bounds.
// The layout/stack alternative would overlay every child at one origin and
// measure all of them, so it needs the same gating while doing strictly more
// work (N measures instead of 1); it is therefore a deprecation candidate,
// not the stage's mechanism. A plain layout/linear host cannot gate either,
// because its measuredSize fallback re-measures any child with a zero
// MeasuredSize — so the stage is a bespoke single-active-child host.
type Stage struct {
	facet.Facet
	layout facet.LayoutRole

	activeExhibit *store.ValueStore[ExhibitID]
	roots         map[ExhibitID]facet.FacetImpl
	order         []Exhibit

	rt      facet.RuntimeServices
	cleanup func()
}

// NewStage builds the exhibit stage over the given catalog, building each
// exhibit's root from the shared app state. The active exhibit defaults to
// the first catalog entry.
func NewStage(exhibits []Exhibit, appState *state.AppState) *Stage {
	s := &Stage{
		activeExhibit: store.NewValueStore(ExhibitID("")),
		roots:         make(map[ExhibitID]facet.FacetImpl, len(exhibits)),
		order:         append([]Exhibit(nil), exhibits...),
	}
	if len(exhibits) > 0 {
		s.activeExhibit.Set(exhibits[0].ID())
	}
	s.Facet = facet.NewFacet()
	for _, e := range exhibits {
		root := e.Build(appState)
		if root == nil {
			continue
		}
		s.roots[e.ID()] = root
		s.AddChild(root.Base())
	}

	s.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke single-active-exhibit stage (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return s.measure(ctx, c)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			s.arrange(ctx, bounds)
		},
	}
	s.layout.Child = linearChildContract(facet.StretchPolicy{
		Width:  facet.StretchAlways,
		Height: facet.StretchAlways,
	})
	s.AddRole(&s.layout)
	return s
}

// ActiveExhibit returns the store driving which exhibit is shown.
func (s *Stage) ActiveExhibit() *store.ValueStore[ExhibitID] { return s.activeExhibit }

// Exhibits returns the catalog order.
func (s *Stage) Exhibits() []Exhibit { return append([]Exhibit(nil), s.order...) }

// ActiveRoot returns the currently active exhibit's root facet.
func (s *Stage) ActiveRoot() facet.FacetImpl { return s.roots[s.activeExhibit.Get()] }

func (s *Stage) measure(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
	if role := s.activeRootRole(); role != nil {
		role.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
	}
	return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
}

func (s *Stage) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	active := s.activeExhibit.Get()
	for id, root := range s.roots {
		role := root.Base().LayoutRole()
		if role == nil {
			continue
		}
		if id == active {
			role.Arrange(ctx, bounds)
		} else {
			role.Arrange(ctx, gfx.Rect{})
		}
	}
}

func (s *Stage) activeRootRole() *facet.LayoutRole {
	root := s.ActiveRoot()
	if root == nil || root.Base() == nil {
		return nil
	}
	return root.Base().LayoutRole()
}

func (s *Stage) OnAttach(ctx facet.AttachContext) {
	s.rt = ctx.Runtime
	id := s.activeExhibit.OnChange.Subscribe(func(signal.Change[ExhibitID]) {
		invalidateLayout(s, s.rt, "stage.activeExhibit")
	})
	s.cleanup = func() { s.activeExhibit.OnChange.Unsubscribe(id) }
}

func (s *Stage) OnDetach() {
	if s.cleanup != nil {
		s.cleanup()
		s.cleanup = nil
	}
}

func (s *Stage) Base() *facet.Facet { s.BindImpl(s); return &s.Facet }
func (s *Stage) OnActivate()        {}
func (s *Stage) OnDeactivate()      {}
