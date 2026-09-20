package studio

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

// ExhibitInspector is the per-exhibit inspector pane: a Card showing the active
// exhibit's title, description, and demonstrated-mark count, all store-bound so
// the panel updates when the exhibit switches. The content is read-only text —
// the framework Card's self-projected content (F-card-content) is exactly the
// right host for non-interactive display. In sheet mode (the narrow bottom
// sheet, FR-17c) it renders a static drag-handle affordance and closes on
// Escape.
type ExhibitInspector struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	input  facet.InputRole

	card      *structure.Card
	titleText *primitive.Text
	descText  *primitive.Text
	countText *primitive.Text
	hintText  *primitive.Text

	titleDesc *store.Derived[string]
	descStore *store.Derived[string]
	countDesc *store.Derived[string]
	hintDesc  *store.Derived[string]

	shell       *ShellState
	sheetMode   bool
	handleColor gfx.Color
}

// NewExhibitInspector builds the inspector over the shared shell state. counts
// maps an exhibit id to its demonstrated mark count (computed once by walking
// each exhibit's root facet tree).
func NewExhibitInspector(shell *ShellState, counts map[ExhibitID]int) *ExhibitInspector {
	p := &ExhibitInspector{shell: shell}

	titleDesc := store.NewDerived(func() string {
		return exhibitTitle(shell.ActiveExhibit.Get())
	}, shell.ActiveExhibit)
	descStore := store.NewDerived(func() string {
		return exhibitDescription(shell.ActiveExhibit.Get())
	}, shell.ActiveExhibit)
	countDesc := store.NewDerived(func() string {
		return markCountText(counts[shell.ActiveExhibit.Get()])
	}, shell.ActiveExhibit)
	hintDesc := store.NewDerived(func() string {
		return "Try: " + exhibitHint(shell.ActiveExhibit.Get())
	}, shell.ActiveExhibit)

	p.titleDesc = titleDesc
	p.descStore = descStore
	p.countDesc = countDesc
	p.hintDesc = hintDesc

	p.titleText = primitive.NewText(marks.FromDerived(titleDesc, facet.DirtyProjection))
	p.titleText.Typography = marks.Const(theme.TextHeadingS)
	p.descText = primitive.NewText(marks.FromDerived(descStore, facet.DirtyProjection))
	p.descText.Typography = marks.Const(theme.TextBodyS)
	p.descText.MultiLine = marks.Const(true) // wraps in the 280dp inspector pane (FR-9 / AC-9)
	p.countText = primitive.NewText(marks.FromDerived(countDesc, facet.DirtyProjection))
	p.countText.Typography = marks.Const(theme.TextLabelM)
	p.hintText = primitive.NewText(marks.FromDerived(hintDesc, facet.DirtyProjection))
	p.hintText.Typography = marks.Const(theme.TextLabelS)
	p.hintText.MultiLine = marks.Const(true)

	p.card = structure.NewCard("Exhibit")
	p.card.GridColumns = marks.Const(1)
	p.card.GridRows = marks.Const(4)
	p.card.ChildrenContent = []structure.CardChild{
		{Key: "title", Facet: p.titleText, Grid: facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1}},
		{Key: "desc", Facet: p.descText, Grid: facet.GridPlacement{ColStart: 0, RowStart: 1, ColSpan: 1, RowSpan: 1}},
		{Key: "count", Facet: p.countText, Grid: facet.GridPlacement{ColStart: 0, RowStart: 2, ColSpan: 1, RowSpan: 1}},
		{Key: "hint", Facet: p.hintText, Grid: facet.GridPlacement{ColStart: 0, RowStart: 3, ColSpan: 1, RowSpan: 1}},
	}

	p.Facet = facet.NewFacet()
	p.AddChild(p.card.Base())

	p.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke inspector-pane host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			if role := p.card.Base().LayoutRole(); role != nil {
				role.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
			}
			return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			if resolved, ok := ctx.Theme.(theme.ResolvedContext); ok {
				p.handleColor = resolved.Color(theme.ColorTextSecondary)
			}
			if role := p.card.Base().LayoutRole(); role != nil {
				role.Arrange(ctx, bounds)
			}
		},
	}
	p.layout.Child = linearChildContract(facet.StretchPolicy{
		Width:  facet.StretchAlways,
		Height: facet.StretchAlways,
	})
	p.AddRole(&p.layout)
	return p
}

// NewSheetInspector builds the narrow-mode bottom sheet: the wide inspector
// plus a static drag-handle affordance and an Escape dismissal (FR-17c). The
// drag handle is a visual affordance only; drag-to-dismiss is a named follow-on.
func NewSheetInspector(shell *ShellState, counts map[ExhibitID]int) *ExhibitInspector {
	p := NewExhibitInspector(shell, counts)
	p.sheetMode = true

	p.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			if bounds.IsEmpty() {
				return
			}
			// The drag handle: a static rounded bar centered at the sheet top.
			w := float32(40)
			h := float32(4)
			handle := gfx.RectFromXYWH(bounds.Min.X+(bounds.Width()-w)*0.5, bounds.Min.Y+6, w, h)
			list.Add(gfx.FillRect{Rect: handle, Brush: gfx.SolidBrush(p.handleColor)})
		},
	}
	p.input = facet.InputRole{
		OnKey: func(e facet.KeyEvent) bool {
			if !p.sheetMode || p.shell == nil {
				return false
			}
			if e.Kind == platform.KeyPress && e.Key == platform.KeyEscape && p.shell.InspectorOpen.Get() {
				p.shell.InspectorOpen.Set(false)
				return true
			}
			return false
		},
	}
	p.AddRole(&p.render)
	p.AddRole(&p.input)
	return p
}

// markCountText pluralizes the demonstrated-mark count line (RX-1 P9: the
// "1 marks demonstrated" grammar bug).
func markCountText(n int) string {
	if n == 1 {
		return "1 mark demonstrated"
	}
	return fmt.Sprintf("%d marks demonstrated", n)
}

// Card returns the inspector's content card.
func (p *ExhibitInspector) Card() *structure.Card { return p.card }

// TitleText returns the title text mark.
func (p *ExhibitInspector) TitleText() *primitive.Text { return p.titleText }

// DescText returns the description text mark.
func (p *ExhibitInspector) DescText() *primitive.Text { return p.descText }

// CountText returns the mark-count text mark.
func (p *ExhibitInspector) CountText() *primitive.Text { return p.countText }

// Hint returns the "Try:" guidance text mark (RX-1 P9).
func (p *ExhibitInspector) Hint() *primitive.Text { return p.hintText }

func (p *ExhibitInspector) Base() *facet.Facet { p.BindImpl(p); return &p.Facet }
func (p *ExhibitInspector) OnAttach(_ facet.AttachContext) {
}
func (p *ExhibitInspector) OnDetach()     {}
func (p *ExhibitInspector) OnActivate()   {}
func (p *ExhibitInspector) OnDeactivate() {}
