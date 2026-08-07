package studio

import (
	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout/linear"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Root is the linear-vertical gallery shell: the chrome bar, the 3-pane split
// (index | stage | inspector), and the status bar. It is the first production
// consumer of the layout/linear policy, exercised through the group-parent
// bridge (§1.6, GroupLayoutLinearVertical).
type Root struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	chrome  *ChromeStack
	gallery *GallerySplit
	status  *StatusBar

	gap float32
}

// NewRoot builds the gallery shell for a resolved app context.
func NewRoot(ctx app.BuildContext) *Root {
	appState := state.NewAppState(nil) // Slice P5 wires the seed CSV
	stage := NewStage([]Exhibit{NewLayoutPolicies()}, appState)
	r := &Root{
		chrome:  NewChromeStack(ctx.Theme),
		gallery: NewGallerySplit(newShellPanes(stage), dividerSize),
		status:  NewStatusBar(ctx.Theme),
	}
	r.Facet = facet.NewFacet()
	r.AddChild(r.chrome.Base())
	r.AddChild(r.gallery.Base())
	r.AddChild(r.status.Base())

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
	r.AddRole(&r.layout)
	r.AddRole(&r.render)
	return r
}

// BuildRoot constructs the app root facet (the rootBuilder in main.go).
func BuildRoot(ctx app.BuildContext) facet.FacetImpl { return NewRoot(ctx) }

// ChromeStack returns the top chrome bar.
func (r *Root) ChromeStack() *ChromeStack { return r.chrome }

// GallerySplit returns the 3-pane split.
func (r *Root) GallerySplit() *GallerySplit { return r.gallery }

// StatusBar returns the bottom status bar.
func (r *Root) StatusBar() *StatusBar { return r.status }

func (r *Root) measure(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
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
	// Arrange cannot fail here: every child declares a linear placement
	// contract in its constructor, so the policy's contract panics are
	// impossible. The error value is discarded explicitly.
	_, _ = policy.Arrange(children, bounds)
}

// Children returns the shell's immediate group children (the group-parent
// bridge's ChildSource).
func (r *Root) Children() []facet.GroupChild {
	return []facet.GroupChild{
		linearGroupChild(crossStretch(0), r.chrome),
		linearGroupChild(facet.LinearPlacement{Order: 1, CrossAxisAlign: facet.CrossAxisStretch, MainAxisSize: facet.MainAxisMax}, r.gallery),
		linearGroupChild(crossStretch(2), r.status),
	}
}

func (r *Root) Base() *facet.Facet             { r.BindImpl(r); return &r.Facet }
func (r *Root) OnAttach(_ facet.AttachContext) {}
func (r *Root) OnDetach()                      {}
func (r *Root) OnActivate()                    {}
func (r *Root) OnDeactivate()                  {}
