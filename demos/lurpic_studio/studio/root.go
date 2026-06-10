package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/text"
)

type coloredPane struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	color  gfx.Color
	fixedW float32
	fixedH float32
}

func newColoredPane(color gfx.Color, fixedW, fixedH float32) *coloredPane {
	p := &coloredPane{
		color:  color,
		fixedW: fixedW,
		fixedH: fixedH,
	}
	p.Facet = facet.NewFacet()
	p.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			size := c.MaxSize
			if p.fixedW > 0 && (size.W > p.fixedW || size.W == 0) {
				size.W = p.fixedW
			}
			if p.fixedH > 0 && (size.H > p.fixedH || size.H == 0) {
				size.H = p.fixedH
			}
			if c.MaxSize.W > 0 && size.W > c.MaxSize.W {
				size.W = c.MaxSize.W
			}
			if c.MaxSize.H > 0 && size.H > c.MaxSize.H {
				size.H = c.MaxSize.H
			}
			return facet.MeasureResult{
				Size: size,
				Intrinsic: facet.IntrinsicSize{
					Min:       size,
					Preferred: size,
					Max:       size,
				},
			}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			p.layout.ArrangedBounds = bounds
			p.layout.Constraints = facet.Constraints{MaxSize: gfx.Size{W: bounds.Width(), H: bounds.Height()}}
		},
	}
	p.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			if list == nil || bounds.IsEmpty() {
				return
			}
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(p.color)})
		},
	}
	p.AddRole(&p.layout)
	p.AddRole(&p.render)
	return p
}

func (p *coloredPane) Base() *facet.Facet { p.Facet.BindImpl(p); return &p.Facet }
func (p *coloredPane) OnAttach(ctx facet.AttachContext)  {}
func (p *coloredPane) OnDetach()                         {}
func (p *coloredPane) OnActivate()                       {}
func (p *coloredPane) OnDeactivate()                     {}

type RootFacet struct {
	facet.Facet
	layout   facet.LayoutRole
	render   facet.RenderRole
	appState *state.AppState

	chromeBar       *ChromeBar
	bodyPane        *coloredPane
	statusBar       *StatusBar
	sourcesPanel    *SourcesPanel
	centerPanel     *CenterPanel
	inspectorPanel  *InspectorPanel
	overlayHost     *OverlayHost
	fonts           *text.FontRegistry

	lastLayoutMode state.LayoutMode
}

func NewRoot(appState *state.AppState, windowSize gfx.Size, fonts *text.FontRegistry) *RootFacet {
	initialMode := ModeFor(windowSize)
	appState.LayoutMode.Set(initialMode)
	r := &RootFacet{
		appState:       appState,
		lastLayoutMode: initialMode,
		fonts:          fonts,
	}
	r.Facet = facet.NewFacet()

	r.chromeBar = NewChromeBar(appState, windowSize)
	r.statusBar = NewStatusBar(appState)
	r.sourcesPanel = NewSourcesPanel(appState)
	r.centerPanel = NewCenterPanel(appState, fonts)
	r.inspectorPanel = NewInspectorPanel(appState)

	reg := runtime.NewCommandRegistry()
	registerCommands(reg, appState)
	r.overlayHost = NewOverlayHost(appState, reg)

	r.bodyPane = newColoredPane(gfx.Color{R: 0.14, G: 0.16, B: 0.21, A: 1}, 0, 0)

	r.Facet.AddChild(r.chromeBar.Base())
	r.Facet.AddChild(r.bodyPane.Base())
	r.Facet.AddChild(r.statusBar.Base())
	r.Facet.AddChild(r.overlayHost.Base())

	r.bodyPane.Facet.AddChild(r.sourcesPanel.Base())
	r.bodyPane.Facet.AddChild(r.centerPanel.Base())
	r.bodyPane.Facet.AddChild(r.inspectorPanel.Base())

	r.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return r.onMeasure(ctx, c)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			r.onArrange(bounds)
		},
	}
	r.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			bg := gfx.Color{R: 0.10, G: 0.12, B: 0.16, A: 1}
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(bg)})
		},
	}
	r.AddRole(&r.layout)
	r.AddRole(&r.render)
	return r
}

func (r *RootFacet) Base() *facet.Facet                { return &r.Facet }
func (r *RootFacet) OnAttach(ctx facet.AttachContext)  {}
func (r *RootFacet) OnDetach()                         {}
func (r *RootFacet) OnActivate()                       {}
func (r *RootFacet) OnDeactivate()                     {}

