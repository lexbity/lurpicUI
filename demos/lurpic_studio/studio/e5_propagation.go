package studio

import (
	"strconv"
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/status"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// PropagationData is the E5 exhibit descriptor; Build constructs the facet
// over the shared dirty sink (F-diag-access: main owns the sink, shares it by
// pointer).
type PropagationData struct {
	sink  *DirtySink
	fonts *text.FontRegistry
	theme theme.ResolvedContext
}

// NewPropagationData builds the E5 descriptor.
func NewPropagationData(sink *DirtySink, fonts *text.FontRegistry, themeCtx theme.ResolvedContext) *PropagationData {
	return &PropagationData{sink: sink, fonts: fonts, theme: themeCtx}
}

func (p *PropagationData) ID() ExhibitID { return ExhibitPropagation }
func (p *PropagationData) Title() string { return "Reactive Propagation" }
func (p *PropagationData) Build(_ *state.AppState) facet.FacetImpl {
	return NewPropagationFacet(p.sink, p.fonts, p.theme)
}

// propagationNode is one facet in the captured shell tree.
type propagationNode struct {
	id     facet.FacetID
	depth  int
	label  string
	bounds gfx.Rect
}

// dirtyInfo is the latest retained dirty state for one facet.
type dirtyInfo struct {
	flags  facet.DirtyFlags
	source string
	frame  uint64
}

// edgeIntrospectionNote is the R-fake-viz honesty label: store-level causal
// provenance and dependency edges are not introspectable, so the exhibit says
// so rather than fabricating edge data (F-edges, deferred).
const edgeIntrospectionNote = "store-level causal edges: not yet introspectable (F-edges)"

// Propagation is the E5 facet: it renders the shell's facet tree with the
// dirty-set waves observed through the DirtySnapshotSink, labeling each dirty
// node with its invalidation source. The dirty-node highlighting is drawn by
// the framework's diagnostics.Overlay (F-overlay-precedent: HighlightDirty),
// not a parallel dirty-highlight renderer. Store-level causal provenance and
// dependency edges are not introspectable and are labeled as such, never
// fabricated (R-fake-viz).
type Propagation struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	tick   facet.TickRole

	sink *DirtySink
	rt   facet.RuntimeServices

	// overlay is the framework's dirty-highlight renderer (F-overlay-precedent):
	// E5 reuses diagnostics.Overlay.HighlightDirty for its dirty-node
	// highlighting instead of authoring a parallel dirty-highlight renderer.
	overlay *diagnostics.Overlay

	paused     *store.ValueStore[bool]
	retention  *store.ValueStore[float64]
	dirtyCount *store.ValueStore[string]
	notLive    *store.ValueStore[bool]

	controls *structure.Card
	badge    *status.Badge
	light    *status.StatusLight

	shaper    *text.Shaper
	textStyle text.TextStyle
	textColor gfx.Color
	dimColor  gfx.Color
	bg        gfx.Color
	nodeFill  gfx.Color
	nodeEdge  gfx.Color

	tree     []propagationNode
	treeArea gfx.Rect
	active   bool

	cleanup func()
}

// FR-16 constants for the E5 wave view: the facet tree renders as a
// bounds-scaled map whose labels are culled to the plot rect, de-collided by
// 64px buckets, and capped so a deep shell tree can never explode into
// overlapping text (A-11).
const (
	// waveLabelHeight is the minimum scaled node height that earns a label.
	waveLabelHeight float32 = 16
	// waveLabelCap is the maximum rendered labels per frame (AC-11).
	waveLabelCap = 24
	// waveBucketHeight is the vertical label de-collision bucket size.
	waveBucketHeight float32 = 64
)

