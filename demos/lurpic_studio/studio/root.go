package studio

import (
	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/layout/linear"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/platform"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Palette returns the command palette mark.
func (r *Root) Palette() *action.CommandPalette { return r.palette }

// Commands returns the shell command registry.
func (r *Root) Commands() *runtimepkg.CommandRegistry { return r.registry }

// LayoutMode selects the shell's responsive arrangement (FR-resp).
type LayoutMode uint8

const (
	// LayoutWide is the 3-pane split: index | stage | inspector.
	LayoutWide LayoutMode = iota
	// LayoutNarrow collapses to a full-width stage with overlay re-hosts.
	LayoutNarrow
)

// breakpointWide is the minimum width (in dp) for the wide layout; narrower
// widths collapse to the narrow layout. Content-scale aware via ModeFor.
const breakpointWide = 960

// ModeFor returns the shell layout mode for a window width in device pixels,
// compared against the breakpoint scaled by content scale (dp).
func ModeFor(width, contentScale float32) LayoutMode {
	bp := breakpointWide * contentScale
	if bp < 1 {
		bp = breakpointWide
	}
	if width >= bp {
		return LayoutWide
	}
	return LayoutNarrow
}

// Root is the gallery shell: the chrome bar, the responsive gallery (3-pane
// split in wide mode; full-width stage + overlay re-hosts in narrow mode), the
// wired status bar, the command palette overlay, and the shared shell state
// every sub-tree binds. It is the first production consumer of the
// layout/linear policy (§1.6, GroupLayoutLinearVertical).
type Root struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	hit    facet.HitRole
	input  facet.InputRole
	focus  facet.FocusRole

	shell     *ShellState
	chrome    *ChromeStack
	gallery   *GallerySplit
	index     *ExhibitIndex
	inspector *ExhibitInspector
	narrow    *NarrowShell
	stage     *Stage
	status    *StatusBar
	palette   *action.CommandPalette
	registry  *runtimepkg.CommandRegistry

	layoutMode LayoutMode
	gap        float32

	rt      facet.RuntimeServices
	cleanup func()
}

// NewRoot builds the gallery shell for a resolved app context. The dirty sink
// is shared with the E5 exhibit (nil disables E5's live capture but keeps the
// exhibit renderable). The seed rows populate the shared AppState (the app
// loads the metrics.csv snapshot and passes it down; the feed reshapes it).
// The layer registry carries the studio custom layers (E2 hit policies, E3
// anchored recipe).
func NewRoot(ctx app.BuildContext, sink *DirtySink, seed []dataset.Row, reg *layout.LayerRegistry) *Root {
	appState := state.NewAppState(seed)
	shell := NewShellState(appState)
	ids := studioLayersFrom(reg)

	stage := NewStage([]Exhibit{
		NewRealtimeData(ctx.FontRegistry, ctx.Theme),
		NewLayoutPolicies(),
		NewLayersData(ctx.FontRegistry, ctx.Theme, ids),
		NewAnchorsData(ctx.FontRegistry, ctx.Theme, ids),
		NewPropagationData(sink, ctx.FontRegistry, ctx.Theme),
		NewPlayground(appState),
		NewCapabilityIndexData(),
	}, appState, shell.ActiveExhibit)

	var feed *Feed
	if e1, ok := stage.RootFor(ExhibitRealtime).(*Realtime); ok {
		feed = e1.Feed()
	}
	counts := perExhibitMarkCounts(stage)

	r := &Root{
		shell:      shell,
		chrome:     NewChromeStack(ctx.Theme, shell),
		index:      NewExhibitIndex(shell),
		inspector:  NewExhibitInspector(shell, counts),
		narrow:     NewNarrowShell(shell, counts),
		stage:      stage,
		status:     NewStatusBar(ctx.Theme, shell, feed, exhibitTitle),
		layoutMode: LayoutWide,
		gap:        0,
	}
	r.registry, r.palette = ShellCommands(shell, r.toggleLive)

	r.gallery = NewGallerySplit([]Pane{
		{Facet: r.index, FixedWidth: indexPaneWidth, MinWidth: indexPaneWidth},
		{Facet: stage, Flex: stagePaneFlex, MinWidth: stagePaneMinWidth},
		{Facet: r.inspector, FixedWidth: inspectorPaneWidth, MinWidth: inspectorPaneWidth},
	}, dividerSize)

	r.Facet = facet.NewFacet()
	r.AddChild(r.chrome.Base())  //lurpiclint:ignore LL021 -- the shell hosts chrome as a regular child, not an overlay (LL021 over-fires on any field ref)
	r.AddChild(r.gallery.Base()) //lurpiclint:ignore LL021 -- the shell hosts the gallery as a regular child, not an overlay (LL021 over-fires on any field ref)
	r.AddChild(r.narrow.Base())  //lurpiclint:ignore LL021 -- the narrow overlay sub-tree is a Root child gated by the host (LL021 over-fires on overlays hosted as regular children)
	r.AddChild(r.status.Base())  //lurpiclint:ignore LL021 -- the shell hosts the status bar as a regular child, not an overlay (LL021 over-fires on any field ref)
	r.AddChild(r.palette.Base()) //lurpiclint:ignore LL021 -- the command palette self-mounts its layered surface; the Root hosts the palette facet itself (LL021 over-fires)

	r.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke linear group-parent host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return r.measure(ctx, c)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			r.arrange(ctx, bounds)
		},
	}
	r.layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   groupPolicy{kind: facet.GroupLayoutLinearVertical, host: r},
		Children: r,
	}
	// The runtime arranges the app root directly with the default grid
	// placement, so Root's child contract must declare SupportsGrid alongside
	// the linear placement its shell children use.
	r.layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsLinear,
		Stretch: facet.StretchPolicy{
			Width:  facet.StretchNever,
			Height: facet.StretchNever,
		},
	}
	r.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(ctx.Theme.Color(theme.ColorBackground))})
		},
	}
	// The Root claims the window background so a click on empty shell space
	// focuses the shell (focus manager's requestFocus walks the hit path),
	// which is what routes Ctrl+K to the Root's key handler.
	r.hit = facet.HitRole{
		OnHitTest: func(p gfx.Point) facet.HitResult {
			if r.layout.ArrangedBounds.IsEmpty() {
				return facet.HitResult{}
			}
			if r.layout.ArrangedBounds.Contains(p) {
				return facet.HitResult{Hit: true}
			}
			return facet.HitResult{}
		},
	}
	r.input = facet.InputRole{
		OnKey: func(e facet.KeyEvent) bool {
			if e.Kind == platform.KeyPress && e.Key == platform.KeyK && e.Modifiers&platform.ModControl != 0 {
				if !r.shell.CommandOpen.Get() {
					r.shell.CommandOpen.Set(true)
				}
				return true
			}
			return false
		},
	}
	r.focus = facet.FocusRole{
		Focusable: func() bool { return true },
		TabIndex:  0,
	}
	r.AddRole(&r.layout)
	r.AddRole(&r.render)
	r.AddRole(&r.hit)
	r.AddRole(&r.input)
	r.AddRole(&r.focus)
	return r
}

