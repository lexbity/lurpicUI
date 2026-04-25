package ui

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_replay/engine"
	"codeburg.org/lexbit/ui_replay/model"
	"codeburg.org/lexbit/ui_replay/store"
)

// ReplayRootFacet is the root facet managing the replay shell layout.
type ReplayRootFacet struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	th     theme.Context
	shaper *text.Shaper
	meta   model.BuildMetadata

	header    *HeaderFacet
	sidebar   *SidebarFacet
	content   *ContentFacet
	inspector *InspectorFacet
	footer    *FooterFacet

	adder          facetChildAdder
	frameRequester interface{ RequestFrame() }

	shellBounds    ShellBounds
	responsive     structure.ResponsiveLayout
	headerPanel    structure.PanelToggleState
	sidebarPanel   structure.PanelToggleState
	contentPanel   structure.PanelToggleState
	inspectorPanel structure.PanelToggleState
	footerPanel    structure.PanelToggleState
	mobilePanel    string

	hasError bool
	errorMsg string

	// Execution control
	runner      *engine.Runner
	runCallback func(*model.RunResult)
}

// NewReplayRootFacet creates the root facet with all child facets.
func NewReplayRootFacet(th theme.Context, shaper *text.Shaper, meta model.BuildMetadata) *ReplayRootFacet {
	root := &ReplayRootFacet{
		Facet:  facet.NewFacet(),
		th:     th,
		shaper: shaper,
		meta:   meta,
	}

	root.header = NewHeaderFacet(th, shaper, meta)
	root.sidebar = NewSidebarFacet(th, shaper)
	root.content = NewContentFacet(th, shaper)
	root.inspector = NewInspectorFacet(th, shaper)
	root.footer = NewFooterFacet(th, shaper)

	root.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		w := c.MaxSize.W
		if w <= 0 {
			w = c.MinSize.W
		}
		if w <= 0 {
			w = 1
		}
		h := c.MaxSize.H
		if h <= 0 {
			h = c.MinSize.H
		}
		if h <= 0 {
			h = 1
		}
		return gfx.Size{W: w, H: h}
	}

	root.layout.OnArrange = func(bounds gfx.Rect) {
		root.layout.ArrangedBounds = bounds
		root.syncResponsiveLayout(bounds)
		root.arrangeShell(bounds)
	}
	root.AddRole(&root.layout)

	root.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}

		defer func() {
			if r := recover(); r != nil {
				root.hasError = true
				root.errorMsg = fmt.Sprintf("Render panic: %v\n%s", r, debug.Stack())
				list.Add(gfx.FillRect{
					Rect:  bounds,
					Brush: gfx.SolidBrush(root.th.Color(theme.ColorBackground)),
				})
				root.renderErrorState(list, bounds)
			}
		}()

		if root.hasError {
			list.Add(gfx.FillRect{
				Rect:  bounds,
				Brush: gfx.SolidBrush(root.th.Color(theme.ColorBackground)),
			})
			root.renderErrorState(list, bounds)
			return
		}

		list.Add(gfx.FillRect{
			Rect:  bounds,
			Brush: gfx.SolidBrush(root.th.Color(theme.ColorBackground)),
		})
	}
	root.AddRole(&root.render)

	return root
}