// NewPropagationFacet builds the E5 facet over the dirty sink.
func NewPropagationFacet(sink *DirtySink, fonts *text.FontRegistry, themeCtx theme.ResolvedContext) *Propagation {
	e := &Propagation{
		sink:       sink,
		overlay:    diagnostics.NewOverlay(),
		paused:     store.NewValueStore(false),
		retention:  store.NewValueStore(10.0),
		dirtyCount: store.NewValueStore("0"),
		notLive:    store.NewValueStore(true),
	}
	e.Facet = facet.NewFacet()
	if fonts != nil {
		e.shaper = text.NewShaper(fonts)
	}
	e.textStyle = themeCtx.TextStyle(theme.TextLabelS)
	e.textColor = themeCtx.Color(theme.ColorText)
	e.dimColor = themeCtx.Color(theme.ColorTextSecondary)
	e.bg = themeCtx.Color(theme.ColorSurface)
	// FR-16 node map colors: subtle fills so the layout map reads without
	// competing with the dirty highlights.
	e.nodeFill = e.textColor
	e.nodeFill.A = 0.06
	e.nodeEdge = e.textColor
	e.nodeEdge.A = 0.28

	e.buildControls()
	e.AddChild(e.controls.Base())

	e.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke exhibit host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			content := facet.Constraints{MaxSize: gfx.Size{W: c.MaxSize.W}}
			if role := e.controls.Base().LayoutRole(); role != nil {
				role.Measure(ctx, content)
			}
			return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			controlsH := e.controls.Base().LayoutRole().MeasuredSize.H
			if controlsH < 1 {
				controlsH = 1
			}
			treeH := bounds.Height() - controlsH
			if treeH < 1 {
				treeH = 1
			}
			e.treeArea = gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), treeH)
			e.controls.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y+treeH, bounds.Width(), controlsH))
			// The attach-time capture runs before the first arrangement (its
			// bounds are empty). Re-capture once the shell has been laid out so
			// the bounds-scaled wave map has real geometry (FR-16). Read-only
			// inspector walk, runtime thread, no invalidation — no loop.
			if e.treeBoundsEmpty() {
				e.captureTree()
			}
		},
	}
	e.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(e.bg)})
			area := e.treeArea
			if area.IsEmpty() {
				area = bounds
			}
			list.Commands = append(list.Commands, e.treeCommands(area)...)
			list.Commands = append(list.Commands, e.edgeHonestyCommands(area)...)
			list.Commands = append(list.Commands, e.legendCommands(area)...)
		},
	}
	e.tick = facet.TickRole{OnTick: func(_ time.Duration) {
		if !e.active {
			return
		}
		e.captureTree()
		if e.sink != nil {
			if latest, ok := e.sink.Latest(); ok {
				e.dirtyCount.Set(strconv.Itoa(len(latest.Dirty)))
			}
			e.notLive.Set(e.sink.Paused() || !e.sink.Live())
		} else {
			e.notLive.Set(true)
		}
		e.Invalidate(facet.DirtyProjection)
	}}
	e.AddRole(&e.layout)
	e.AddRole(&e.render)
	e.AddRole(&e.tick)
	return e
}

// DirtyCount returns the badge's store (the latest dirty count).
func (e *Propagation) DirtyCount() *store.ValueStore[string] { return e.dirtyCount }

// NotLive returns the status light's store (true when the sink is not live).
func (e *Propagation) NotLive() *store.ValueStore[bool] { return e.notLive }

// EdgeNote returns the R-fake-viz honesty label shown for the parts that are
// not introspectable (dependency edges). It is the exhibit's entire edge view.
func (e *Propagation) EdgeNote() string { return edgeIntrospectionNote }

// TreeArea returns the wave view's plot rect (the tree region above the
// controls), valid after arrange.
func (e *Propagation) TreeArea() gfx.Rect { return e.treeArea }