// BuildRoot constructs the app root facet (the rootBuilder in main.go).
func BuildRoot(ctx app.BuildContext, sink *DirtySink, seed []dataset.Row, reg *layout.LayerRegistry) facet.FacetImpl {
	return NewRoot(ctx, sink, seed, reg)
}

// ChromeStack returns the top chrome bar.
func (r *Root) ChromeStack() *ChromeStack { return r.chrome }

// GallerySplit returns the gallery split host.
func (r *Root) GallerySplit() *GallerySplit { return r.gallery }

// StatusBar returns the wired status bar.
func (r *Root) StatusBar() *StatusBar { return r.status }

// Stage returns the exhibit stage.
func (r *Root) Stage() *Stage { return r.stage }

// Index returns the wide-mode exhibit index pane.
func (r *Root) Index() *ExhibitIndex { return r.index }

// Inspector returns the wide-mode inspector pane.
func (r *Root) Inspector() *ExhibitInspector { return r.inspector }

// Narrow returns the narrow-mode overlay sub-tree.
func (r *Root) Narrow() *NarrowShell { return r.narrow }

// Shell returns the shared shell state.
func (r *Root) Shell() *ShellState { return r.shell }

// LayoutMode returns the currently active responsive mode.
func (r *Root) LayoutMode() LayoutMode { return r.layoutMode }

func (r *Root) measure(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
	r.applyMode(ModeFor(c.MaxSize.W, ctx.ContentScale))

	children := []linear.Child{
		linearChild(ctx, c.MaxSize, r.chrome, crossStretch(0)),
		linearChild(ctx, c.MaxSize, r.gallery, facet.LinearPlacement{
			Order:          1,
			CrossAxisAlign: facet.CrossAxisStretch,
			MainAxisSize:   facet.MainAxisMax,
		}),
		linearChild(ctx, c.MaxSize, r.status, crossStretch(2)),
	}
	policy := linear.New(linear.Config{Axis: linear.Vertical, Gap: r.gap})
	size, err := policy.Measure(children, c.MaxSize)
	if err != nil {
		size = c.MaxSize
	}
	// The narrow overlay sub-tree participates in narrow mode only.
	if r.layoutMode == LayoutNarrow {
		if role := r.narrow.Base().LayoutRole(); role != nil {
			role.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
		}
	}
	return facet.MeasureResult{Size: size}
}

