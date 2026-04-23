package ui

import (
	"fmt"
	"runtime/debug"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
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
	adder          facetChildAdder
	frameRequester interface{ RequestFrame() }

	// Layout state
	shellBounds    ShellBounds
	sidebarWidth   float32
	inspectorWidth float32

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
		sidebarWidth:   sidebarWidthDefault,
		inspectorWidth: inspectorWidthDefault,
	}

	// Create child facets
	root.header = NewHeaderFacet(th, shaper, meta)
	root.sidebar = NewSidebarFacet(th, shaper)
	root.content = NewContentFacet(th, shaper)
	root.inspector = NewInspectorFacet(th, shaper)
	root.footer = NewFooterFacet(th, shaper)

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

	// Attach children with their respective layer specs
	attachChild(&f.Facet, f, f.header, f.adder, layout.ChildAttachment{LayerID: 1})
	attachChild(&f.Facet, f, f.sidebar, f.adder, layout.ChildAttachment{LayerID: 2})
	attachChild(&f.Facet, f, f.content, f.adder, layout.ChildAttachment{LayerID: 3})
	attachChild(&f.Facet, f, f.inspector, f.adder, layout.ChildAttachment{LayerID: 4})
	attachChild(&f.Facet, f, f.footer, f.adder, layout.ChildAttachment{LayerID: 5})
}

// OnDetach handles detachment.
func (f *CatalogRootFacet) OnDetach() {}

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

	shell := CalculateShellBounds(bounds, f.sidebarWidth, f.inspectorWidth)

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

	f.shellBounds = CalculateShellBounds(bounds, f.sidebarWidth, f.inspectorWidth)

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
			origin := gfx.Point{X: x, Y: y}
			for _, run := range line.Runs {
				list.Add(gfx.DrawGlyphRun{
					Run:    run,
					Origin: origin,
					Brush:  gfx.SolidBrush(f.th.Color(theme.ColorText)),
				})
			}
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
