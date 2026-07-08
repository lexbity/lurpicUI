package ll018_bad

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks/feedback"
)

type BadFacet struct {
	facet.Facet
}

func newBadFacet() *BadFacet {
	b := &BadFacet{}
	dialog := feedback.NewDialog("Title", "Message", nil)
	_ = dialog
	return b
}