func (r *Root) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if bounds.IsEmpty() {
		return
	}
	children := []linear.Child{
		linearChildOf(r.chrome, crossStretch(0)),
		linearChildOf(r.gallery, facet.LinearPlacement{
			Order:          1,
			CrossAxisAlign: facet.CrossAxisStretch,
			MainAxisSize:   facet.MainAxisMax,
		}),
		linearChildOf(r.status, crossStretch(2)),
	}
	policy := linear.New(linear.Config{Axis: linear.Vertical, Gap: r.gap})
	_, _ = policy.Arrange(children, bounds)

	// The narrow overlay sub-tree only participates in narrow mode.
	if r.layoutMode == LayoutNarrow {
		statusH := r.status.Base().LayoutRole().ArrangedBounds.Height()
		chromeH := r.chrome.Base().LayoutRole().ArrangedBounds.Height()
		body := gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y+chromeH+r.gap, bounds.Width(), bounds.Height()-chromeH-statusH-2*r.gap)
		r.narrow.Base().LayoutRole().Arrange(ctx, body)
	} else {
		r.narrow.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
	}
}

// applyMode switches the gallery's pane list when the responsive mode changes:
// wide shows [index | stage | inspector], narrow shows the stage full-width
// with the index/inspector re-hosted as overlays (FR-resp).
func (r *Root) applyMode(mode LayoutMode) {
	if mode == r.layoutMode {
		return
	}
	r.layoutMode = mode
	r.shell.Mode = mode
	var panes []Pane
	if mode == LayoutWide {
		panes = []Pane{
			{Facet: r.index, FixedWidth: indexPaneWidth, MinWidth: indexPaneWidth},
			{Facet: r.stage, Flex: stagePaneFlex, MinWidth: stagePaneMinWidth},
			{Facet: r.inspector, FixedWidth: inspectorPaneWidth, MinWidth: inspectorPaneWidth},
		}
	} else {
		panes = []Pane{
			{Facet: r.stage, Flex: 1, MinWidth: stagePaneMinWidth},
		}
	}
	r.gallery.SetPanes(panes)
	invalidateLayout(r, r.rt, "root.applyMode")
}

// toggleLive flips the E1 feed's Live gate (a registered command).
func (r *Root) toggleLive() {
	if e1, ok := r.stage.RootFor(ExhibitRealtime).(*Realtime); ok {
		live := e1.Feed().Live()
		live.Set(!live.Get())
	}
}

// Children returns the shell's immediate group children (the group-parent
// bridge's ChildSource). The narrow overlay sub-tree is deliberately excluded:
// it is arranged by the Root as an overlay, not a member of the vertical flow.
func (r *Root) Children() []facet.GroupChild {
	return []facet.GroupChild{
		linearGroupChild(crossStretch(0), r.chrome),
		linearGroupChild(facet.LinearPlacement{Order: 1, CrossAxisAlign: facet.CrossAxisStretch, MainAxisSize: facet.MainAxisMax}, r.gallery),
		linearGroupChild(crossStretch(2), r.status),
	}
}

// perExhibitMarkCounts walks each exhibit's root facet tree once and counts its
// reachable marks (the inspector + coverage audit share the result).
func perExhibitMarkCounts(stage *Stage) map[ExhibitID]int {
	counts := make(map[ExhibitID]int, len(exhibitCatalog))
	for _, e := range exhibitCatalog {
		if root := stage.RootFor(e.id); root != nil {
			counts[e.id] = len(walkMarkDescriptors(root))
		}
	}
	return counts
}

func (r *Root) OnAttach(ctx facet.AttachContext) {
	r.rt = ctx.Runtime
	// Keep the shell's Connection reflecting the E1 feed's live gate.
	liveID := signal.SubscriptionID(0)
	if e1, ok := r.stage.RootFor(ExhibitRealtime).(*Realtime); ok {
		live := e1.Feed().Live()
		r.shell.Connection.Set(live.Get())
		liveID = live.OnChange.Subscribe(func(c signal.Change[bool]) {
			r.shell.Connection.Set(c.New)
		})
	}
	r.cleanup = func() {
		if liveID != 0 {
			if e1, ok := r.stage.RootFor(ExhibitRealtime).(*Realtime); ok {
				e1.Feed().Live().OnChange.Unsubscribe(liveID)
			}
		}
	}
}

func (r *Root) OnDetach() {
	if r.cleanup != nil {
		r.cleanup()
		r.cleanup = nil
	}
}

func (r *Root) OnActivate()   {}
func (r *Root) OnDeactivate() {}

func (r *Root) Base() *facet.Facet { r.BindImpl(r); return &r.Facet }
