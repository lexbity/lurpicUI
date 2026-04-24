package shell

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// LogsPanelFacet displays event and state logs
type LogsPanelFacet struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	theme  theme.Context
	shaper *text.Shaper

	// Log entries
	entries    []LogEntry
	maxEntries int
}

// LogEntry represents a single log line
type LogEntry struct {
	Category string
	Message  string
	Time     string
	Ordinal  int
}

// NewLogsPanelFacet constructs the logs panel
func NewLogsPanelFacet(th theme.Context, shaper *text.Shaper) *LogsPanelFacet {
	l := &LogsPanelFacet{
		Facet:      facet.NewFacet(),
		theme:      th,
		shaper:     shaper,
		maxEntries: 100,
	}

	l.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: 120} // Fixed height for logs
	}
	l.layout.OnArrange = func(bounds gfx.Rect) {
		l.layout.ArrangedBounds = bounds
	}
	l.AddRole(&l.layout)

	l.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		l.renderPanel(list, bounds)
	}
	l.AddRole(&l.render)

	return l
}

func (l *LogsPanelFacet) Base() *facet.Facet {
	l.Facet.BindImpl(l)
	return &l.Facet
}

func (l *LogsPanelFacet) OnAttach(ctx facet.AttachContext) {}
func (l *LogsPanelFacet) OnDetach()                        {}
func (l *LogsPanelFacet) OnActivate()                      {}
func (l *LogsPanelFacet) OnDeactivate()                    {}

func (l *LogsPanelFacet) renderPanel(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	// Background
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(l.theme.Color(theme.ColorSurface)),
	})

	// Top border
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), 1),
		Brush: gfx.SolidBrush(l.theme.Color(theme.ColorBorder)),
	})

	if l.shaper == nil {
		return
	}

	// Header
	y := bounds.Min.Y + 8
	y = l.renderHeader(list, bounds, y)

	// Log entries (show last few that fit)
	lineHeight := float32(16)
	maxVisible := int((bounds.Max.Y - y) / lineHeight)
	if maxVisible < 1 {
		return
	}

	start := len(l.entries) - maxVisible
	if start < 0 {
		start = 0
	}

	for i := start; i < len(l.entries); i++ {
		entry := l.entries[i]
		y = l.renderLogEntry(list, bounds, y, entry)
		if y > bounds.Max.Y {
			break
		}
	}
}

func (l *LogsPanelFacet) renderHeader(list *gfx.CommandList, bounds gfx.Rect, y float32) float32 {
	text := "Event Log"
	style := l.theme.TextStyle(theme.TextLabelS)
	layout := l.shaper.ShapeSimple(text, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		origin := gfx.Point{X: bounds.Min.X + 12, Y: y + line.Baseline}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(l.theme.Color(theme.ColorTextSecondary)),
			})
		}
		return y + layout.Bounds.Height() + 4
	}
	return y + 16
}

func (l *LogsPanelFacet) renderLogEntry(list *gfx.CommandList, bounds gfx.Rect, y float32, entry LogEntry) float32 {
	// Format: [CATEGORY] message
	text := ""
	if entry.Ordinal > 0 {
		text += fmt.Sprintf("#%04d ", entry.Ordinal)
	}
	text += "[" + entry.Category + "] " + entry.Message
	style := l.theme.TextStyle(theme.TextMonoS)
	layout := l.shaper.ShapeSimple(text, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		origin := gfx.Point{X: bounds.Min.X + 12, Y: y + 12}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(l.theme.Color(theme.ColorText)),
			})
		}
	}
	return y + 16
}

// AppendLog adds a new log entry
func (l *LogsPanelFacet) AppendLog(entry LogEntry) {
	l.entries = append(l.entries, entry)

	// Trim if exceeding max
	if len(l.entries) > l.maxEntries {
		l.entries = l.entries[len(l.entries)-l.maxEntries:]
	}

	l.Invalidate(facet.DirtyProjection)
}

// Clear removes all log entries
func (l *LogsPanelFacet) Clear() {
	l.entries = nil
	l.Invalidate(facet.DirtyProjection)
}

// SetMaxEntries sets the maximum number of entries to keep
func (l *LogsPanelFacet) SetMaxEntries(max int) {
	l.maxEntries = max
	if len(l.entries) > max {
		l.entries = l.entries[len(l.entries)-max:]
		l.Invalidate(facet.DirtyProjection)
	}
}

// Entries returns a copy of the log entries in chronological order.
func (l *LogsPanelFacet) Entries() []LogEntry {
	if l == nil || len(l.entries) == 0 {
		return nil
	}
	out := make([]LogEntry, len(l.entries))
	copy(out, l.entries)
	return out
}
