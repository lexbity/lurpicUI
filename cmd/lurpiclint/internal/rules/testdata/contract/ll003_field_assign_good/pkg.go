package ll003_field_assign_good

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
)

type GoodContainer struct {
	facet.Facet
	col *layout.ColumnLayout
}

func newGoodContainer() *GoodContainer {
	c := &GoodContainer{}
	c.Facet = facet.NewFacet()
	c.col = layout.NewColumnLayout()
	c.Facet.AddChild(c.col.Base())
	return c
}
