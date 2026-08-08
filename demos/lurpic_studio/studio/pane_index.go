package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// ExhibitIndex is the exhibit-index pane: a nav_rail (icon switch) above a
// tree_navigator (concept groups). Both bind the same ShellState.ActiveExhibit
// store, so the two controls stay in sync and the stage switches (FR-nav).
// It is a bespoke vertical host because the framework Card does not attach its
// content to the facet tree (F-card-content) — nav_rail/tree_navigator inside
// a Card would not receive pointer input.
type ExhibitIndex struct {
	facet.Facet
	layout facet.LayoutRole

	shell  *ShellState
	rail   *navigation.NavRail
	tree   *navigation.TreeNavigator
	railID *store.ValueStore[int]

	rt      facet.RuntimeServices
	cleanup func()
}

// NewExhibitIndex builds the index pane over the shared shell state.
func NewExhibitIndex(shell *ShellState) *ExhibitIndex {
	p := &ExhibitIndex{
		shell:  shell,
		railID: store.NewValueStore(exhibitIndex(shell.ActiveExhibit.Get())),
	}
	p.Facet = facet.NewFacet()

	items := make([]navigation.NavRailItem, 0, len(exhibitCatalog))
	for _, e := range exhibitCatalog {
		items = append(items, navigation.NavRailItem{Key: string(e.id), Label: e.title, IconRef: e.icon})
	}
	p.rail = navigation.NewNavRail("Exhibits", items, p.railID)

	p.tree = navigation.NewTreeNavigator("Exhibit tree", indexTreeNodes(shell.ActiveExhibit.Get()))

	p.AddChild(p.rail.Base()) //lurpiclint:ignore LL021 -- the index pane hosts navigational marks as regular children, not overlays (LL021 over-fires)
	p.AddChild(p.tree.Base()) //lurpiclint:ignore LL021 -- the index pane hosts navigational marks as regular children, not overlays (LL021 over-fires)

	p.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke index-pane host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return p.measure(ctx, c)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			p.arrange(ctx, bounds)
		},
	}
	p.layout.Child = linearChildContract(facet.StretchPolicy{
		Width:  facet.StretchAlways,
		Height: facet.StretchAlways,
	})
	p.AddRole(&p.layout)
	return p
}

// indexTreeNodes builds the tree_navigator's concept-grouped nodes. Each leaf
// key is the exhibit id; the initially active exhibit is marked selected.
func indexTreeNodes(active ExhibitID) []navigation.TreeNode {
	grouped := make(map[string][]navigation.TreeNode)
	order := make([]string, 0)
	for _, e := range exhibitCatalog {
		leaf := navigation.TreeNode{Key: string(e.id), Label: e.title, Selected: e.id == active}
		if _, seen := grouped[e.group]; !seen {
			order = append(order, e.group)
		}
		grouped[e.group] = append(grouped[e.group], leaf)
	}
	nodes := make([]navigation.TreeNode, 0, len(order))
	for _, g := range order {
		nodes = append(nodes, navigation.TreeNode{Key: g, Label: g, Expanded: true, Children: grouped[g]})
	}
	return nodes
}

// selectTreePath marks the node with the given key path selected (the exhibit
// key is the leaf key).
func selectTreePath(nodes []navigation.TreeNode, key string) {
	clearTreeSelection(nodes)
	markTreePath(nodes, key)
}

func markTreePath(nodes []navigation.TreeNode, key string) bool {
	for i := range nodes {
		if nodes[i].Key == key {
			nodes[i].Selected = true
			return true
		}
		if len(nodes[i].Children) > 0 && markTreePath(nodes[i].Children, key) {
			return true
		}
	}
	return false
}

func clearTreeSelection(nodes []navigation.TreeNode) {
	for i := range nodes {
		nodes[i].Selected = false
		clearTreeSelection(nodes[i].Children)
	}
}

// selectedTreeNode returns the first selected leaf key in the tree data.
func selectedTreeNode(nodes []navigation.TreeNode) string {
	for i := range nodes {
		if nodes[i].Selected {
			return nodes[i].Key
		}
		if k := selectedTreeNode(nodes[i].Children); k != "" {
			return k
		}
	}
	return ""
}

