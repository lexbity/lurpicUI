package action

import (
	"codeburg.org/lexbit/lurpicui/facet"
)

// two marks claim the same author identity — the runtime panics at attach;
// LL039 flags the statically-visible case.
type saveButton struct {
	facet.Facet
}

type saveToolbar struct {
	facet.Facet
}

func newSaveButton() *saveButton {
	b := &saveButton{Facet: facet.NewFacet()}
	b.SetTestID("action.save")
	return b
}

func newSaveToolbar() *saveToolbar {
	t := &saveToolbar{Facet: facet.NewFacet()}
	t.TestID = "action.save"
	return t
}
