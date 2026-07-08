package studio

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/signal"
)

const iconGlobe = "globe"

type sourcesPanel struct {
	col    *layout.ColumnLayout
	rail   *navigation.NavRail
	tree   *navigation.TreeNavigator
	scroll *structure.ScrollRegion
	list   *structure.List
}

func newSourcesPanel(as *state.AppState) *sourcesPanel {
	sp := &sourcesPanel{}

	sp.rail = navigation.NewNavRail(labelSources, []navigation.NavRailItem{
		{Key: "sources", Label: labelSources, IconRef: "database"},
		{Key: "favorites", Label: "Favorites", IconRef: "star"},
		{Key: "settings", Label: "Settings", IconRef: "settings"},
	})
	sp.rail.ActiveIndex = marks.Const(0)

	sp.tree = navigation.NewTreeNavigator("Regions", []navigation.TreeNode{
		{
			Key: "regions", Label: "Regions", Expanded: true,
			Children: []navigation.TreeNode{
				{Key: "NA", Label: "North America"},
				{Key: "EU", Label: "Europe"},
				{Key: "APAC", Label: "Asia Pacific"},
				{Key: labelLATAM, Label: "Latin America"},
			},
		},
	})

	sp.list = structure.NewList("Region Details", []structure.ListEntry{
		{Key: "NA", Label: "NA", SupportingText: "", LeadingIconRef: iconGlobe},
		{Key: "EU", Label: "EU", SupportingText: "", LeadingIconRef: iconGlobe},
		{Key: "APAC", Label: "APAC", SupportingText: "", LeadingIconRef: iconGlobe},
		{Key: labelLATAM, Label: labelLATAM, SupportingText: "", LeadingIconRef: iconGlobe},
	})

	sp.scroll = structure.NewScrollRegion("Sources scroll")
	sp.scroll.SetChildren([]structure.ScrollRegionChild{
		{Facet: sp.tree},
		{Facet: sp.list},
	})

	sp.col = layout.NewColumnLayout()
	sp.col.Gap = 0
	sp.col.Add(layout.Fixed(sp.rail))
	sp.col.Add(layout.Flexible(sp.scroll, 1))

	wireSourcesTreeSelection(as, sp.tree)
	wireSourceRowCounts(as, sp.list)

	return sp
}

func wireSourcesTreeSelection(as *state.AppState, tree *navigation.TreeNavigator) {
	tree.Data.OnChange.Subscribe(func(c signal.Change[[]navigation.TreeNode]) {
		selected := findSelectedTreeKey(c.New)
		if selected != "" {
			as.SelectedSource.Set(selected)
		}
	})
}

func findSelectedTreeKey(nodes []navigation.TreeNode) string {
	for _, n := range nodes {
		if n.Selected {
			return n.Key
		}
		if k := findSelectedTreeKey(n.Children); k != "" {
			return k
		}
	}
	return ""
}

func wireSourceRowCounts(as *state.AppState, list *structure.List) {
	as.SelectedSource.OnChange.Subscribe(func(c signal.Change[string]) {
		entries := list.Data.Get()
		for i := range entries {
			if entries[i].Key == c.New {
				entries[i].Selected = true
			} else {
				entries[i].Selected = false
			}
			count := countRowsForSource(as, entries[i].Key)
			if count > 0 {
				entries[i].SupportingText = fmt.Sprintf("%d rows", count)
			} else {
				entries[i].SupportingText = ""
			}
		}
		list.Data.Set(entries)
	})
}

func countRowsForSource(as *state.AppState, source string) int {
	all := as.Rows.All()
	if source == "" {
		return len(all)
	}
	count := 0
	for _, r := range all {
		if r.Region == source {
			count++
		}
	}
	return count
}