func (e *Propagation) buildControls() {
	pauseSwitch := selection.NewSwitch("Pause capture", e.paused)
	retentionSlider := selection.NewSlider("Retention", 1, 30, 1, e.retention)
	e.badge = status.NewBadge("")
	e.badge.Label = marks.FromStore(e.dirtyCount, facet.DirtyProjection)
	e.light = status.NewStatusLight("sink")
	e.light.ShowLabel = marks.Const(false)
	e.light.Disabled = marks.FromStore(e.notLive, facet.DirtyProjection)

	e.controls = structure.NewCard("Capture")
	e.controls.GridColumns = marks.Const(2)
	e.controls.GridRows = marks.Const(2)
	e.controls.ChildrenContent = []structure.CardChild{
		{Key: "pause", Facet: pauseSwitch, Grid: facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1}},
		{Key: "retention", Facet: retentionSlider, Grid: facet.GridPlacement{ColStart: 1, RowStart: 0, ColSpan: 1, RowSpan: 1}},
		{Key: "dirty", Facet: e.badge, Grid: facet.GridPlacement{ColStart: 0, RowStart: 1, ColSpan: 1, RowSpan: 1}},
		{Key: "live", Facet: e.light, Grid: facet.GridPlacement{ColStart: 1, RowStart: 1, ColSpan: 1, RowSpan: 1}},
	}
}

// captureTree snapshots the shell facet tree via the runtime inspector. It
// runs on the runtime thread (the tick), so the cached tree is stable when the
// (possibly forked) projection reads it.
func (e *Propagation) captureTree() {
	rt, ok := e.rt.(*runtime.Runtime)
	if !ok || rt == nil {
		return
	}
	nodes := make([]propagationNode, 0, 64)
	rt.Inspect(func(inspector *diagnostics.Inspector) {
		inspector.Walk(func(depth int, info diagnostics.FacetInfo) {
			nodes = append(nodes, propagationNode{
				id:     info.ID,
				depth:  depth,
				label:  info.TypeName,
				bounds: info.ArrangedBounds,
			})
		})
	})
	e.tree = nodes
}

