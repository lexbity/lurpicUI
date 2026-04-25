package ui

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
	"codeburg.org/lexbit/ui_catalog/store"
)

// CatalogRootFacet is the root facet managing the catalog shell layout.
type CatalogRootFacet struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	th     theme.Context
	shaper *text.Shaper
	meta   model.BuildMetadata

	// Child facets
	header    *HeaderFacet
	sidebar   *SidebarFacet
	content   *ContentFacet
	inspector *InspectorFacet
	footer    *FooterFacet

	// Runtime services
	adder           facetChildAdder
	frameRequester  interface{ RequestFrame() }
	themeSub        signal.SubscriptionID
	densitySub      signal.SubscriptionID
	compareSub      signal.SubscriptionID
	compareThemeSub signal.SubscriptionID

	// Layout state
	sidebarWidth   float32
	inspectorWidth float32
	layoutProfile  LayoutProfile
	responsive     structure.ResponsiveLayout
	shellBounds    ShellBounds
	sidebarPanel   structure.PanelToggleState
	inspectorPanel structure.PanelToggleState
	footerPanel    structure.PanelToggleState
	mobilePanel    string

	// Error state
	hasError bool
	errorMsg string
}

// NewCatalogRootFacet creates the root facet with all child facets.
func NewCatalogRootFacet(th theme.Context, shaper *text.Shaper, meta model.BuildMetadata) *CatalogRootFacet {
	root := &CatalogRootFacet{
		Facet:          facet.NewFacet(),
		th:             th,
		shaper:         shaper,
		meta:           meta,
		sidebarWidth:   sidebarWidthDefaultNormal,
		inspectorWidth: inspectorWidthDefaultNormal,
		layoutProfile:  DefaultLayoutProfile(),
	}

	// Create child facets
	root.header = NewHeaderFacet(th, shaper, meta)
	root.sidebar = NewSidebarFacet(th, shaper)
	root.content = NewContentFacet(th, shaper)
	root.inspector = NewInspectorFacet(th, shaper)
	root.footer = NewFooterFacet(th, shaper)
	root.applyLayoutProfile(root.layoutProfile)

	// Configure layout role
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

	// Configure render role - just background fill with panic recovery
	root.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}

		// Panic recovery for error boundaries
		defer func() {
			if r := recover(); r != nil {
				root.hasError = true
				root.errorMsg = fmt.Sprintf("Render panic: %v\n%s", r, debug.Stack())
				// Still draw background on error
				list.Add(gfx.FillRect{
					Rect:  bounds,
					Brush: gfx.SolidBrush(root.th.Color(theme.ColorBackground)),
				})
				// Draw error message overlay
				root.renderErrorState(list, bounds)
			}
		}()

		// Check if we're in error state
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
func (f *CatalogRootFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

// OnAttach attaches child facets.
func (f *CatalogRootFacet) OnAttach(ctx facet.AttachContext) {
	if adder, ok := ctx.Runtime.(facetChildAdder); ok {
		f.adder = adder
	}
	if req, ok := ctx.Runtime.(interface{ RequestFrame() }); ok {
		f.frameRequester = req
	}
	f.themeSub = store.ThemeStore.OnChange.Subscribe(func(change signal.Change[store.ThemeMode]) {
		f.RequestFrame()
	})
	f.densitySub = store.DensityStore.OnChange.Subscribe(func(change signal.Change[store.DensityMode]) {
		f.applyLayoutProfile(LayoutProfileForDensity(change.New))
		if !f.layout.ArrangedBounds.IsEmpty() {
			f.arrangeShell(f.layout.ArrangedBounds)
		}
		f.RequestFrame()
	})
	f.compareSub = store.CompareStore.OnChange.Subscribe(func(change signal.Change[store.CompareMode]) {
		f.RequestFrame()
	})
	f.compareThemeSub = store.CompareThemeStore.OnChange.Subscribe(func(change signal.Change[store.ThemeMode]) {
		f.RequestFrame()
	})

	// Attach children with their respective layer specs
	attachChild(&f.Facet, f, f.header, f.adder, layout.ChildAttachment{LayerID: 1})
	attachChild(&f.Facet, f, f.sidebar, f.adder, layout.ChildAttachment{LayerID: 2})
	attachChild(&f.Facet, f, f.content, f.adder, layout.ChildAttachment{LayerID: 3})
	attachChild(&f.Facet, f, f.inspector, f.adder, layout.ChildAttachment{LayerID: 4})
	attachChild(&f.Facet, f, f.footer, f.adder, layout.ChildAttachment{LayerID: 5})

	if f.header != nil {
		f.header.OnToggleBrowse.Subscribe(func(_ struct{}) { f.setMobilePanel("browse") })
		f.header.OnToggleDetails.Subscribe(func(_ struct{}) { f.setMobilePanel("details") })
	}
}

// OnDetach handles detachment.
func (f *CatalogRootFacet) OnDetach() {
	store.ThemeStore.OnChange.Unsubscribe(f.themeSub)
	store.DensityStore.OnChange.Unsubscribe(f.densitySub)
	store.CompareStore.OnChange.Unsubscribe(f.compareSub)
	store.CompareThemeStore.OnChange.Unsubscribe(f.compareThemeSub)
}

// OnActivate handles activation.
func (f *CatalogRootFacet) OnActivate() {}

// OnDeactivate handles deactivation.
func (f *CatalogRootFacet) OnDeactivate() {}

// OnLayerSpecs returns layer specifications for child facets.
func (f *CatalogRootFacet) OnLayerSpecs() []layout.LayerSpec {
	bounds := f.layout.ArrangedBounds
	if bounds.IsEmpty() {
		return nil
	}

	shell := f.currentShellBounds(bounds)

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
func (f *CatalogRootFacet) arrangeShell(bounds gfx.Rect) {
	if f == nil {
		return
	}

	shell := f.currentShellBounds(bounds)
	f.shellBounds = shell

	if f.header != nil {
		f.header.layout.Arrange(shell.Header)
	}
	if f.sidebar != nil {
		f.sidebar.layout.Arrange(shell.Sidebar)
	}
	if f.content != nil {
		f.content.layout.Arrange(shell.Content)
	}
	if f.inspector != nil {
		f.inspector.layout.Arrange(shell.Inspector)
	}
	if f.footer != nil {
		f.footer.layout.Arrange(shell.Footer)
	}
}

func (f *CatalogRootFacet) currentShellBounds(bounds gfx.Rect) ShellBounds {
	if f == nil {
		return ShellBounds{}
	}
	if f.responsive.Variant == "" {
		f.syncResponsiveLayout(bounds)
	}
	switch f.responsive.Variant {
	case structure.ShellVariantMobilePortrait:
		headerH := f.layoutProfile.HeaderHeight
		header := gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), headerH)
		contentTop := header.Max.Y
		remaining := gfx.RectFromXYWH(bounds.Min.X, contentTop, bounds.Width(), bounds.Max.Y-contentTop)
		switch f.mobilePanel {
		case "details":
			inspectorH := remaining.Height() * 0.38
			if inspectorH < 220 {
				inspectorH = 220
			}
			if inspectorH > remaining.Height() {
				inspectorH = remaining.Height()
			}
			content := gfx.RectFromXYWH(bounds.Min.X, contentTop, bounds.Width(), remaining.Height()-inspectorH)
			inspector := gfx.RectFromXYWH(bounds.Min.X, content.Max.Y, bounds.Width(), inspectorH)
			return ShellBounds{
				Header:    header,
				Content:   content,
				Sidebar:   gfx.Rect{},
				Inspector: inspector,
				Footer:    gfx.Rect{},
			}
		default:
			sidebarH := remaining.Height() * 0.42
			if sidebarH < 240 {
				sidebarH = 240
			}
			if sidebarH > remaining.Height() {
				sidebarH = remaining.Height()
			}
			sidebar := gfx.RectFromXYWH(bounds.Min.X, contentTop, bounds.Width(), sidebarH)
			content := gfx.RectFromXYWH(bounds.Min.X, sidebar.Max.Y, bounds.Width(), remaining.Height()-sidebarH)
			return ShellBounds{
				Header:    header,
				Sidebar:   sidebar,
				Content:   content,
				Inspector: gfx.Rect{},
				Footer:    gfx.Rect{},
			}
		}
	default:
		shell := CalculateShellBoundsWithProfile(bounds, f.sidebarWidth, f.inspectorWidth, f.layoutProfile)
		if !f.sidebarPanel.Visible(true) {
			shell.Sidebar = gfx.Rect{}
		}
		if !f.inspectorPanel.Visible(true) {
			shell.Inspector = gfx.Rect{}
		}
		if !f.footerPanel.Visible(true) {
			shell.Footer = gfx.Rect{}
		}
		return shell
	}
}

