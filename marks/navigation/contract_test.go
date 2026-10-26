package navigation

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestNavDrawer_contract_group_children(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl {
			return NewNavDrawer(scope, "label", nil, store.NewValueStore(scope, false), store.NewValueStore(scope, 0))
		},
		func(facet.FacetImpl) {},
	)
}
