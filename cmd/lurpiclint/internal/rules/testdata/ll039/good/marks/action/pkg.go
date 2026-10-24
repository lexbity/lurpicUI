package action

import (
	"codeburg.org/lexbit/lurpicui/facet"
)

// distinct author identities per mark — the LDP locator contract.
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
	t.TestID = "action.save-toolbar"
	return t
}
