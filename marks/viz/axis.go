package viz

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/scale"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// AxisOrientation describes which side of the plot the axis is on.
type AxisOrientation uint8

const (
	AxisBottom AxisOrientation = iota
	AxisLeft
	AxisTop
	AxisRight
)

// Axis renders tick marks and labels for a scale.
type Axis struct {
	marks.Core

	Scale       *reactive.ReactiveScale
	Orientation marks.Binding[AxisOrientation]
	TickCount   marks.Binding[int]
	TickLength  marks.Binding[float32]
	LabelSize   marks.Binding[float32]
	LabelColor  gfx.Color

	fonts   *text.FontRegistry
	shaper  *text.Shaper
	entries []axisEntry

	themeFontFamily string
	themeLabelSize  float32
	themeTickLength float32
	themeLabelColor gfx.Color
	themeTickColor  gfx.Color

	cleanups []func()
}

type axisEntry struct {
	Value float64
	Label string
	Pixel float64
}

var _ facet.FacetImpl = (*Axis)(nil)
var _ layout.AnchorExporter = (*Axis)(nil)
var _ marks.Mark = (*Axis)(nil)

// NewAxis constructs an axis mark.
func NewAxis(scale *reactive.ReactiveScale, orientation marks.Binding[AxisOrientation], fonts *text.FontRegistry) *Axis {
	a := &Axis{
		Scale:           scale,
		Orientation:     orientation,
		TickCount:       marks.Const(5),
		TickLength:      marks.Const[float32](0),
		LabelSize:       marks.Const[float32](0),
		LabelColor:      gfx.Color{},
		themeLabelColor: gfx.Color{R: 0.3, G: 0.3, B: 0.3, A: 1}, //lurpiclint:ignore LL028 -- theme-synced fallback, overridden at runtime
		themeTickColor:  gfx.Color{R: 0.3, G: 0.3, B: 0.3, A: 1}, //lurpiclint:ignore LL028 -- theme-synced fallback, overridden at runtime
		fonts:           fonts,
	}
	a.Facet = facet.NewFacet()
	if a.fonts != nil {
		a.shaper = text.NewShaper(a.fonts)
	}
	a.AddBinding(a.Orientation)
	a.AddBinding(a.TickCount)
	a.AddBinding(a.TickLength)
	a.AddBinding(a.LabelSize)

	a.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		a.syncTheme(ctx)
		s := a.measureSize()
		return facet.MeasureResult{Size: s}
	}
	a.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		a.Layout.ArrangedBounds = bounds
		a.syncThemeWithTheme(ctx.Theme)
		a.computeEntries()
	}
	a.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return a.buildCommands(a.Layout.ArrangedBounds)
	}
	a.RegisterRoles()
	return a
}

func (a *Axis) Base() *facet.Facet {
	a.BindImpl(a)
	return &a.Facet
}

func (a *Axis) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "viz", TypeName: "axis"}
}

func (a *Axis) OnAttach(ctx facet.AttachContext) {
	a.Core.OnAttach(ctx)
	if a.Scale != nil {
		signal.Track(a.Subs(), &a.Scale.OnChange, func(signal.Unit) {
			a.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
		})
	}
}
func (a *Axis) OnDetach() {
	a.Core.OnDetach()
	for _, c := range a.cleanups {
		if c != nil {
			c()
		}
	}
	a.cleanups = nil
	a.entries = nil
}
func (a *Axis) OnActivate()   { a.Core.OnActivate() }
func (a *Axis) OnDeactivate() { a.Core.OnDeactivate() }

func (a *Axis) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	return a.DefaultAnchors(a.Layout.ArrangedBounds, ctx)
}

func (a *Axis) measureSize() gfx.Size {
	tickLen := a.TickLength.Get()
	if tickLen == 0 {
		tickLen = a.themeTickLength
	}
	if tickLen == 0 {
		tickLen = 6
	}
	labelSize := a.LabelSize.Get()
	if labelSize == 0 {
		labelSize = a.themeLabelSize
	}
	if labelSize == 0 {
		labelSize = 11
	}
	switch a.Orientation.Get() {
	case AxisBottom, AxisTop:
		// The vertical extent is the tick length plus the single-line label
		// height (a fixed line estimate; label text is single-line).
		return gfx.Size{W: 0, H: tickLen + labelSize + 5}
	case AxisLeft, AxisRight:
		// RX-1 FR-15: the label column is the widest formatted tick label
		// measured via text metrics — a constant estimate clipped formatted
		// tick text and caused the A-10 dash-clipping class. The fallback is
		// the pre-FR-15 per-character estimate (no shaper attached).
		labelW := a.measureWidestLabel(labelSize)
		return gfx.Size{W: tickLen + labelW + 6, H: 0}
	}
	return gfx.Size{}
}

