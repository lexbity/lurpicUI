package chart

import (
	"fmt"
	"strings"
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

const (
	defaultAxisLength     = float32(240)
	defaultAxisThickness  = float32(72)
	defaultTickCount      = 5
	defaultLabelPadding   = float32(8)
	defaultBaselineInset  = float32(20)
	defaultTickLength     = float32(8)
	defaultGridAlpha      = float32(0.18)
	defaultLabelCharWidth = float32(7)
	defaultTitleHeight    = float32(18)
)

// Axis renders chart axes, ticks, labels, and optional grid lines.
type Axis struct {
	ID          string
	Orientation AxisOrientation
	Scale       ScaleModel
	ShowGrid    bool
	Title       string

	base         facet.Facet
	once         sync.Once
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	cacheKey     string
	cacheList    *gfx.CommandList
	cacheHits    int
	cacheMisses  int
}

// AxisOrientation selects the baseline direction.
type AxisOrientation uint8

const (
	AxisTop AxisOrientation = iota
	AxisRight
	AxisBottom
	AxisLeft
)

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyChart,
		ConstructionClass: marks.ConstructionGenerated,
		Type:              marks.TypeName("chart:axis"),
		AnchorExporting:   true,
		HitTestable:       true,
	})
}

func (a *Axis) Base() *facet.Facet { a.ensureInit(); return &a.base }
func (a *Axis) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyChart, ConstructionClass: marks.ConstructionGenerated, Type: marks.TypeName("chart:axis"), AnchorExporting: true, HitTestable: true}
}
func (a *Axis) AuthoredID() string               { return a.ID }
func (a *Axis) OnAttach(ctx facet.AttachContext) { a.syncRoles() }
func (a *Axis) OnDetach()                        {}
func (a *Axis) OnActivate()                      {}
func (a *Axis) OnDeactivate()                    {}

func (a *Axis) ensureInit() {
	a.once.Do(func() {
		a.base.BindImpl(a)
		a.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			b := a.bounds()
			return gfx.Size{W: b.Width(), H: b.Height()}
		}}
		a.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		a.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return a.project(ctx) }}
		a.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if a.bounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		a.base.AddRole(a.layoutRole)
		a.base.AddRole(a.viewportRole)
		a.base.AddRole(a.projection)
		a.base.AddRole(a.hitRole)
		a.syncRoles()
	})
}

func (a *Axis) syncRoles() {
	syncLayout(a.layoutRole, a.bounds())
	syncViewport(a.viewportRole, gfx.Identity())
}

func (a *Axis) bounds() gfx.Rect {
	switch a.Orientation {
	case AxisLeft, AxisRight:
		return gfx.RectFromXYWH(0, 0, defaultAxisThickness, defaultAxisLength+defaultAxisThickness)
	default:
		return gfx.RectFromXYWH(0, 0, defaultAxisLength+defaultAxisThickness, defaultAxisThickness)
	}
}

func (a *Axis) axisLength() float32 {
	switch a.Orientation {
	case AxisLeft, AxisRight:
		return a.bounds().Height() - defaultBaselineInset*2
	default:
		return a.bounds().Width() - defaultBaselineInset*2
	}
}

func (a *Axis) axisStart() float32 {
	return defaultBaselineInset
}

func (a *Axis) axisEnd() float32 {
	return a.axisStart() + a.axisLength()
}

func (a *Axis) tickValues() []any {
	if a.Scale != nil {
		values := a.Scale.Ticks(defaultTickCount)
		if len(values) > 0 {
			return values
		}
	}
	out := make([]any, defaultTickCount)
	for i := range out {
		out[i] = float64(i)
	}
	return out
}

func (a *Axis) tickPositions() []float32 {
	values := a.tickValues()
	out := make([]float32, 0, len(values))
	for _, value := range values {
		if a.Scale != nil {
			out = append(out, a.Scale.Map(value))
			continue
		}
		denom := float32(len(values) - 1)
		if denom <= 0 {
			denom = 1
		}
		out = append(out, a.axisStart()+float32(len(out))*((a.axisEnd()-a.axisStart())/denom))
	}
	return out
}