// Base returns the base facet.
func (f *ReplayRootFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

// OnAttach attaches child facets.
func (f *ReplayRootFacet) OnAttach(ctx facet.AttachContext) {
	if adder, ok := ctx.Runtime.(facetChildAdder); ok {
		f.adder = adder
	}
	if req, ok := ctx.Runtime.(interface{ RequestFrame() }); ok {
		f.frameRequester = req
	}

	attachChild(&f.Facet, f, f.header, f.adder, layout.ChildAttachment{LayerID: 1})
	attachChild(&f.Facet, f, f.sidebar, f.adder, layout.ChildAttachment{LayerID: 2})
	attachChild(&f.Facet, f, f.content, f.adder, layout.ChildAttachment{LayerID: 3})
	attachChild(&f.Facet, f, f.inspector, f.adder, layout.ChildAttachment{LayerID: 4})
	attachChild(&f.Facet, f, f.footer, f.adder, layout.ChildAttachment{LayerID: 5})

	if f.header != nil {
		f.header.OnToggleScenarios.Subscribe(func(_ struct{}) { f.setMobilePanel("scenarios") })
		f.header.OnToggleDetails.Subscribe(func(_ struct{}) { f.setMobilePanel("details") })
		f.header.OnToggleArtifacts.Subscribe(func(_ struct{}) { f.setMobilePanel("artifacts") })
		f.header.OnRun.Subscribe(func(_ struct{}) {
			if f.runner != nil && store.ExecutionStateStore.Get().CanStart() {
				_ = f.ReloadScenario()
			}
		})
		f.header.OnCancel.Subscribe(func(_ struct{}) {
			_ = f.CancelRun()
		})
		f.header.OnExport.Subscribe(func(_ struct{}) {
			_ = f.ExportBundle("")
		})
	}
}

// OnDetach handles detachment.
func (f *ReplayRootFacet) OnDetach() {}

// OnActivate handles activation.
func (f *ReplayRootFacet) OnActivate() {}

// OnDeactivate handles deactivation.
func (f *ReplayRootFacet) OnDeactivate() {}

// OnLayerSpecs returns layer specifications for child facets.
func (f *ReplayRootFacet) OnLayerSpecs() []layout.LayerSpec {
	bounds := f.layout.ArrangedBounds
	if bounds.IsEmpty() {
		return nil
	}

	shell := f.shellBounds
	if shell.Header.IsEmpty() {
		shell = f.currentShellBounds(bounds)
	}

	return []layout.LayerSpec{
		{
			ID:          1,
			Placement:   layout.PlacementFree,
			Measurement: layout.MeasureNonStructural,
			CoordSpace:  layout.CoordParentLayout,
			CoordLimits: layout.CoordLimits{Bounds: shell.Header},
			HitPolicy:   layout.HitNormal,
			RenderOrder: 0,
			ClipPolicy:  layout.ClipToParent,
		},
		{
			ID:          2,
			Placement:   layout.PlacementFree,
			Measurement: layout.MeasureNonStructural,
			CoordSpace:  layout.CoordParentLayout,
			CoordLimits: layout.CoordLimits{Bounds: shell.Sidebar},
			HitPolicy:   layout.HitNormal,
			RenderOrder: 1,
			ClipPolicy:  layout.ClipToParent,
		},
		{
			ID:          3,
			Placement:   layout.PlacementFree,
			Measurement: layout.MeasureNonStructural,
			CoordSpace:  layout.CoordParentLayout,
			CoordLimits: layout.CoordLimits{Bounds: shell.Content},
			HitPolicy:   layout.HitNormal,
			RenderOrder: 2,
			ClipPolicy:  layout.ClipToParent,
		},
		{
			ID:          4,
			Placement:   layout.PlacementFree,
			Measurement: layout.MeasureNonStructural,
			CoordSpace:  layout.CoordParentLayout,
			CoordLimits: layout.CoordLimits{Bounds: shell.Inspector},
			HitPolicy:   layout.HitNormal,
			RenderOrder: 3,
			ClipPolicy:  layout.ClipToParent,
		},
		{
			ID:          5,
			Placement:   layout.PlacementFree,
			Measurement: layout.MeasureNonStructural,
			CoordSpace:  layout.CoordParentLayout,
			CoordLimits: layout.CoordLimits{Bounds: shell.Footer},
			HitPolicy:   layout.HitNormal,
			RenderOrder: 4,
			ClipPolicy:  layout.ClipToParent,
		},
	}
}

// arrangeShell updates all child facet bounds.
func (f *ReplayRootFacet) arrangeShell(bounds gfx.Rect) {
	if f == nil {
		return
	}

	f.shellBounds = f.currentShellBounds(bounds)

	if f.header != nil {
		f.header.layout.Arrange(f.shellBounds.Header)
	}
	if f.sidebar != nil {
		f.sidebar.layout.Arrange(f.shellBounds.Sidebar)
	}
	if f.content != nil {
		f.content.layout.Arrange(f.shellBounds.Content)
	}
	if f.inspector != nil {
		f.inspector.layout.Arrange(f.shellBounds.Inspector)
	}
	if f.footer != nil {
		f.footer.layout.Arrange(f.shellBounds.Footer)
	}
}

func (f *ReplayRootFacet) currentShellBounds(bounds gfx.Rect) ShellBounds {
	if f == nil {
		return ShellBounds{}
	}
	if f.responsive.Variant == "" {
		f.syncResponsiveLayout(bounds)
	}
	switch f.responsive.Variant {
	case structure.ShellVariantMobilePortrait:
		headerH := float32(headerHeight)
		header := gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), headerH)
		remaining := gfx.RectFromXYWH(bounds.Min.X, header.Max.Y, bounds.Width(), bounds.Max.Y-header.Max.Y)
		switch f.mobilePanel {
		case "details":
			return ShellBounds{
				Header:    header,
				Content:   remaining,
				Sidebar:   gfx.Rect{},
				Inspector: gfx.Rect{},
				Footer:    gfx.Rect{},
			}
		case "artifacts":
			footerH := float32(footerHeight)
			inspectorH := remaining.Height() - footerH - 8
			if inspectorH < 0 {
				inspectorH = 0
			}
			inspector := gfx.RectFromXYWH(bounds.Min.X, header.Max.Y, bounds.Width(), inspectorH)
			footer := gfx.RectFromXYWH(bounds.Min.X, inspector.Max.Y+8, bounds.Width(), remaining.Max.Y-inspector.Max.Y-8)
			return ShellBounds{
				Header:    header,
				Content:   gfx.Rect{},
				Sidebar:   gfx.Rect{},
				Inspector: inspector,
				Footer:    footer,
			}
		default:
			return ShellBounds{
				Header:  header,
				Sidebar: remaining,
			}
		}
	default:
		return CalculateShellBounds(bounds)
	}
}

