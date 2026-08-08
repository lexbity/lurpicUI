package studio

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

// ExhibitInspector is the per-exhibit inspector pane: a Card showing the active
// exhibit's title, description, and demonstrated-mark count, all store-bound so
// the panel updates when the exhibit switches. The content is read-only text —
// the framework Card's self-projected content (F-card-content) is exactly the
// right host for non-interactive display.
type ExhibitInspector struct {
	facet.Facet
	layout facet.LayoutRole

	card      *structure.Card
	titleText *primitive.Text
	descText  *primitive.Text
	countText *primitive.Text

	titleDesc *store.Derived[string]
	descStore *store.Derived[string]
	countDesc *store.Derived[string]
}

// NewExhibitInspector builds the inspector over the shared shell state. counts
// maps an exhibit id to its demonstrated mark count (computed once by walking
// each exhibit's root facet tree).
func NewExhibitInspector(shell *ShellState, counts map[ExhibitID]int) *ExhibitInspector {
	p := &ExhibitInspector{}

	titleDesc := store.NewDerived(func() string {
		return exhibitTitle(shell.ActiveExhibit.Get())
	}, shell.ActiveExhibit)
	descStore := store.NewDerived(func() string {
		return exhibitDescription(shell.ActiveExhibit.Get())
	}, shell.ActiveExhibit)
	countDesc := store.NewDerived(func() string {
		return fmt.Sprintf("%d marks demonstrated", counts[shell.ActiveExhibit.Get()])
	}, shell.ActiveExhibit)

	p.titleDesc = titleDesc
	p.descStore = descStore
	p.countDesc = countDesc

	p.titleText = primitive.NewText(marks.FromDerived(titleDesc, facet.DirtyProjection))
	p.titleText.Typography = marks.Const(theme.TextHeadingS)
	p.descText = primitive.NewText(marks.FromDerived(descStore, facet.DirtyProjection))
	p.descText.Typography = marks.Const(theme.TextBodyS)
	p.countText = primitive.NewText(marks.FromDerived(countDesc, facet.DirtyProjection))
	p.countText.Typography = marks.Const(theme.TextLabelM)

	p.card = structure.NewCard("Exhibit")
	p.card.GridColumns = marks.Const(1)
	p.card.GridRows = marks.Const(3)
	p.card.ChildrenContent = []structure.CardChild{
		{Key: "title", Facet: p.titleText, Grid: facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1}},
		{Key: "desc", Facet: p.descText, Grid: facet.GridPlacement{ColStart: 0, RowStart: 1, ColSpan: 1, RowSpan: 1}},
		{Key: "count", Facet: p.countText, Grid: facet.GridPlacement{ColStart: 0, RowStart: 2, ColSpan: 1, RowSpan: 1}},
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

// Card returns the inspector's content card.
func (p *ExhibitInspector) Card() *structure.Card { return p.card }

// TitleText returns the title text mark.
func (p *ExhibitInspector) TitleText() *primitive.Text { return p.titleText }

// DescText returns the description text mark.
func (p *ExhibitInspector) DescText() *primitive.Text { return p.descText }

// CountText returns the mark-count text mark.
func (p *ExhibitInspector) CountText() *primitive.Text { return p.countText }

func (p *ExhibitInspector) Base() *facet.Facet { p.BindImpl(p); return &p.Facet }
func (p *ExhibitInspector) OnAttach(_ facet.AttachContext) {
}
func (p *ExhibitInspector) OnDetach()     {}
func (p *ExhibitInspector) OnActivate()   {}
func (p *ExhibitInspector) OnDeactivate() {}