// measureWidestLabel measures the widest formatted tick label for the current
// scale domain through the shaper (RX-1 FR-15). It falls back to a
// per-character estimate when no shaper is available or the scale is not a
// Ticker.
func (a *Axis) measureWidestLabel(labelSize float32) float32 {
	fallback := labelSize + 5
	if a.Scale == nil || a.shaper == nil {
		return fallback
	}
	s := a.Scale.Get()
	ticker, ok := s.(scale.Ticker)
	if !ok {
		return fallback
	}
	style := text.TextStyle{Size: labelSize, Family: a.themeFontFamily}
	width := float32(0)
	for _, t := range ticker.Ticks(a.TickCount.Get()) {
		if t.Label == "" {
			continue
		}
		shaped := a.shaper.ShapeSimple(t.Label, style)
		if shaped == nil || len(shaped.Lines) == 0 || len(shaped.Lines[0].Runs) == 0 {
			continue
		}
		if w := shaped.Lines[0].Runs[0].Bounds.Width(); w > width {
			width = w
		}
	}
	if width <= 0 {
		return fallback
	}
	return width
}

func (a *Axis) computeEntries() {
	if a.Scale == nil {
		a.entries = nil
		return
	}
	s := a.Scale.Get()
	ticker, ok := s.(scale.Ticker)
	if !ok {
		a.entries = nil
		return
	}
	ticks := ticker.Ticks(a.TickCount.Get())
	a.entries = make([]axisEntry, 0, len(ticks))
	for _, t := range ticks {
		a.entries = append(a.entries, axisEntry{
			Value: t.Value,
			Label: t.Label,
			Pixel: s.Map(t.Value),
		})
	}
}

func (a *Axis) syncTheme(ctx facet.MeasureContext) {
	a.syncThemeWithTheme(ctx.Theme)
}

func (a *Axis) syncThemeWithTheme(t any) {
	if t != nil {
		var rc theme.ResolvedContext
		var ok bool
		if rc, ok = t.(theme.ResolvedContext); !ok {
			var rcp *theme.ResolvedContext
			if rcp, ok = t.(*theme.ResolvedContext); ok {
				rc = *rcp
			}
		}
		if ok {
			ts := rc.TextStyle(theme.TextLabelM)
			if a.themeFontFamily == "" {
				a.themeFontFamily = ts.Family
			}
			if a.themeLabelSize == 0 {
				a.themeLabelSize = ts.Size
			}
			if a.themeTickLength == 0 {
				a.themeTickLength = ts.Size * 0.5
			}
			if a.themeLabelColor == (gfx.Color{}) {
				a.themeLabelColor = rc.Color(theme.ColorText)
			}
			if a.themeTickColor == (gfx.Color{}) {
				a.themeTickColor = rc.Color(theme.ColorTextSecondary)
			}
		}
	}
}