func (f *ReplayRootFacet) syncResponsiveLayout(bounds gfx.Rect) {
	if f == nil {
		return
	}
	caps := structure.Capabilities{
		Touch:    runtime.GOOS == "android" || bounds.Width() <= 900,
		Hover:    runtime.GOOS != "android",
		Keyboard: true,
		IME:      runtime.GOOS == "android",
	}
	f.responsive = structure.ResponsiveLayoutForViewport(structure.Viewport{Width: int(bounds.Width()), Height: int(bounds.Height())}, caps)
	if f.responsive.Variant == structure.ShellVariantMobilePortrait {
		if f.mobilePanel == "" {
			f.mobilePanel = "scenarios"
		}
		switch f.mobilePanel {
		case "details":
			f.headerPanel.Expand()
			f.sidebarPanel.Collapse()
			f.contentPanel.Expand()
			f.inspectorPanel.Collapse()
			f.footerPanel.Collapse()
		case "artifacts":
			f.headerPanel.Expand()
			f.sidebarPanel.Collapse()
			f.contentPanel.Collapse()
			f.inspectorPanel.Expand()
			f.footerPanel.Expand()
		default:
			f.mobilePanel = "scenarios"
			f.headerPanel.Expand()
			f.sidebarPanel.Expand()
			f.contentPanel.Collapse()
			f.inspectorPanel.Collapse()
			f.footerPanel.Collapse()
		}
		return
	}
	f.headerPanel.Expand()
	f.sidebarPanel.Expand()
	f.contentPanel.Expand()
	f.inspectorPanel.Expand()
	f.footerPanel.Expand()
}

// RequestFrame requests a new frame render.
func (f *ReplayRootFacet) RequestFrame() {
	if f.frameRequester != nil {
		f.frameRequester.RequestFrame()
	}
}

func (f *ReplayRootFacet) setMobilePanel(mode string) {
	if f == nil {
		return
	}
	if mode == "" {
		mode = "scenarios"
	}
	f.mobilePanel = mode
	if !f.layout.ArrangedBounds.IsEmpty() {
		f.syncResponsiveLayout(f.layout.ArrangedBounds)
		f.arrangeShell(f.layout.ArrangedBounds)
	}
	f.RequestFrame()
}

