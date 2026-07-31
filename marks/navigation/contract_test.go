package navigation

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestNavDrawer_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl {
			return NewNavDrawer("label", nil, store.NewValueStore(false), store.NewValueStore(0))
		},
		func(facet.FacetImpl) {},
	)
}