func (a *Axis) buildCommands(bounds gfx.Rect) []gfx.Command {
	if bounds.IsEmpty() || len(a.entries) == 0 {
		return nil
	}
	cmds := make([]gfx.Command, 0, len(a.entries)*2)
	orient := a.Orientation.Get()
	tickLen := a.TickLength.Get()
	if tickLen == 0 {
		tickLen = a.themeTickLength
	}
	if tickLen == 0 {
		tickLen = 6
	}
	labelSize := a.LabelSize.Get()
	if labelSize == 0 {
		labelSize = a.themeLabelSize
	}
	if labelSize == 0 {
		labelSize = 11
	}
	fontFamily := a.themeFontFamily
	labelColor := a.LabelColor
	if labelColor == (gfx.Color{}) {
		labelColor = a.themeLabelColor
	}
	tickColor := a.themeTickColor
	brush := gfx.SolidBrush(labelColor)
	tickBrush := gfx.SolidBrush(tickColor)

	// Pre-measure label extents for collision avoidance.
	type labelSlot struct {
		entry axisEntry
		w, h  float32
		skip  bool
	}
	slots := make([]labelSlot, len(a.entries))

	style := text.TextStyle{Size: labelSize, Family: fontFamily}
	for i, e := range a.entries {
		slots[i].entry = e
		if a.shaper == nil || e.Label == "" {
			slots[i].skip = true
			continue
		}
		shaped := a.shaper.ShapeSimple(e.Label, style)
		if shaped == nil || len(shaped.Lines) == 0 || len(shaped.Lines[0].Runs) == 0 {
			slots[i].skip = true
			continue
		}
		run := shaped.Lines[0].Runs[0]
		slots[i].w = run.Bounds.Width()
		slots[i].h = shaped.Lines[0].Bounds.Height()
	}

	switch orient {
	case AxisBottom:
		var lastEnd float32
		for i, s := range slots {
			x := float32(s.entry.Pixel) + bounds.Min.X
			cmds = append(cmds, gfx.StrokePath{
				Path: gfx.Path{Segments: []gfx.PathSegment{
					{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{{X: x, Y: bounds.Min.Y}}},
					{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{{X: x, Y: bounds.Min.Y + tickLen}}},
				}},
				Stroke: gfx.StrokeStyle{Width: 1},
				Brush:  tickBrush,
			})
			if s.skip {
				continue
			}
			labelX := x - s.w/2
			if labelX < bounds.Min.X {
				labelX = bounds.Min.X
			}
			labelEnd := labelX + s.w
			if labelEnd > bounds.Max.X {
				labelX = bounds.Max.X - s.w
				if labelX < bounds.Min.X {
					continue
				}
			}
			if i > 0 && labelX < lastEnd {
				continue
			}
			lastEnd = labelEnd
			cmds = append(cmds, gfx.DrawGlyphRun{
				Run: a.shaper.ShapeSimple(s.entry.Label, style).Lines[0].Runs[0],
				Origin: gfx.Point{
					X: labelX,
					Y: bounds.Min.Y + tickLen + 2 + a.shaper.ShapeSimple(s.entry.Label, style).Lines[0].Baseline,
				},
				Brush: brush,
			})
		}

	case AxisTop:
		var lastEnd float32
		for i, s := range slots {
			x := float32(s.entry.Pixel) + bounds.Min.X
			cmds = append(cmds, gfx.StrokePath{
				Path: gfx.Path{Segments: []gfx.PathSegment{
					{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{{X: x, Y: bounds.Max.Y}}},
					{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{{X: x, Y: bounds.Max.Y - tickLen}}},
				}},
				Stroke: gfx.StrokeStyle{Width: 1},
				Brush:  tickBrush,
			})
			if s.skip {
				continue
			}
			labelX := x - s.w/2
			if labelX < bounds.Min.X {
				labelX = bounds.Min.X
			}
			labelEnd := labelX + s.w
			if labelEnd > bounds.Max.X {
				labelX = bounds.Max.X - s.w
				if labelX < bounds.Min.X {
					continue
				}
			}
			if i > 0 && labelX < lastEnd {
				continue
			}
			lastEnd = labelEnd
			shaped := a.shaper.ShapeSimple(s.entry.Label, style)
			cmds = append(cmds, gfx.DrawGlyphRun{
				Run: shaped.Lines[0].Runs[0],
				Origin: gfx.Point{
					X: labelX,
					Y: bounds.Max.Y - tickLen - 2,
				},
				Brush: brush,
			})
		}

	case AxisLeft:
		// Labels iterate bottom→top (the y-scale is inverted: pixel 0 is the
		// domain's high end). Collision tracks the previous (lower) label's top
		// — the old code tracked its bottom, so every label above the first was
		// skipped and the axis showed a single clipped label (the A-10
		// dash-clipping class, RX-1 FR-15).
		var lastTop float32
		for i, s := range slots {
			y := float32(s.entry.Pixel) + bounds.Min.Y
			cmds = append(cmds, gfx.StrokePath{
				Path: gfx.Path{Segments: []gfx.PathSegment{
					{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{{X: bounds.Max.X - tickLen, Y: y}}},
					{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{{X: bounds.Max.X, Y: y}}},
				}},
				Stroke: gfx.StrokeStyle{Width: 1},
				Brush:  tickBrush,
			})
			if s.skip {
				continue
			}
			labelY := y - s.h/2
			if labelY < bounds.Min.Y {
				labelY = bounds.Min.Y
			}
			labelEnd := labelY + s.h
			if labelEnd > bounds.Max.Y {
				labelY = bounds.Max.Y - s.h
				if labelY < bounds.Min.Y {
					continue
				}
			}
			if i > 0 && labelEnd > lastTop {
				continue
			}
			lastTop = labelY
			shaped := a.shaper.ShapeSimple(s.entry.Label, style)
			cmds = append(cmds, gfx.DrawGlyphRun{
				Run: shaped.Lines[0].Runs[0],
				Origin: gfx.Point{
					X: bounds.Max.X - tickLen - 2 - s.w,
					Y: labelY + shaped.Lines[0].Baseline,
				},
				Brush: brush,
			})
		}

	case AxisRight:
		// Same bottom→top label collision fix as AxisLeft (RX-1 FR-15).
		var lastTop float32
		for i, s := range slots {
			y := float32(s.entry.Pixel) + bounds.Min.Y
			cmds = append(cmds, gfx.StrokePath{
				Path: gfx.Path{Segments: []gfx.PathSegment{
					{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{{X: bounds.Min.X, Y: y}}},
					{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{{X: bounds.Min.X + tickLen, Y: y}}},
				}},
				Stroke: gfx.StrokeStyle{Width: 1},
				Brush:  tickBrush,
			})
			if s.skip {
				continue
			}
			labelY := y - s.h/2
			if labelY < bounds.Min.Y {
				labelY = bounds.Min.Y
			}
			labelEnd := labelY + s.h
			if labelEnd > bounds.Max.Y {
				labelY = bounds.Max.Y - s.h
				if labelY < bounds.Min.Y {
					continue
				}
			}
			if i > 0 && labelEnd > lastTop {
				continue
			}
			lastTop = labelY
			shaped := a.shaper.ShapeSimple(s.entry.Label, style)
			cmds = append(cmds, gfx.DrawGlyphRun{
				Run: shaped.Lines[0].Runs[0],
				Origin: gfx.Point{
					X: bounds.Min.X + tickLen + 2,
					Y: labelY + shaped.Lines[0].Baseline,
				},
				Brush: brush,
			})
		}
	}
	return cmds
}
