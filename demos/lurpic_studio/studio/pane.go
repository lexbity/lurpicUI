package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/structure"
)

// Shell pane sizing (fixed panes plus the stage's flex/min).
const (
	indexPaneWidth     = 220
	inspectorPaneWidth = 240
	stagePaneMinWidth  = 240
	stagePaneFlex      = 1
)

// newPaneCard builds a placeholder pane: a Card whose title text is its first
// grid child (Card does not render its Label, so the title is explicit).
func newPaneCard(label string) *structure.Card {
	card := structure.NewCard(label)
	title := primitive.NewText(marks.Const(label))
	card.ChildrenContent = append(card.ChildrenContent, structure.CardChild{
		Key:   "title",
		Facet: title,
		Grid:  facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1},
	})
	return card
}