func (f *CatalogRootFacet) syncResponsiveLayout(bounds gfx.Rect) {
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
			f.mobilePanel = "browse"
		}
		switch f.mobilePanel {
		case "details":
			f.sidebarPanel.Collapse()
			f.inspectorPanel.Expand()
			f.footerPanel.Collapse()
		default:
			f.mobilePanel = "browse"
			f.sidebarPanel.Expand()
			f.inspectorPanel.Collapse()
			f.footerPanel.Collapse()
		}
		return
	}
	f.sidebarPanel.Expand()
	f.inspectorPanel.Expand()
	f.footerPanel.Expand()
}

func (f *CatalogRootFacet) setMobilePanel(mode string) {
	if f == nil {
		return
	}
	if mode == "" {
		mode = "browse"
	}
	f.mobilePanel = mode
	if !f.layout.ArrangedBounds.IsEmpty() {
		f.syncResponsiveLayout(f.layout.ArrangedBounds)
		f.arrangeShell(f.layout.ArrangedBounds)
	}
	f.RequestFrame()
}

// RequestFrame requests a new frame render.
func (f *CatalogRootFacet) RequestFrame() {
	if f.frameRequester != nil {
		f.frameRequester.RequestFrame()
	}
}

// SetSidebarWidth updates the sidebar width and rearranges.
func (f *CatalogRootFacet) SetSidebarWidth(width float32) {
	if f == nil {
		return
	}
	f.sidebarWidth = width
	if !f.layout.ArrangedBounds.IsEmpty() {
		f.arrangeShell(f.layout.ArrangedBounds)
	}
	f.RequestFrame()
}

