package studio

import (
	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/signal"
)

type RootFacet struct {
	facet.Facet
	layout   facet.LayoutRole
	render   facet.RenderRole
	appState *state.AppState

	chromeColumn   *layout.ColumnLayout
	sourcesPanel   *sourcesPanel
	centerPanel    *centerPanel
	inspectorPanel *inspectorPanel

	ribbon      *action.Ribbon
	toolbar     *action.Toolbar
	breadcrumbs *navigation.Breadcrumbs
	actionBar   *action.ActionBar

	overlays  *overlays
	statusBar *statusBar

	hamburger *action.IconButton
	chromeRow *layout.RowLayout
}

func (r *RootFacet) Base() *facet.Facet               { return &r.Facet }
func (r *RootFacet) OnAttach(ctx facet.AttachContext) {}
func (r *RootFacet) OnDetach()                        {}
func (r *RootFacet) OnActivate()                      {}
func (r *RootFacet) OnDeactivate()                    {}

func newRootFacet(as *state.AppState, ctx app.BuildContext) *RootFacet {
	r := &RootFacet{appState: as}
	r.Facet = facet.NewFacet()

	bg := ctx.Theme.TokenSet().Color.Background

	sp := newSourcesPanel(as)
	cp := newCenterPanel(as)
	ip := newInspectorPanel(as)
	sb := newStatusBar(as)
	ribbon, toolbar, breadcrumbs, actionBar := newChromePane(as)
	ov := newOverlays(as)

	r.ribbon = ribbon
	r.toolbar = toolbar
	r.breadcrumbs = breadcrumbs
	r.actionBar = actionBar

	r.hamburger = action.NewIconButton(primitive.IconRef("menu"))
	r.hamburger.Activated.Subscribe(func(signal.Unit) {
		ov.navDrawer.Open = marks.Const(!ov.navDrawer.Open.Get())
		ov.navDrawer.Invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	})

	allowLinear(toolbar)
	allowLinear(breadcrumbs)
	r.chromeRow = layout.NewRowLayout()
	r.chromeRow.Gap = 0
	r.chromeRow.Add(layout.Fixed(r.hamburger))
	r.chromeRow.Add(layout.Flexible(toolbar, 1))
	r.chromeRow.Add(layout.Fixed(breadcrumbs))

	r.chromeColumn = layout.NewColumnLayout()
	r.chromeColumn.Gap = 0
	r.chromeColumn.Add(layout.Fixed(ribbon))
	r.chromeColumn.Add(layout.Fixed(r.chromeRow))

	// Overlays must be children of the root so they participate in the facet tree.
	r.Facet.AddChild(r.chromeColumn.Base())
	r.Facet.AddChild(sp.col.Base())
	r.Facet.AddChild(cp.col.Base())
	r.Facet.AddChild(ip.col.Base())
	r.Facet.AddChild(sb.row.Base())
	r.Facet.AddChild(ov.dialog.Base())
	r.Facet.AddChild(ov.exportToast.Base())
	r.Facet.AddChild(ov.tooltip.Base())
	r.Facet.AddChild(ov.commandPalette.Base())
	r.Facet.AddChild(ov.popupPalette.Base())
	r.Facet.AddChild(ov.navDrawer.Base())

	r.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(bg)})
	}

	r.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult { //lurpiclint:ignore LL001 -- root responsive layout sizes panels based on container break width
		mode := ModeFor(c.MaxSize)
		if mode != as.LayoutMode.Get() {
			as.LayoutMode.Set(mode)
		}

		w := c.MaxSize.W
		h := c.MaxSize.H
		ribbonH := float32(40)
		toolbarH := float32(30)
		statusH := float32(32)
		chromeH := ribbonH + toolbarH
		middleH := h - chromeH - statusH
		if middleH < 0 {
			middleH = 0
		}

		r.chromeColumn.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: w, H: chromeH}})
		sb.row.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: w, H: statusH}})

		if mode == state.LayoutWide {
			srcW := float32(220)
			insW := float32(280)
			cp.col.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: w - srcW - insW, H: middleH}})
			sp.col.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: srcW, H: middleH}})
			ip.col.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: insW, H: middleH}})
		} else {
			cp.col.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: w, H: middleH}})
			sp.col.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 0, H: 0}})
			ip.col.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 0, H: 0}})
		}

		for _, oc := range overlayChildren(ov) {
			if lr := oc.Base().LayoutRole(); lr != nil {
				lr.Measure(ctx, c)
			}
		}

		return facet.MeasureResult{Size: c.MaxSize}
	}

	r.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) { //lurpiclint:ignore LL001 -- root responsive layout positions panels per break width
		r.layout.ArrangedBounds = bounds

		mode := as.LayoutMode.Get()
		ribbonH := float32(40)
		toolbarH := float32(30)
		statusH := float32(32)
		chromeH := ribbonH + toolbarH
		middleH := bounds.Height() - chromeH - statusH
		if middleH < 0 {
			middleH = 0
		}

		arrangeChildAtCtx(r.chromeColumn, gfx.Rect{
			Min: bounds.Min,
			Max: gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y + chromeH},
		}, ctx)

		y := bounds.Min.Y + chromeH

		if mode == state.LayoutWide {
			srcW := float32(220)
			insW := float32(280)

			arrangeChildAtCtx(sp.col, gfx.Rect{
				Min: gfx.Point{X: bounds.Min.X, Y: y},
				Max: gfx.Point{X: bounds.Min.X + srcW, Y: y + middleH},
			}, ctx)
			arrangeChildAtCtx(cp.col, gfx.Rect{
				Min: gfx.Point{X: bounds.Min.X + srcW, Y: y},
				Max: gfx.Point{X: bounds.Max.X - insW, Y: y + middleH},
			}, ctx)
			arrangeChildAtCtx(ip.col, gfx.Rect{
				Min: gfx.Point{X: bounds.Max.X - insW, Y: y},
				Max: gfx.Point{X: bounds.Max.X, Y: y + middleH},
			}, ctx)
		} else {
			arrangeChildAtCtx(cp.col, gfx.Rect{
				Min: gfx.Point{X: bounds.Min.X, Y: y},
				Max: gfx.Point{X: bounds.Max.X, Y: y + middleH},
			}, ctx)
			arrangeChildAtCtx(sp.col, gfx.Rect{}, ctx)
			arrangeChildAtCtx(ip.col, gfx.Rect{}, ctx)
		}

		arrangeChildAtCtx(sb.row, gfx.Rect{
			Min: gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y + chromeH + middleH},
			Max: bounds.Max,
		}, ctx)
	}

	r.Facet.AddRole(&r.layout)
	r.Facet.AddRole(&r.render)

	r.sourcesPanel = sp
	r.centerPanel = cp
	r.inspectorPanel = ip
	r.overlays = ov
	r.statusBar = sb

	return r
}

func overlayChildren(ov *overlays) []facet.FacetImpl {
	return []facet.FacetImpl{
		ov.dialog, ov.exportToast, ov.tooltip,
		ov.commandPalette, ov.popupPalette, ov.navDrawer,
	}
}

func arrangeChildAtCtx(parent facet.FacetImpl, bounds gfx.Rect, ctx facet.ArrangeContext) {
	if parent == nil || parent.Base() == nil {
		return
	}
	lr := parent.Base().LayoutRole()
	if lr == nil {
		return
	}
	lr.ArrangedBounds = bounds
	if lr.OnArrange != nil {
		lr.OnArrange(ctx, bounds)
	}
}

func BuildRoot(as *state.AppState, ctx app.BuildContext) facet.FacetImpl {
	return newRootFacet(as, ctx)
}
