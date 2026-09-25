package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/status"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

// statusStripHeight bounds the status marks' measured height so the strip
// stays slim regardless of the marks' labelled content height.
const statusStripHeight float32 = 44

// StatusBar is the bottom status strip wired to real shell state (FR-status):
// the status_light reflects the feed connection, the progress_bar/ring track
// the streaming job progress in lock-step, the badge reflects the live row
// count, and the caption names the active exhibit.
//
// The strip is a structure.Row composition (RX-2 P1): the progress bar is the
// weighted segment (Weight 1 — it absorbs the free width), every other mark
// hugs its measured size. The row hosts the marks as real tree children, so
// the runtime projects and hit-tests them at their arranged bounds; the strip
// itself only draws its background and delegates measure/arrange to the row.
type StatusBar struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	row   *structure.Row
	light *status.StatusLight
	bar   *status.ProgressBar
	ring  *status.ProgressRing
	badge *status.Badge

	caption      *primitive.Text
	notConnected *store.Derived[bool]
	titleText    *store.Derived[string]

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
		background:   themeCtx.Color(theme.ColorSurfaceVariant),
	}
	s.light.ShowLabel = marks.Const(false)
	s.light.Disabled = marks.FromDerived(notConnected, facet.DirtyProjection)
	// Progress value and badge/caption text change the marks' measured size
	// (bar length, badge width, caption length), so these bindings declare
	// DirtyLayout — under RX-1 FR-3 a DirtyLayout-declaring content change
	// re-measures through the status bar's layout root.
	s.bar.Value = marks.FromStore(feed.JobProgress, facet.DirtyLayout|facet.DirtyProjection)
	// The ring carries no label so the status strip stays slim (the progress
	// bar already names the feed).
	s.ring.Label = marks.Const("")
	s.ring.Value = marks.FromStore(feed.JobProgress, facet.DirtyLayout|facet.DirtyProjection)
	s.badge.Label = marks.FromDerived(shell.RowCount, facet.DirtyLayout|facet.DirtyProjection)
	s.caption.Content = marks.FromDerived(titleText, facet.DirtyLayout|facet.DirtyProjection)

	// The binding fields above are assigned after construction (the marks'
	// constructors register their own default Const bindings via AddBinding).
	// Register the replaced bindings explicitly so the marks' OnAttach
	// subscribes them — an unregistered binding field reads live but never
	// invalidates (RX-1 A-6; replaced by declared bindings in RX-2 P3).
	s.light.AddBinding(s.light.Disabled)
	s.bar.AddBinding(s.bar.Value)
	s.ring.AddBinding(s.ring.Value)
	s.badge.AddBinding(s.badge.Label)
	s.caption.AddBinding(s.caption.Content)

	// The weighted bar absorbs the free width after the hug-sized light,
	// ring, badge, and caption are placed; vertical centering keeps the
	// slim marks aligned in the strip.
	s.row = structure.NewRow(
		[]structure.AxisChild{
			{Facet: s.light, MarkID: 1},
			{Facet: s.bar, MarkID: 2, Weight: 1},
			{Facet: s.ring, MarkID: 3},
			{Facet: s.badge, MarkID: 4},
			{Facet: s.caption, MarkID: 5},
		},
		structure.AxisConfig{
			Gap:        float32(themeCtx.Spacing(theme.SpacingM)),
			PadX:       float32(themeCtx.Spacing(theme.SpacingL)),
			PadY:       float32(themeCtx.Spacing(theme.SpacingXS)),
			CrossAlign: structure.CrossAlignCenter,
		},
	)

	s.Facet = facet.NewFacet()
	s.AddChild(s.row.Base()) //lurpiclint:ignore LL021 -- the shell hosts the composition row as a regular child, not an overlay

	s.layout = facet.LayoutRole{ //lurpiclint:ignore * -- single-child wrapper: background fill + measure/arrange delegation to the row (structure.Row owns the layout)
		OnMeasure: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
			// Clamp the row's height so labelled marks cannot inflate the
			// strip; the row measures its children under the same bound.
			c := constraints
			c.MaxSize.H = statusStripHeight
			result := s.row.Base().LayoutRole().Measure(ctx, c)
			s.layout.MeasuredSize = result.Size
			return result
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			s.layout.ArrangedBounds = bounds
			if role := s.row.Base().LayoutRole(); role != nil {
				role.Arrange(ctx, bounds)
			}
		},
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

// Caption returns the active-exhibit caption text mark.
func (s *StatusBar) Caption() *primitive.Text { return s.caption }

// Row returns the strip's composition row (the structure.row coverage
// instance).
func (s *StatusBar) Row() *structure.Row { return s.row }

func (s *StatusBar) Base() *facet.Facet             { s.BindImpl(s); return &s.Facet }
func (s *StatusBar) OnAttach(_ facet.AttachContext) {}
func (s *StatusBar) OnDetach()                      {}
func (s *StatusBar) OnActivate()                    {}
func (s *StatusBar) OnDeactivate()                  {}