// SetInspectorWidth updates the inspector width and rearranges.
func (f *CatalogRootFacet) SetInspectorWidth(width float32) {
	if f == nil {
		return
	}
	f.inspectorWidth = width
	if !f.layout.ArrangedBounds.IsEmpty() {
		f.arrangeShell(f.layout.ArrangedBounds)
	}
	f.RequestFrame()
}

// HeaderFacet returns the header facet for testing.
func (f *CatalogRootFacet) HeaderFacet() *HeaderFacet {
	if f == nil {
		return nil
	}
	return f.header
}

// SidebarFacet returns the sidebar facet for testing.
func (f *CatalogRootFacet) SidebarFacet() *SidebarFacet {
	if f == nil {
		return nil
	}
	return f.sidebar
}

// ContentFacet returns the content facet for testing.
func (f *CatalogRootFacet) ContentFacet() *ContentFacet {
	if f == nil {
		return nil
	}
	return f.content
}

// InspectorFacet returns the inspector facet for testing.
func (f *CatalogRootFacet) InspectorFacet() *InspectorFacet {
	if f == nil {
		return nil
	}
	return f.inspector
}

// FooterFacet returns the footer facet for testing.
func (f *CatalogRootFacet) FooterFacet() *FooterFacet {
	if f == nil {
		return nil
	}
	return f.footer
}

// renderErrorState draws an error overlay when a panic occurs
func (f *CatalogRootFacet) renderErrorState(list *gfx.CommandList, bounds gfx.Rect) {
	// Semi-transparent red overlay
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(gfx.Color{R: 0.9, G: 0.3, B: 0.3, A: 0.9}),
	})

	// Error title
	if f.shaper != nil {
		titleStyle := f.th.TextStyle(theme.TextHeadingS)
		titleLayout := f.shaper.ShapeSimple("Application Error", titleStyle)
		if titleLayout != nil && len(titleLayout.Lines) > 0 {
			line := titleLayout.Lines[0]
			x := bounds.Min.X + (bounds.Width()-line.Bounds.Width())/2
			y := bounds.Min.Y + bounds.Height()/3
			drawTextLine(list, x, y, line, f.th.Color(theme.ColorText))
		}

		// Error message
		msgStyle := f.th.TextStyle(theme.TextBodyS)
		msgLayout := f.shaper.ShapeSimple(f.errorMsg, msgStyle)
		if msgLayout != nil && len(msgLayout.Lines) > 0 {
			// Show first few lines
			for i, line := range msgLayout.Lines {
				if i >= 5 {
					break
				}
				x := bounds.Min.X + 20
				y := bounds.Min.Y + bounds.Height()/2 + float32(i)*20
				drawTextLine(list, x, y, line, f.th.Color(theme.ColorText))
			}
		}
	}
}

func (f *CatalogRootFacet) applyLayoutProfile(profile LayoutProfile) {
	if f == nil {
		return
	}
	f.layoutProfile = profile
	if f.sidebar != nil {
		f.sidebar.SetLayoutProfile(profile)
	}
	if f.content != nil {
		f.content.SetLayoutProfile(profile)
	}
	if f.inspector != nil {
		f.inspector.SetLayoutProfile(profile)
	}
	if f.footer != nil {
		f.footer.SetLayoutProfile(profile)
	}
	if f.header != nil {
		f.header.SetLayoutProfile(profile)
	}
}
