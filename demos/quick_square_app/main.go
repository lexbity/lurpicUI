package main

import (
	"fmt"
	"math"
	"os"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func main() {
	cfg := app.DefaultConfig("lurpicUI Square Demo", 800, 600)
	cfg.Render = app.RenderBackendSoftware
	if err := app.Run(cfg, buildRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

type squareDemoRoot struct {
	root   facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
}

func (r *squareDemoRoot) Base() *facet.Facet               { return &r.root }
func (r *squareDemoRoot) OnAttach(ctx facet.AttachContext) {}
func (r *squareDemoRoot) OnDetach()                        {}
func (r *squareDemoRoot) OnActivate()                      {}
func (r *squareDemoRoot) OnDeactivate()                    {}

func buildRoot(ctx app.BuildContext) facet.FacetImpl {
	root := &squareDemoRoot{}
	root.layout = facet.LayoutRole{ //lurpiclint:ignore LL001 -- intentional demo of raw LayoutRole construction
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			size := c.MaxSize
			if size == (gfx.Size{}) {
				size = c.MinSize
			}
			if size == (gfx.Size{}) {
				size = gfx.Size{W: 800, H: 600}
			}
			return facet.MeasureResult{Size: size}
		},
	}
	root.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			bg := gfx.Color{R: 0.10, G: 0.12, B: 0.16, A: 1}
			accent := gfx.Color{R: 0.31, G: 0.78, B: 0.62, A: 1}
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(bg)})
			side := squareSide(bounds)
			center := gfx.Point{
				X: (bounds.Min.X + bounds.Max.X) * 0.5,
				Y: (bounds.Min.Y + bounds.Max.Y) * 0.5,
			}
			square := gfx.RectFromXYWH(center.X-side*0.5, center.Y-side*0.5, side, side)
			list.Add(gfx.FillRect{Rect: square, Brush: gfx.SolidBrush(accent)})
		},
	}
	root.root.AddRole(&root.layout)
	root.root.AddRole(&root.render)
	return root
}

func squareSide(bounds gfx.Rect) float32 {
	ww := bounds.Max.X - bounds.Min.X
	hh := bounds.Max.Y - bounds.Min.Y
	if ww <= 0 || hh <= 0 {
		return 0
	}
	return float32(math.Min(float64(ww), float64(hh))) * 0.6
}
