package action

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestActionBar_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl {
			return NewActionBar("label", []ActionBarAction{{Key: "a", Label: "Alpha"}})
		},
		func(facet.FacetImpl) {},
	)
}

func TestRibbon_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl {
			return NewRibbon("label", []RibbonSection{{Key: "home", Label: "Home"}})
		},
		func(facet.FacetImpl) {},
	)
}

func TestToolbar_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl {
			return NewToolbar(marks.Const("label"), []ToolbarGroup{
				{Key: "primary", Actions: []ActionGroupAction{{Key: "a", Label: "Alpha"}}},
			}, nil)
		},
		func(facet.FacetImpl) {},
	)
}

func TestCommandPalette_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl {
			return NewCommandPalette(marks.Const("label"), nil, store.NewValueStore(true))
		},
		func(facet.FacetImpl) {},
	)
}

func TestMenuButton_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl {
			m := NewMenuButton("label", []MenuButtonEntry{{Key: "a", Label: "Alpha"}})
			m.Open = true
			return m
		},
		func(facet.FacetImpl) {},
	)
}

func TestPopupPalette_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl {
			return NewPopupPalette("label", nil, store.NewValueStore(true))
		},
		func(facet.FacetImpl) {},
	)
}

func TestRadialMenu_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl {
			return NewRadialMenu("label", nil, nil)
		},
		func(facet.FacetImpl) {},
	)
}

func TestSplitButton_contract_group_children(t *testing.T) {
	contracttest.AssertGroupChildren(t,
		func() facet.FacetImpl {
			s := NewSplitButton("label", []SplitButtonItem{{Key: "a", Label: "Alpha"}})
			s.Open = true
			return s
		},
		func(facet.FacetImpl) {},
	)
}
