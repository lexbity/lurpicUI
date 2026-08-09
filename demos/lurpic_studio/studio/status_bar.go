package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/status"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

// statusStripHeight bounds the status marks' measured height so the strip
// stays slim regardless of the marks' labelled content height.
const statusStripHeight float32 = 44

// StatusBar is the bottom status strip wired to real shell state (FR-status):
// the status_light reflects the feed connection, the progress_bar/ring track
// the streaming job progress in lock-step, the badge reflects the live row
// count, and the caption names the active exhibit. Like ChromeStack it is a
// linear-kind group-parent host that arranges its mark children directly
// (F-linear-marks).
type StatusBar struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	light   *status.StatusLight
	bar     *status.ProgressBar
	ring    *status.ProgressRing
	badge   *status.Badge
	caption facet.FacetImpl

	notConnected *store.Derived[bool]
	titleText    *store.Derived[string]

	gap        float32
	padX       float32
	padY       float32
	background gfx.Color
}

// NewStatusBar builds the status strip over the shared shell state and the
// streaming feed (E1). titleOf maps an exhibit id to its display title.
func NewStatusBar(themeCtx theme.ResolvedContext, shell *ShellState, feed *Feed, titleOf func(ExhibitID) string) *StatusBar {
	notConnected := store.NewDerived(func() bool { return !shell.Connection.Get() }, shell.Connection)
	titleText := store.NewDerived(func() string {
		return titleOf(shell.ActiveExhibit.Get())
	}, shell.ActiveExhibit)

	s := &StatusBar{
		light:        status.NewStatusLight("connection"),
		bar:          status.NewProgressBar("feed"),
		ring:         status.NewProgressRing("feed"),
		badge:        status.NewBadge(""),
		caption:      primitive.NewText(marks.Const("Lurpic Studio")),
		notConnected: notConnected,
		titleText:    titleText,
		gap:          float32(themeCtx.Spacing(theme.SpacingM)),
		padX:         float32(themeCtx.Spacing(theme.SpacingL)),
		padY:         float32(themeCtx.Spacing(theme.SpacingXS)),
		background:   themeCtx.Color(theme.ColorSurfaceVariant),
	}
	s.light.ShowLabel = marks.Const(false)
	s.light.Disabled = marks.FromDerived(notConnected, facet.DirtyProjection)
	s.bar.Value = marks.FromStore(feed.JobProgress, facet.DirtyProjection)
	// The ring carries no label so the status strip stays slim (the progress
	// bar already names the feed).
	s.ring.Label = marks.Const("")
	s.ring.Value = marks.FromStore(feed.JobProgress, facet.DirtyProjection)
	s.badge.Label = marks.FromDerived(shell.RowCount, facet.DirtyProjection)
	s.caption.(*primitive.Text).Content = marks.FromDerived(titleText, facet.DirtyProjection)

	s.Facet = facet.NewFacet()
	s.AddChild(s.light.Base())
	s.AddChild(s.bar.Base())
	s.AddChild(s.ring.Base())
	s.AddChild(s.badge.Base())
	s.AddChild(s.caption.Base())

	s.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke linear-kind group-parent host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
			return s.measure(ctx, constraints)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			s.arrange(ctx, bounds)
		},
	}
	s.layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearHorizontal,
		Policy:   groupPolicy{kind: facet.GroupLayoutLinearHorizontal, host: s},
		Children: s,
	}
	s.layout.Child = linearChildContract(facet.StretchPolicy{
		Width:  facet.StretchAlways,
		Height: facet.StretchNever,
	})
	s.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(s.background)})
		},
	}
	s.AddRole(&s.layout)
	s.AddRole(&s.render)
	return s
}

// Light returns the connection status_light.
func (s *StatusBar) Light() *status.StatusLight { return s.light }

// Bar returns the feed progress_bar.
func (s *StatusBar) Bar() *status.ProgressBar { return s.bar }

// Ring returns the feed progress_ring.
func (s *StatusBar) Ring() *status.ProgressRing { return s.ring }

// Badge returns the row-count badge.
func (s *StatusBar) Badge() *status.Badge { return s.badge }

// Caption returns the active-exhibit caption facet.
func (s *StatusBar) Caption() facet.FacetImpl { return s.caption }

func (s *StatusBar) items() []facet.FacetImpl {
	return []facet.FacetImpl{s.light, s.bar, s.ring, s.badge, s.caption}
}

func (s *StatusBar) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	items := s.items()
	// Bound the marks' height so the status strip stays slim: the ring/bar
	// would otherwise size to their labelled content height.
	itemC := facet.Constraints{MaxSize: gfx.Size{W: constraints.MaxSize.W, H: statusStripHeight}}
	width := s.padX * 2
	height := float32(0)
	for i, item := range items {
		role := item.Base().LayoutRole()
		role.Measure(ctx, itemC)
		size := role.MeasuredSize
		width += size.W
		if i < len(items)-1 {
			width += s.gap
		}
		if size.H > height {
			height = size.H
		}
	}
	height += s.padY * 2
	return facet.MeasureResult{Size: gfx.Size{W: width, H: height}}
}

func (s *StatusBar) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if bounds.IsEmpty() {
		return
	}
	items := s.items()
	// The progress_bar is the flexible segment: its mark measures itself to the
	// full available width (ProgressBar.measure claims constraints.MaxSize.W),
	// so the strip must clamp it to the width left after the fixed-size items
	// (status_light, progress_ring, badge, caption) are reserved — otherwise it
	// shoves the ring/badge/caption past the window's right edge.
	const flex = 1 // s.bar
	reserved := s.padX * 2
	for i, item := range items {
		if i == flex {
			continue
		}
		if role := item.Base().LayoutRole(); role != nil {
			reserved += role.MeasuredSize.W
		}
	}
	reserved += s.gap * float32(len(items)-1)
	barW := bounds.Width() - reserved
	if barW < 1 {
		barW = 1
	}
	x := bounds.Min.X + s.padX
	for i, item := range items {
		role := item.Base().LayoutRole()
		w := role.MeasuredSize.W
		if i == flex {
			w = barW
		}
		arrangeChild(facet.ArrangeContext{}, item, gfx.RectFromXYWH(x, bounds.Min.Y, w, bounds.Height()))
		x += w + s.gap
	}
}

// Children returns the status bar's group children.
func (s *StatusBar) Children() []facet.GroupChild {
	return linearGroupChildren(s.items())
}

func (s *StatusBar) Base() *facet.Facet             { s.BindImpl(s); return &s.Facet }
func (s *StatusBar) OnAttach(_ facet.AttachContext) {}
func (s *StatusBar) OnDetach()                      {}
func (s *StatusBar) OnActivate()                    {}
func (s *StatusBar) OnDeactivate()                  {}
