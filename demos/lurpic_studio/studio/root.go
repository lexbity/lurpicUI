// Package studio builds the Lurpic Studio application shell and exhibits.
//
// Slice P0 provides the placeholder root facet the gallery shell will replace
// in Slice P2. Subsequent slices add the workshop split (index | stage |
// inspector) and the six exhibits plus the capability index.
package studio

import (
	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Root is the placeholder top-level facet of the gallery shell. It fills the
// window with the resolved theme background so the app visibly runs. Slice P2
// replaces this single facet with the chrome stack, the 3-pane split, and the
// status bar.
type Root struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	background gfx.Color
}

// BuildRoot constructs the root facet for a resolved app build context.
// It is the app's rootBuilder (demos/lurpic_studio/main.go).
func BuildRoot(ctx app.BuildContext) facet.FacetImpl {
	r := &Root{
		background: ctx.Theme.Color(theme.ColorBackground),
	}
	r.layout = facet.LayoutRole{ //lurpiclint:ignore LL001 -- placeholder shell root; Slice P2 replaces with a linear group-parent host
		OnMeasure: func(_ facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			size := c.MaxSize
			if size == (gfx.Size{}) {
				size = c.MinSize
			}
			return facet.MeasureResult{Size: size}
		},
	}
	r.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(r.background)})
		},
	}
	r.Facet = facet.NewFacet()
	r.AddRole(&r.layout)
	r.AddRole(&r.render)
	return r
}

func (r *Root) Base() *facet.Facet             { r.BindImpl(r); return &r.Facet }
func (r *Root) OnAttach(_ facet.AttachContext) {}
func (r *Root) OnDetach()                      {}
func (r *Root) OnActivate()                    {}
func (r *Root) OnDeactivate()                  {}