func (p *ExhibitIndex) measure(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
	if role := p.rail.Base().LayoutRole(); role != nil {
		role.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
	}
	if role := p.tree.Base().LayoutRole(); role != nil {
		role.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: c.MaxSize.W}})
	}
	return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
}

func (p *ExhibitIndex) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if bounds.IsEmpty() {
		if role := p.rail.Base().LayoutRole(); role != nil {
			role.Arrange(ctx, gfx.Rect{})
		}
		if role := p.tree.Base().LayoutRole(); role != nil {
			role.Arrange(ctx, gfx.Rect{})
		}
		return
	}
	railH := p.rail.Base().LayoutRole().MeasuredSize.H
	p.rail.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), railH))
	treeH := bounds.Height() - railH
	if treeH < 1 {
		treeH = 1
	}
	p.tree.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y+railH, bounds.Width(), treeH))
}

func (p *ExhibitIndex) OnAttach(ctx facet.AttachContext) {
	p.rt = ctx.Runtime

	railID := p.rail.Activated.Subscribe(func(index int) {
		if index >= 0 && index < len(exhibitCatalog) {
			p.setActive(exhibitCatalog[index].id, "index.nav_rail")
		}
	})
	activeID := p.shell.ActiveExhibit.OnChange.Subscribe(func(c signal.Change[ExhibitID]) {
		p.syncFromActive(c.New)
	})
	treeID := p.tree.Data.OnChange.Subscribe(func(signal.Change[[]navigation.TreeNode]) {
		if k := selectedTreeNode(p.tree.Data.Get()); k != "" {
			p.setActive(ExhibitID(k), "index.tree_navigator")
		}
	})
	p.cleanup = func() {
		p.rail.Activated.Unsubscribe(railID)
		p.shell.ActiveExhibit.OnChange.Unsubscribe(activeID)
		p.tree.Data.OnChange.Unsubscribe(treeID)
	}
}

func (p *ExhibitIndex) OnDetach() {
	if p.cleanup != nil {
		p.cleanup()
		p.cleanup = nil
	}
}

// setActive writes the shell's ActiveExhibit store (guarded against re-entry).
func (p *ExhibitIndex) setActive(id ExhibitID, source string) {
	if p.shell.ActiveExhibit.Get() == id {
		return
	}
	invalidateLayout(p, p.rt, source)
	p.shell.ActiveExhibit.Set(id)
}

// cloneTreeNodes deep-copies a tree node forest (TreeNode.Children is a slice,
// so a shallow append would share child nodes and corrupt the store's data).
func cloneTreeNodes(nodes []navigation.TreeNode) []navigation.TreeNode {
	out := make([]navigation.TreeNode, len(nodes))
	for i, n := range nodes {
		out[i] = n
		out[i].Children = cloneTreeNodes(n.Children)
	}
	return out
}

// syncFromActive reflects an externally-driven ActiveExhibit change in the
// nav_rail's index and the tree's selection.
func (p *ExhibitIndex) syncFromActive(id ExhibitID) {
	if idx := exhibitIndex(id); idx >= 0 && p.railID.Get() != idx {
		p.railID.Set(idx)
	}
	nodes := cloneTreeNodes(p.tree.Data.Get())
	if !selectTreePathFor(nodes, string(id)) {
		return
	}
	p.tree.Data.Set(nodes)
}

// selectTreePathFor clones-and-selects the given exhibit key, reporting
// whether the selection actually changed (guards the ActiveExhibit loop).
func selectTreePathFor(nodes []navigation.TreeNode, key string) bool {
	current := selectedTreeNode(nodes)
	selectTreePath(nodes, key)
	return selectedTreeNode(nodes) != current
}

// Rail returns the nav_rail mark.
func (p *ExhibitIndex) Rail() *navigation.NavRail { return p.rail }

// Tree returns the tree_navigator mark.
func (p *ExhibitIndex) Tree() *navigation.TreeNavigator { return p.tree }

// RailIndex returns the nav_rail's active-index store.
func (p *ExhibitIndex) RailIndex() *store.ValueStore[int] { return p.railID }

func (p *ExhibitIndex) Base() *facet.Facet { p.BindImpl(p); return &p.Facet }
func (p *ExhibitIndex) OnActivate()        {}
func (p *ExhibitIndex) OnDeactivate()      {}
