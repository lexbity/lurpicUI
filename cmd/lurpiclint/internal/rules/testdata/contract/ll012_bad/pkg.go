package ll012_bad

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/store"
)

type BadFacet struct {
	facet.Facet
	Items []store.CollectionStore
}