func (a *Axis) baseline() (gfx.Point, gfx.Point) {
	switch a.Orientation {
	case AxisTop:
		y := defaultBaselineInset
		return gfx.Point{X: a.axisStart(), Y: y}, gfx.Point{X: a.axisEnd(), Y: y}
	case AxisBottom:
		y := a.bounds().Height() - defaultBaselineInset
		return gfx.Point{X: a.axisStart(), Y: y}, gfx.Point{X: a.axisEnd(), Y: y}
	case AxisLeft:
		x := defaultBaselineInset
		return gfx.Point{X: x, Y: a.axisStart()}, gfx.Point{X: x, Y: a.axisEnd()}
	default:
		x := a.bounds().Width() - defaultBaselineInset
		return gfx.Point{X: x, Y: a.axisStart()}, gfx.Point{X: x, Y: a.axisEnd()}
	}
}

func (a *Axis) tickMarkRect(pos float32) gfx.Rect {
	switch a.Orientation {
	case AxisTop:
		return gfx.RectFromXYWH(pos, defaultBaselineInset, 1, defaultTickLength)
	case AxisBottom:
		return gfx.RectFromXYWH(pos, a.bounds().Height()-defaultBaselineInset-defaultTickLength, 1, defaultTickLength)
	case AxisLeft:
		return gfx.RectFromXYWH(defaultBaselineInset-defaultTickLength, pos, defaultTickLength, 1)
	default:
		return gfx.RectFromXYWH(a.bounds().Width()-defaultBaselineInset, pos, defaultTickLength, 1)
	}
}

func (a *Axis) labelBox(index int, value any, pos float32) gfx.Rect {
	label := a.formatTick(value)
	width := float32(len(label))*defaultLabelCharWidth + defaultLabelPadding
	switch a.Orientation {
	case AxisTop:
		return gfx.RectFromXYWH(pos-width/2, defaultBaselineInset+defaultTickLength+2, width, defaultTitleHeight)
	case AxisBottom:
		return gfx.RectFromXYWH(pos-width/2, a.bounds().Height()-defaultBaselineInset-defaultTickLength-defaultTitleHeight-2, width, defaultTitleHeight)
	case AxisLeft:
		return gfx.RectFromXYWH(defaultBaselineInset-defaultTickLength-width-2, pos-defaultTitleHeight/2, width, defaultTitleHeight)
	default:
		return gfx.RectFromXYWH(a.bounds().Width()-defaultBaselineInset+defaultTickLength+2, pos-defaultTitleHeight/2, width, defaultTitleHeight)
	}
}

func (a *Axis) titleBox() gfx.Rect {
	if strings.TrimSpace(a.Title) == "" {
		return gfx.Rect{}
	}
	width := float32(len(a.Title))*defaultLabelCharWidth + 16
	switch a.Orientation {
	case AxisTop:
		return gfx.RectFromXYWH(a.axisStart(), 0, width, defaultTitleHeight)
	case AxisBottom:
		return gfx.RectFromXYWH(a.axisStart(), a.bounds().Height()-defaultTitleHeight, width, defaultTitleHeight)
	case AxisLeft:
		return gfx.RectFromXYWH(0, a.axisStart(), defaultTitleHeight, width)
	default:
		return gfx.RectFromXYWH(a.bounds().Width()-defaultTitleHeight, a.axisStart(), defaultTitleHeight, width)
	}
}

func (a *Axis) gridLineRects(pos float32) []gfx.Rect {
	if !a.ShowGrid {
		return nil
	}
	switch a.Orientation {
	case AxisTop, AxisBottom:
		return []gfx.Rect{gfx.RectFromXYWH(pos, defaultBaselineInset, 1, a.bounds().Height()-defaultBaselineInset*2)}
	default:
		return []gfx.Rect{gfx.RectFromXYWH(defaultBaselineInset, pos, a.bounds().Width()-defaultBaselineInset*2, 1)}
	}
}

func (a *Axis) labelBoxes() []gfx.Rect {
	values := a.tickValues()
	positions := a.tickPositions()
	out := make([]gfx.Rect, 0, len(values))
	for i, value := range values {
		out = append(out, a.labelBox(i, value, positions[i]))
	}
	return out
}

