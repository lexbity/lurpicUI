package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
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
// The bar is a structure.Row composition (RX-2 P1): the title is the weighted
// segment (Weight 1 — it absorbs the free width, which pushes the two icon
// buttons to the right edge), the buttons hug their measured sizes. Compact
// density tightens the row's padding through a Derived over the shell's
// Compact store, so the padding change routes through the RX-1 FR-3 binding
// propagation with no author-written invalidation.
type ChromeStack struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	row   *structure.Row
	title *primitive.Text
	cmdK  *action.IconButton
	theme *action.IconButton

	shell *ShellState

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
		background: themeCtx.Color(theme.ColorSurface),
	}
	c.Facet = facet.NewFacet()

	// Compact density is a content change (padding), not a structural one:
	// the Derived re-resolves on every Compact write and the PadX/PadY
	// bindings route the re-measure (RX-1 FR-3) — no subscription needed.
	gap := float32(themeCtx.Spacing(theme.SpacingS))
	padX := float32(themeCtx.Spacing(theme.SpacingL))
	padY := float32(themeCtx.Spacing(theme.SpacingS))
	padXCompact := store.NewDerived(func() float32 {
		if shell.Compact.Get() {
			return padX * 0.6
		}
		return padX
	}, shell.Compact)
	padYCompact := store.NewDerived(func() float32 {
		if shell.Compact.Get() {
			return padY * 0.6
		}
		return padY
	}, shell.Compact)

	c.row = structure.NewRow(
		[]structure.AxisChild{
			{Facet: c.title, MarkID: 1, Weight: 1},
			{Facet: c.cmdK, MarkID: 2},
			{Facet: c.theme, MarkID: 3},
		},
		structure.AxisConfig{
			Gap:        gap,
			CrossAlign: structure.CrossAlignCenter,
		},
	)
	// Compact density is content: the padding sources are Derived over the
	// shell's Compact store, so a toggle re-measures the row via the RX-1
	// FR-3 binding propagation with no author-written invalidation.
	c.row.PadX = marks.FromDerived(padXCompact, facet.DirtyLayout|facet.DirtyProjection)
	c.row.PadY = marks.FromDerived(padYCompact, facet.DirtyLayout|facet.DirtyProjection)

	c.AddChild(c.row.Base()) //lurpiclint:ignore LL021 -- the shell hosts the composition row as a regular child, not an overlay

	c.layout = facet.LayoutRole{ //lurpiclint:ignore * -- single-child wrapper: background fill + measure/arrange delegation to the row (structure.Row owns the layout)
		OnMeasure: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
			result := c.row.Base().LayoutRole().Measure(ctx, constraints)
			c.layout.MeasuredSize = result.Size
			return result
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			c.layout.ArrangedBounds = bounds
			if role := c.row.Base().LayoutRole(); role != nil {
				role.Arrange(ctx, bounds)
			}
		},
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

// Title returns the title text mark.
func (c *ChromeStack) Title() *primitive.Text { return c.title }

// CmdK returns the command-palette trigger mark.
func (c *ChromeStack) CmdK() *action.IconButton { return c.cmdK }

// Theme returns the theme toggle mark.
func (c *ChromeStack) Theme() *action.IconButton { return c.theme }

// Row returns the bar's composition row (the structure.row coverage instance).
func (c *ChromeStack) Row() *structure.Row { return c.row }

// Base satisfies facet.FacetImpl.
func (c *ChromeStack) Base() *facet.Facet { c.BindImpl(c); return &c.Facet }

// OnAttach wires the chrome buttons: ⌘K opens the command palette and the
// theme button toggles compact density (the padding response rides the
// Derived-bound PadX/PadY bindings — no manual invalidation routing).
func (c *ChromeStack) OnAttach(ctx facet.AttachContext) {
	c.rt = ctx.Runtime

	cmdKID := c.cmdK.Activated.Subscribe(func(signal.Unit) {
		if !c.shell.CommandOpen.Get() {
			c.shell.CommandOpen.Set(true)
		}
	})
	themeBtnID := c.theme.Activated.Subscribe(func(signal.Unit) {
		c.shell.Compact.Set(!c.shell.Compact.Get())
	})
	c.cleanup = func() {
		c.cmdK.Activated.Unsubscribe(cmdKID)
		c.theme.Activated.Unsubscribe(themeBtnID)
	}
}

// OnDetach clears the chrome's button subscriptions.
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