// renderErrorState draws an error overlay when a panic occurs
func (f *ReplayRootFacet) renderErrorState(list *gfx.CommandList, bounds gfx.Rect) {
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(gfx.Color{R: 0.9, G: 0.3, B: 0.3, A: 0.9}),
	})

	if f.shaper != nil {
		titleStyle := f.th.TextStyle(theme.TextHeadingS)
		titleLayout := f.shaper.ShapeSimple("Application Error", titleStyle)
		if titleLayout != nil && len(titleLayout.Lines) > 0 {
			line := titleLayout.Lines[0]
			x := bounds.Min.X + (bounds.Width()-line.Bounds.Width())/2
			y := bounds.Min.Y + bounds.Height()/3
			origin := gfx.Point{X: x, Y: y}
			for _, run := range line.Runs {
				list.Add(gfx.DrawGlyphRun{
					Run:    run,
					Origin: origin,
					Brush:  gfx.SolidBrush(f.th.Color(theme.ColorText)),
				})
			}
		}

		msgStyle := f.th.TextStyle(theme.TextBodyS)
		msgLayout := f.shaper.ShapeSimple(f.errorMsg, msgStyle)
		if msgLayout != nil && len(msgLayout.Lines) > 0 {
			for i, line := range msgLayout.Lines {
				if i >= 5 {
					break
				}
				x := bounds.Min.X + 20
				y := bounds.Min.Y + bounds.Height()/2 + float32(i)*20
				origin := gfx.Point{X: x, Y: y}
				for _, run := range line.Runs {
					list.Add(gfx.DrawGlyphRun{
						Run:    run,
						Origin: origin,
						Brush:  gfx.SolidBrush(f.th.Color(theme.ColorText)),
					})
				}
			}
		}
	}
}

// SetRunner sets the runner for execution control.
func (f *ReplayRootFacet) SetRunner(runner *engine.Runner) {
	f.runner = runner
}

// SetRunCallback sets the callback for run completion notifications.
func (f *ReplayRootFacet) SetRunCallback(cb func(*model.RunResult)) {
	f.runCallback = cb
}

// ReloadScenario reloads the currently selected scenario and runs it.
func (f *ReplayRootFacet) ReloadScenario() error {
	scenario, ok := store.SelectedScenario()
	if !ok {
		return fmt.Errorf("no scenario selected")
	}

	if f.runner == nil {
		return fmt.Errorf("no runner available")
	}

	go func() {
		result, err := f.runner.Run(scenario)
		if err != nil {
			// Error is already logged by the runner
			return
		}
		if f.runCallback != nil {
			f.runCallback(result)
		}
		f.RequestFrame()
	}()

	return nil
}

// CancelRun cancels the current execution.
func (f *ReplayRootFacet) CancelRun() error {
	if f.runner == nil {
		return fmt.Errorf("no runner available")
	}

	f.runner.Cancel()
	return nil
}

// ExportBundle exports the last run bundle to the specified path.
func (f *ReplayRootFacet) ExportBundle(path string) error {
	if f.runner == nil {
		return fmt.Errorf("no runner available")
	}

	bundle := f.runner.GetLastBundle()
	if bundle == nil {
		return fmt.Errorf("no bundle available - run a scenario first")
	}

	if path == "" {
		path = bundle.OutputPath
	}

	if err := bundle.SaveToDisk(); err != nil {
		return fmt.Errorf("failed to save bundle: %w", err)
	}

	return nil
}

// GetExecutionState returns the current execution state.
func (f *ReplayRootFacet) GetExecutionState() store.ExecutionState {
	return store.ExecutionStateStore.Get()
}

// IsRunning returns true if a scenario is currently running.
func (f *ReplayRootFacet) IsRunning() bool {
	exec := store.ExecutionStateStore.Get()
	return exec.IsRunning()
}

// SelectScenario selects a scenario by ID.
func (f *ReplayRootFacet) SelectScenario(id model.ScenarioID) {
	store.SelectScenario(id)
}

// GetSelectedScenario returns the currently selected scenario.
func (f *ReplayRootFacet) GetSelectedScenario() (*model.Scenario, bool) {
	return store.SelectedScenario()
}

// GetRunHistory returns the run history.
func (f *ReplayRootFacet) GetRunHistory() *store.RunHistory {
	return store.RunHistoryStore.Get()
}
