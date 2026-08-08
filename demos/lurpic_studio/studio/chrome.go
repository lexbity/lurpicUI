package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
)

// cmdKIcon is the command-palette trigger glyph (inline SVG so it renders
// deterministically without the asset manager, matching the marks' own golden
// tests).
const cmdKIcon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="7"/><path d="M21 21l-4.35-4.35"/></svg>`

const themeIcon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41"/></svg>`

// ChromeStack is the top chrome bar: the title on the left and the
// command-palette (⌘K) and theme triggers on the right.
//
// It is a linear-kind group-parent host that arranges its mark children
// directly — the toolbar/action_group production idiom — because the standard
// text/icon-button marks do not declare SupportsLinear, which layout/linear
// requires (F-linear-marks). The linear policy is consumed by Root.
type ChromeStack struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	shell *ShellState
	title facet.FacetImpl
	cmdK  facet.FacetImpl
	theme facet.FacetImpl

	gap        float32
	padX       float32
	padY       float32
	background gfx.Color

	rt      facet.RuntimeServices
	cleanup func()
}

// NewChromeStack builds the chrome bar for the given resolved theme and shared
// shell state. The ⌘K button opens the command palette; the theme button
// toggles the shell's compact density (a genuine runtime preference → re-layout
// response).
func NewChromeStack(themeCtx theme.ResolvedContext, shell *ShellState) *ChromeStack {
	c := &ChromeStack{
		shell:      shell,
		title:      primitive.NewText(marks.Const("Lurpic Studio")),
		cmdK:       action.NewIconButton(primitive.IconSVG(cmdKIcon)),
		theme:      action.NewIconButton(primitive.IconSVG(themeIcon)),
		gap:        float32(themeCtx.Spacing(theme.SpacingS)),
		padX:       float32(themeCtx.Spacing(theme.SpacingL)),
		padY:       float32(themeCtx.Spacing(theme.SpacingS)),
		background: themeCtx.Color(theme.ColorSurface),
	}
	c.Facet = facet.NewFacet()
	c.AddChild(c.title.Base()) //lurpiclint:ignore LL021 -- chrome hosts action marks as regular children, not overlays (LL021 over-fires on any field ref)
	c.AddChild(c.cmdK.Base())  //lurpiclint:ignore LL021 -- chrome hosts action marks as regular children, not overlays (LL021 over-fires on any field ref)
	c.AddChild(c.theme.Base()) //lurpiclint:ignore LL021 -- chrome hosts action marks as regular children, not overlays (LL021 over-fires on any field ref)

	c.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke linear-kind group-parent host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
			return c.measure(ctx, constraints)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			c.arrange(ctx, bounds)
		},
	}
	c.layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearHorizontal,
		Policy:   groupPolicy{kind: facet.GroupLayoutLinearHorizontal, host: c},
		Children: c,
	}
	c.layout.Child = linearChildContract(facet.StretchPolicy{
		Width:  facet.StretchAlways,
		Height: facet.StretchNever,
	})
	c.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(c.background)})
		},
	}
	c.AddRole(&c.layout)
	c.AddRole(&c.render)
	return c
}

// Title returns the title text facet.
func (c *ChromeStack) Title() facet.FacetImpl { return c.title }

// CmdK returns the command-palette trigger facet.
func (c *ChromeStack) CmdK() facet.FacetImpl { return c.cmdK }

// Theme returns the theme toggle facet.
func (c *ChromeStack) Theme() facet.FacetImpl { return c.theme }

func (c *ChromeStack) items() []facet.FacetImpl {
	return []facet.FacetImpl{c.title, c.cmdK, c.theme}
}

func (c *ChromeStack) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	items := c.items()
	padX, padY := c.padding()
	width := padX * 2
	height := float32(0)
	for i, item := range items {
		role := item.Base().LayoutRole()
		role.Measure(ctx, facet.Constraints{MaxSize: constraints.MaxSize})
		size := role.MeasuredSize
		width += size.W
		if i < len(items)-1 {
			width += c.gap
		}
		if size.H > height {
			height = size.H
		}
	}
	height += padY * 2
	return facet.MeasureResult{Size: gfx.Size{W: width, H: height}}
}

// padding returns the chrome padding, tightened when compact density is on.
func (c *ChromeStack) padding() (float32, float32) {
	if c.shell != nil && c.shell.Compact.Get() {
		return c.padX * 0.6, c.padY * 0.6
	}
	return c.padX, c.padY
}

func (c *ChromeStack) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if bounds.IsEmpty() {
		return
	}
	items := c.items()
	sizes := make([]gfx.Size, len(items))
	for i, item := range items {
		sizes[i] = item.Base().LayoutRole().MeasuredSize
	}
	padX, _ := c.padding()
	// Right-align the trailing buttons, then place the title on the left.
	x := bounds.Max.X - padX
	for i := len(items) - 1; i >= 1; i-- {
		w := sizes[i].W
		x -= w
		arrangeChild(facet.ArrangeContext{}, items[i], gfx.RectFromXYWH(x, bounds.Min.Y, w, bounds.Height()))
		x -= c.gap
	}
	titleW := sizes[0].W
	arrangeChild(facet.ArrangeContext{}, items[0], gfx.RectFromXYWH(bounds.Min.X+padX, bounds.Min.Y, titleW, bounds.Height()))
}

// Children returns the chrome's group children (the group-parent bridge's
// ChildSource).
func (c *ChromeStack) Children() []facet.GroupChild {
	return linearGroupChildren(c.items())
}

func (c *ChromeStack) Base() *facet.Facet { c.BindImpl(c); return &c.Facet }

// OnAttach wires the chrome buttons: ⌘K opens the command palette and the
// theme button toggles compact density; both re-lay the chrome so the toggle
// is visibly reactive.
func (c *ChromeStack) OnAttach(ctx facet.AttachContext) {
	c.rt = ctx.Runtime
	cmdK := c.cmdK.(*action.IconButton)
	themeBtn := c.theme.(*action.IconButton)

	cmdKID := cmdK.Activated.Subscribe(func(signal.Unit) {
		if !c.shell.CommandOpen.Get() {
			c.shell.CommandOpen.Set(true)
		}
	})
	themeBtnID := themeBtn.Activated.Subscribe(func(signal.Unit) {
		c.shell.Compact.Set(!c.shell.Compact.Get())
	})
	compactID := c.shell.Compact.OnChange.Subscribe(func(signal.Change[bool]) {
		invalidateLayout(c, ctx.Runtime, "chrome.compact")
	})
	c.cleanup = func() {
		cmdK.Activated.Unsubscribe(cmdKID)
		themeBtn.Activated.Unsubscribe(themeBtnID)
		c.shell.Compact.OnChange.Unsubscribe(compactID)
	}
}

// OnDetach clears the chrome's subscriptions.
func (c *ChromeStack) OnDetach() {
	if c.cleanup != nil {
		c.cleanup()
		c.cleanup = nil
	}
}

// OnActivate is unused.
func (c *ChromeStack) OnActivate() {}

// OnDeactivate is unused.
func (c *ChromeStack) OnDeactivate() {}