func (a *Axis) tickAnchors() layout.AnchorSet {
	anchors := layout.AnchorSet{}
	start, end := a.baseline()
	anchors["baseline-start"] = start
	anchors["baseline-end"] = end
	values := a.tickValues()
	positions := a.tickPositions()
	for i, value := range values {
		pos := positions[i]
		label := a.formatTick(value)
		var anchor gfx.Point
		switch a.Orientation {
		case AxisTop:
			anchor = gfx.Point{X: pos, Y: defaultBaselineInset}
		case AxisBottom:
			anchor = gfx.Point{X: pos, Y: a.bounds().Height() - defaultBaselineInset}
		case AxisLeft:
			anchor = gfx.Point{X: defaultBaselineInset, Y: pos}
		default:
			anchor = gfx.Point{X: a.bounds().Width() - defaultBaselineInset, Y: pos}
		}
		anchors[layout.AnchorID(fmt.Sprintf("tick-%d", i))] = anchor
		anchors[layout.AnchorID("tick-"+label)] = anchor
	}
	if title := a.titleBox(); !title.IsEmpty() {
		anchors["title-center"] = gfx.Point{X: title.Min.X + title.Width()/2, Y: title.Min.Y + title.Height()/2}
	}
	return anchors
}

// ExportAnchors exports baseline, tick, and title anchors.
func (a *Axis) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	a.ensureInit()
	transform := gfx.Identity()
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform
	}
	return transformAnchors(transform, a.tickAnchors())
}

func (a *Axis) formatTick(value any) string {
	if a.Scale != nil {
		return a.Scale.FormatTick(value)
	}
	return fmt.Sprint(value)
}

func (a *Axis) OnLayerSpecs() []layout.LayerSpec {
	return []layout.LayerSpec{{
		ID:          1,
		Placement:   layout.PlacementProjected,
		Measurement: layout.MeasureNonStructural,
		CoordSpace:  layout.CoordViewport,
		HitPolicy:   layout.HitNormal,
		RenderOrder: 250,
	}}
}

func (a *Axis) project(ctx facet.ProjectionContext) *gfx.CommandList {
	signature := a.cacheSignature()
	if a.cacheList != nil && signature == a.cacheKey {
		a.cacheHits++
		return a.cacheList
	}
	a.cacheMisses++
	var list gfx.CommandList
	start, end := a.baseline()
	list.Add(gfx.DrawPolyline{
		Points: []gfx.Point{start, end},
		Stroke: gfx.DefaultStroke(2),
		Brush:  gfx.SolidBrush(strokeColor(theme.FromToken(gfx.Color{A: 1}), gfx.Color{R: 0.2, G: 0.2, B: 0.2, A: 1})),
	})
	values := a.tickValues()
	positions := a.tickPositions()
	for i, value := range values {
		pos := positions[i]
		rect := a.tickMarkRect(pos)
		list.Add(gfx.FillRect{Rect: rect, Brush: gfx.SolidBrush(gfx.Color{R: 0.2, G: 0.2, B: 0.2, A: 1})})
		if a.ShowGrid {
			for _, grid := range a.gridLineRects(pos) {
				list.Add(gfx.FillRect{Rect: grid, Brush: gfx.SolidBrush(gfx.Color{R: 0.6, G: 0.6, B: 0.6, A: defaultGridAlpha})})
			}
		}
		label := a.labelBox(i, value, pos)
		list.Add(gfx.FillRect{Rect: label, Brush: gfx.SolidBrush(gfx.Color{R: 0.1, G: 0.1, B: 0.1, A: 1})})
	}
	if title := a.titleBox(); !title.IsEmpty() {
		list.Add(gfx.FillRect{Rect: title, Brush: gfx.SolidBrush(gfx.Color{R: 0.15, G: 0.15, B: 0.15, A: 1})})
	}
	a.cacheKey = signature
	a.cacheList = &list
	return a.cacheList
}

func (a *Axis) cacheSignature() string {
	if a.Scale == nil {
		return fmt.Sprintf("%d|%t|%s|nil|%v", a.Orientation, a.ShowGrid, a.Title, a.tickPositions())
	}
	return fmt.Sprintf(
		"%d|%t|%s|%s|%v|%v",
		a.Orientation,
		a.ShowGrid,
		a.Title,
		a.Scale.Kind(),
		a.Scale.Ticks(defaultTickCount),
		a.tickPositions(),
	)
}

// CacheStats returns the number of cache hits and misses for diagnostics.
func (a *Axis) CacheStats() (hits, misses int) {
	return a.cacheHits, a.cacheMisses
}
