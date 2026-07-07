package studio

import (
	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// placeholderPane is a colored placeholder facet for Phase 3 scaffolding.
// It will be replaced with real marks (card, list, scroll_region, etc.) in later phases.
type placeholderPane struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	color  gfx.Color
	label  string
}

func (p *placeholderPane) Base() *facet.Facet               { return &p.Facet }
func (p *placeholderPane) OnAttach(ctx facet.AttachContext) {}
func (p *placeholderPane) OnDetach()                        {}
func (p *placeholderPane) OnActivate()                      {}
func (p *placeholderPane) OnDeactivate()                    {}

func newPlaceholderPane(label string, color gfx.Color) *placeholderPane {
	p := &placeholderPane{label: label, color: color}
	p.Facet = facet.NewFacet()

	// Each pane reports its intrinsic size
	p.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		// For Phase 3, placeholders report fixed sizes
		// (Real marks in Phase 4+ will compute their own sizes)
		var size gfx.Size
		switch p.label {
		case "chrome":
			size = gfx.Size{W: 0, H: 40} // Height fixed, width flex
		case "status":
			size = gfx.Size{W: 0, H: 32} // Height fixed, width flex
		case "sources":
			size = gfx.Size{W: 220, H: 200} // Fixed width
		case "inspector":
			size = gfx.Size{W: 280, H: 250} // Fixed width
		default: // "center"
			size = gfx.Size{W: 100, H: 100} // Min size, will flex
		}

		// Constrain to available space
		if c.MaxSize.W > 0 && size.W > c.MaxSize.W {
			size.W = c.MaxSize.W
		}
		if c.MaxSize.H > 0 && size.H > c.MaxSize.H {
			size.H = c.MaxSize.H
		}
		if c.MinSize.W > size.W {
			size.W = c.MinSize.W
		}
		if c.MinSize.H > size.H {
			size.H = c.MinSize.H
		}
		return facet.MeasureResult{Size: size}
	}

	p.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		p.layout.ArrangedBounds = bounds
	}

	p.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(p.color)})
	}

	p.Facet.AddRole(&p.layout)
	p.Facet.AddRole(&p.render)
	return p
}

// RootFacet is the demo's root facet, implementing responsive layout.
// It hosts placeholder children via framework layout policies (wide: 3-pane split, narrow: stacked).
type RootFacet struct {
	facet.Facet
	layout   facet.LayoutRole
	render   facet.RenderRole
	appState *state.AppState

	// Framework layout policies for wide and narrow modes
	// These are facet implementations that we delegate to
	wideLayout   *facet.Facet // vertical: chrome, middle row, status (as ColumnLayout)
	narrowLayout *facet.Facet // vertical: chrome, sources, center, inspector, status (as ColumnLayout)
}

func (r *RootFacet) Base() *facet.Facet               { return &r.Facet }
func (r *RootFacet) OnAttach(ctx facet.AttachContext) {}
func (r *RootFacet) OnDetach()                        {}
func (r *RootFacet) OnActivate()                      {}
func (r *RootFacet) OnDeactivate()                    {}

func newRootFacet(as *state.AppState, ctx app.BuildContext) *RootFacet {
	r := &RootFacet{appState: as}

	// Create theme colors for placeholders
	bg := ctx.Theme.TokenSet().Color.Background
	surface := ctx.Theme.TokenSet().Color.Surface
	surfaceVariant := ctx.Theme.TokenSet().Color.SurfaceVariant
	sourcesColor := gfx.Color{R: 0.16, G: 0.18, B: 0.22, A: 1}
	centerColor := gfx.Color{R: 0.12, G: 0.13, B: 0.16, A: 1}
	inspectorColor := gfx.Color{R: 0.16, G: 0.16, B: 0.19, A: 1}

	// Build wide mode layout: column(chrome, row(sources, center, inspector), status)
	// Note: Phase 3 uses separate placeholder instances for wide/narrow.
	// Later phases will reuse the same mark instances via the layer system.
	wideCol := layout.NewColumnLayout()
	wideCol.Add(layout.Fixed(newPlaceholderPane("chrome", surfaceVariant)))

	wideMiddleRow := layout.NewRowLayout()
	wideMiddleRow.Add(layout.Fixed(newPlaceholderPane("sources", sourcesColor)))
	wideMiddleRow.Add(layout.Flexible(newPlaceholderPane("center", centerColor), 1))
	wideMiddleRow.Add(layout.Fixed(newPlaceholderPane("inspector", inspectorColor)))
	wideCol.Add(layout.Flexible(wideMiddleRow, 1))

	wideCol.Add(layout.Fixed(newPlaceholderPane("status", surface)))
	r.wideLayout = &wideCol.Facet

	// Build narrow mode layout: column(chrome, sources, center, inspector, status)
	narrowCol := layout.NewColumnLayout()
	narrowCol.Add(layout.Fixed(newPlaceholderPane("chrome", surfaceVariant)))
	narrowCol.Add(layout.Fixed(newPlaceholderPane("sources", sourcesColor)))
	narrowCol.Add(layout.Flexible(newPlaceholderPane("center", centerColor), 1))
	narrowCol.Add(layout.Fixed(newPlaceholderPane("inspector", inspectorColor)))
	narrowCol.Add(layout.Fixed(newPlaceholderPane("status", surface)))
	r.narrowLayout = &narrowCol.Facet

	// Background fill
	r.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(bg)})
	}

	r.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		// Update responsive mode on measure (only when it changes)
		mode := ModeFor(c.MaxSize)
		if mode != as.LayoutMode.Get() {
			as.LayoutMode.Set(mode)
		}

		// Measure the appropriate layout policy based on mode
		var targetLayout *facet.Facet
		if mode == state.LayoutWide {
			targetLayout = r.wideLayout
		} else {
			targetLayout = r.narrowLayout
		}

		if targetLayout == nil || targetLayout.LayoutRole() == nil {
			return facet.MeasureResult{Size: c.MaxSize}
		}
		return targetLayout.LayoutRole().OnMeasure(ctx, c)
	}

	r.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		r.layout.ArrangedBounds = bounds

		mode := as.LayoutMode.Get()

		var targetLayout *facet.Facet
		if mode == state.LayoutWide {
			targetLayout = r.wideLayout
		} else {
			targetLayout = r.narrowLayout
		}

		if targetLayout != nil && targetLayout.LayoutRole() != nil {
			targetLayout.LayoutRole().OnArrange(ctx, bounds)
		}
	}

	r.Facet.AddRole(&r.layout)
	r.Facet.AddRole(&r.render)

	return r
}

// BuildRoot constructs the root facet for the demo.
func BuildRoot(as *state.AppState, ctx app.BuildContext) facet.FacetImpl {
	return newRootFacet(as, ctx)
}