// treeCommands draws the captured shell tree as a bounds-scaled layout map
// (FR-16): every node renders as a rectangle sized by its arranged bounds,
// dirty nodes are highlighted through the framework overlay, and labels are
// culled to the plot rect, de-collided by 64px vertical buckets, and capped at
// waveLabelCap so a deep tree can never produce overlapping text (A-11).
func (e *Propagation) treeCommands(area gfx.Rect) []gfx.Command {
	if e.sink == nil || len(e.tree) == 0 {
		return nil
	}
	dirty := e.dirtyUnion()
	shell := e.shellExtent()
	if shell.IsEmpty() {
		return nil
	}
	scale := e.mapScale(area, shell)
	base := e.mapOffset(area, shell, scale)

	var cmds []gfx.Command
	var scratch gfx.CommandList
	taken := make(map[int]bool, 16)
	labels := 0
	for _, n := range e.tree {
		r := e.nodeRect(base, scale, n.bounds)
		if r.Width() < 2 || r.Height() < 2 {
			continue
		}
		// Plot-rect culling: nodes fully outside the wave area are skipped
		// (left/right/top/bottom), so nothing renders off the map.
		if r.Max.X <= area.Min.X || r.Min.X >= area.Max.X ||
			r.Max.Y <= area.Min.Y || r.Min.Y >= area.Max.Y {
			continue
		}
		// The node rectangle is bounds-derived (FR-16).
		cmds = append(cmds, gfx.FillRect{Rect: r, Brush: gfx.SolidBrush(e.nodeFill)})
		cmds = append(cmds, gfx.StrokeRect{Rect: r, Stroke: gfx.StrokeStyle{Width: 1}, Brush: gfx.SolidBrush(e.nodeEdge)})
		// The dirty highlight is drawn by the framework's diagnostics.Overlay
		// (F-overlay-precedent), not a parallel renderer.
		if info, isDirty := dirty[n.id]; isDirty {
			e.overlay.HighlightDirty(&scratch, r, info.flags)
			cmds = append(cmds, scratch.Commands...)
			scratch.Commands = nil
		}
		// Labels: only nodes tall enough to read one, capped and de-collided.
		if r.Height() < waveLabelHeight || labels >= waveLabelCap {
			continue
		}
		bucket := int(r.Min.Y / waveBucketHeight)
		if taken[bucket] {
			continue
		}
		taken[bucket] = true
		labels++
		if cmd := e.waveLabel(r, e.nodeLabel(n, dirty), e.textColor, area); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

// nodeLabel is the label text for a node: its type name plus, when dirty, the
// flag category and the recorded invalidation source (the store-bound marks
// invalidate without a recorded runtime source, so the category is the floor).
func (e *Propagation) nodeLabel(n propagationNode, dirty map[facet.FacetID]dirtyInfo) string {
	label := n.label
	if label == "" {
		label = "facet"
	}
	if info, isDirty := dirty[n.id]; isDirty {
		cat := e.flagName(info.flags)
		if info.source != "" {
			cat = cat + " · " + info.source
		}
		label = label + " [" + cat + "]"
	}
	return label
}

// dirtyUnion merges the retained snapshots into the latest dirty state per
// facet (latest snapshot wins).
func (e *Propagation) dirtyUnion() map[facet.FacetID]dirtyInfo {
	snaps := e.sink.Snapshots()
	dirty := make(map[facet.FacetID]dirtyInfo)
	for i := len(snaps) - 1; i >= 0; i-- {
		for id, flags := range snaps[i].Dirty {
			if _, ok := dirty[id]; ok {
				continue
			}
			dirty[id] = dirtyInfo{flags: flags, source: snaps[i].Sources[id], frame: snaps[i].FrameNumber}
		}
	}
	return dirty
}

// treeBoundsEmpty reports whether the captured tree has any arranged geometry
// (the attach-time capture runs before the first layout).
func (e *Propagation) treeBoundsEmpty() bool {
	if len(e.tree) == 0 {
		return true
	}
	for _, n := range e.tree {
		if !n.bounds.IsEmpty() {
			return false
		}
	}
	return true
}

// shellExtent is the union of all captured node bounds (the shell's extent).
func (e *Propagation) shellExtent() gfx.Rect {
	var out gfx.Rect
	for _, n := range e.tree {
		if n.bounds.IsEmpty() {
			continue
		}
		if out.IsEmpty() {
			out = n.bounds
			continue
		}
		out = out.Union(n.bounds)
	}
	return out
}

// mapScale is the uniform scale that fits the shell extent into the wave area
// (bounds-scaled map, FR-16), clamped so degenerate shells cannot collapse or
// blow up the rectangles.
func (e *Propagation) mapScale(area, shell gfx.Rect) float32 {
	if shell.Width() <= 0 || shell.Height() <= 0 {
		return 1
	}
	const pad float32 = 8
	scale := (area.Width() - 2*pad) / shell.Width()
	if h := (area.Height() - 2*pad) / shell.Height(); h < scale {
		scale = h
	}
	if scale < 0.1 {
		scale = 0.1
	}
	if scale > 1.5 {
		scale = 1.5
	}
	return scale
}

// mapOffset is the translate that centers the scaled shell in the wave area.
func (e *Propagation) mapOffset(area, shell gfx.Rect, scale float32) gfx.Point {
	return gfx.Point{
		X: area.Min.X + (area.Width()-shell.Width()*scale)*0.5 - shell.Min.X*scale,
		Y: area.Min.Y + (area.Height()-shell.Height()*scale)*0.5 - shell.Min.Y*scale,
	}
}

// nodeRect maps a captured node's arranged bounds into the wave area.
func (e *Propagation) nodeRect(base gfx.Point, scale float32, b gfx.Rect) gfx.Rect {
	return gfx.RectFromXYWH(base.X+b.Min.X*scale, base.Y+b.Min.Y*scale, b.Width()*scale, b.Height()*scale)
}

// waveLabel builds a label glyph for a node rect, clamped so the shaped glyph
// stays inside the plot rect (FR-16: no label outside the wave area).
func (e *Propagation) waveLabel(r gfx.Rect, label string, color gfx.Color, area gfx.Rect) gfx.Command {
	if e.shaper == nil || label == "" {
		return nil
	}
	shaped := e.shaper.ShapeSimple(label, e.textStyle)
	if shaped == nil || len(shaped.Lines) == 0 || len(shaped.Lines[0].Runs) == 0 {
		return nil
	}
	run := shaped.Lines[0].Runs[0]
	w := run.Bounds.Width()
	x := r.Min.X + 2
	if x < area.Min.X {
		x = area.Min.X
	}
	if x+w > area.Max.X {
		x = area.Max.X - w
	}
	if x < area.Min.X {
		return nil
	}
	y := r.Min.Y + 2
	if y < area.Min.Y {
		y = area.Min.Y
	}
	if y > area.Max.Y {
		return nil
	}
	return gfx.DrawGlyphRun{Run: run, Origin: gfx.Point{X: x, Y: y + 11}, Brush: gfx.SolidBrush(color)}
}

func (e *Propagation) flagName(flags facet.DirtyFlags) string {
	switch {
	case flags&facet.DirtyLayout != 0:
		return "Layout"
	case flags&facet.DirtyProjection != 0:
		return "Projection"
	case flags&facet.DirtyHit != 0:
		return "Hit"
	default:
		return "Dirty"
	}
}

func (e *Propagation) edgeHonestyCommands(area gfx.Rect) []gfx.Command {
	// R-fake-viz: store-level causal provenance and dependency edges are not
	// introspectable (F-edges). This note is drawn, never a fabricated edge.
	return []gfx.Command{e.glyphCommand(area.Min.X, area.Max.Y-14, edgeIntrospectionNote, e.dimColor)}
}

func (e *Propagation) legendCommands(area gfx.Rect) []gfx.Command {
	var list gfx.CommandList
	y := area.Min.Y + 2
	// The legend swatches use the same Overlay dirty-highlight drawing as the
	// tree rows, so the flag→color mapping is a single source.
	swatch := func(x float32, flags facet.DirtyFlags) {
		e.overlay.HighlightDirty(&list, gfx.RectFromXYWH(area.Min.X+x, y, 8, 10), flags)
	}
	swatch(0, facet.DirtyLayout)
	list.Add(e.glyphCommand(area.Min.X+12, y, "Layout", e.textColor))
	swatch(60, facet.DirtyProjection)
	list.Add(e.glyphCommand(area.Min.X+72, y, "Projection", e.textColor))
	swatch(160, facet.DirtyHit)
	list.Add(e.glyphCommand(area.Min.X+172, y, "Hit", e.textColor))
	return list.Commands
}

func (e *Propagation) glyphCommand(x, y float32, label string, color gfx.Color) gfx.Command {
	if e.shaper == nil || label == "" {
		return nil
	}
	shaped := e.shaper.ShapeSimple(label, e.textStyle)
	if shaped == nil || len(shaped.Lines) == 0 || len(shaped.Lines[0].Runs) == 0 {
		return nil
	}
	return gfx.DrawGlyphRun{
		Run:    shaped.Lines[0].Runs[0],
		Origin: gfx.Point{X: x, Y: y + 11},
		Brush:  gfx.SolidBrush(color),
	}
}

func (e *Propagation) OnAttach(ctx facet.AttachContext) {
	e.rt = ctx.Runtime
	if e.sink != nil {
		e.sink.SetPaused(e.paused.Get())
		e.sink.SetRetention(int(e.retention.Get()))
	}
	e.captureTree()
	idPaused := e.paused.OnChange.Subscribe(func(signal.Change[bool]) {
		if e.sink != nil {
			e.sink.SetPaused(e.paused.Get())
		}
	})
	idRet := e.retention.OnChange.Subscribe(func(signal.Change[float64]) {
		if e.sink != nil {
			e.sink.SetRetention(int(e.retention.Get()))
		}
	})
	e.cleanup = func() {
		e.paused.OnChange.Unsubscribe(idPaused)
		e.retention.OnChange.Unsubscribe(idRet)
	}
}

func (e *Propagation) OnDetach() {
	if e.cleanup != nil {
		e.cleanup()
		e.cleanup = nil
	}
}

func (e *Propagation) Base() *facet.Facet { e.BindImpl(e); return &e.Facet }
func (e *Propagation) OnActivate()        { e.active = true }
func (e *Propagation) OnDeactivate()      { e.active = false }