func (r *RootFacet) onMeasure(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
	windowSize := c.MaxSize
	if windowSize == (gfx.Size{}) {
		windowSize = gfx.Size{W: 1280, H: 800}
	}

	newMode := ModeFor(windowSize)
	if newMode != r.lastLayoutMode {
		r.lastLayoutMode = newMode
		r.appState.LayoutMode.Set(newMode)
	}

	r.chromeBar.layout.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: windowSize.W, H: 200}})
	chromeH := r.chromeBar.TotalHeight()

	footerH := float32(32)
	bodyH := windowSize.H - chromeH - footerH
	if bodyH < 0 {
		bodyH = 0
	}

	r.statusBar.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: windowSize.W, H: footerH}})
	r.overlayHost.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: windowSize})

	if newMode == state.LayoutWide {
		sourcesW := float32(200)
		inspectorW := float32(280)
		centerW := windowSize.W - sourcesW - inspectorW
		if centerW < 0 {
			centerW = 0
		}
		r.measureBodyChild(r.sourcesPanel, sourcesW, bodyH)
		r.measureBodyChild(r.centerPanel, centerW, bodyH)
		r.measureBodyChild(r.inspectorPanel, inspectorW, bodyH)
	} else {
		r.measureBodyChild(r.centerPanel, windowSize.W, bodyH)
		r.measureBodyChild(r.sourcesPanel, 0, 0)
		r.measureBodyChild(r.inspectorPanel, 0, 0)
	}

	r.bodyPane.layout.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: windowSize.W, H: bodyH}})

	return facet.MeasureResult{Size: windowSize}
}

func (r *RootFacet) measureBodyChild(f facet.FacetImpl, w, h float32) {
	if f == nil || f.Base() == nil {
		return
	}
	role := f.Base().LayoutRole()
	if role == nil {
		return
	}
	role.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: w, H: h}})
}

func (r *RootFacet) onArrange(bounds gfx.Rect) {
	windowSize := gfx.Size{W: bounds.Width(), H: bounds.Height()}
	chromeH := r.chromeBar.TotalHeight()
	footerH := float32(32)
	bodyH := windowSize.H - chromeH - footerH
	if bodyH < 0 {
		bodyH = 0
	}

	r.chromeBar.layout.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementFree}}, gfx.RectFromXYWH(0, 0, windowSize.W, chromeH))

	bodyBounds := gfx.RectFromXYWH(0, chromeH, windowSize.W, bodyH)
	r.bodyPane.layout.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementFree}}, bodyBounds)

	if r.lastLayoutMode == state.LayoutWide {
		sourcesW := float32(200)
		inspectorW := float32(280)
		centerW := windowSize.W - sourcesW - inspectorW
		if centerW < 0 {
			centerW = 0
		}
		r.arrangeBodyChild(r.sourcesPanel, gfx.RectFromXYWH(0, chromeH, sourcesW, bodyH))
		r.arrangeBodyChild(r.centerPanel, gfx.RectFromXYWH(sourcesW, chromeH, centerW, bodyH))
		r.arrangeBodyChild(r.inspectorPanel, gfx.RectFromXYWH(sourcesW+centerW, chromeH, inspectorW, bodyH))
	} else {
		r.arrangeBodyChild(r.centerPanel, gfx.RectFromXYWH(0, chromeH, windowSize.W, bodyH))
		r.arrangeBodyChild(r.sourcesPanel, gfx.Rect{})
		r.arrangeBodyChild(r.inspectorPanel, gfx.Rect{})
	}

	r.statusBar.Base().LayoutRole().Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(0, chromeH+bodyH, windowSize.W, footerH))
	r.overlayHost.Base().LayoutRole().Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(0, 0, windowSize.W, windowSize.H))
}

func (r *RootFacet) arrangeBodyChild(f facet.FacetImpl, rect gfx.Rect) {
	if f == nil || f.Base() == nil {
		return
	}
	role := f.Base().LayoutRole()
	if role == nil {
		return
	}
	role.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, rect)
}

func (r *RootFacet) ArrangedBounds() struct {
	Header    gfx.Rect
	Body      gfx.Rect
	Footer    gfx.Rect
	Sources   gfx.Rect
	Center    gfx.Rect
	Inspector gfx.Rect
} {
	return struct {
		Header    gfx.Rect
		Body      gfx.Rect
		Footer    gfx.Rect
		Sources   gfx.Rect
		Center    gfx.Rect
		Inspector gfx.Rect
	}{
		Header:    r.chromeBar.layout.ArrangedBounds,
		Body:      r.bodyPane.layout.ArrangedBounds,
		Footer:    r.bodyChildArrangedBounds(r.statusBar),
		Sources:   r.bodyChildArrangedBounds(r.sourcesPanel),
		Center:    r.bodyChildArrangedBounds(r.centerPanel),
		Inspector: r.bodyChildArrangedBounds(r.inspectorPanel),
	}
}

func (r *RootFacet) bodyChildArrangedBounds(f facet.FacetImpl) gfx.Rect {
	if f == nil || f.Base() == nil {
		return gfx.Rect{}
	}
	role := f.Base().LayoutRole()
	if role == nil {
		return gfx.Rect{}
	}
	return role.ArrangedBounds
}
