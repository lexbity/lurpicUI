package studio

import (
	"strings"

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

	shell   *ShellState
	rail    *navigation.NavRail
	tree    *navigation.TreeNavigator
	railID  *store.ValueStore[int]
	treeSel *store.ValueStore[string]

	rt      facet.RuntimeServices
	cleanup func()
}

// NewExhibitIndex builds the index pane over the shared shell state. The nav
// marks bind the shell's ActiveExhibit through their FR-8 selection stores
// (railID / treeSel), so a store write — from either mark, the stage, or a
// pre-attach seed — re-syncs both controls through the marks' own contract.
func NewExhibitIndex(shell *ShellState) *ExhibitIndex {
	p := &ExhibitIndex{
		shell:   shell,
		railID:  store.NewValueStore(exhibitIndex(shell.ActiveExhibit.Get())),
		treeSel: store.NewValueStore(selectedPathForExhibit(shell.ActiveExhibit.Get())),
	}
	p.Facet = facet.NewFacet()

	items := make([]navigation.NavRailItem, 0, len(exhibitCatalog))
	for _, e := range exhibitCatalog {
		items = append(items, navigation.NavRailItem{Key: string(e.id), Label: e.title, IconRef: e.icon})
	}
	p.rail = navigation.NewNavRail("Exhibits", items, p.railID)

	p.tree = navigation.NewTreeNavigator("Exhibit tree", indexTreeNodes(shell.ActiveExhibit.Get()), p.treeSel)

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

// selectTreePath, markTreePath, clearTreeSelection and selectedTreeNode were
// the per-app selection plumbing FR-8 obsoleted: the tree_navigator now owns
// its selection through its FR-8 Selection store, and the index pane binds it
// to ActiveExhibit via selectedPathForExhibit / exhibitFromPath above.

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

	// A rail user selection publishes to railID → the exhibit switch flows
	// through setActive (the rail's own binding re-syncs it).
	railID := p.rail.Activated.Subscribe(func(index int) {
		if index >= 0 && index < len(exhibitCatalog) {
			p.setActive(exhibitCatalog[index].id, "index.nav_rail")
		}
	})
	// A tree user selection publishes its path to treeSel → the exhibit switch.
	treeSelID := p.treeSel.OnChange.Subscribe(func(c signal.Change[string]) {
		if id := exhibitFromPath(c.New); id != "" {
			p.setActive(id, "index.tree_navigator")
		}
	})
	// An external ActiveExhibit write (stage, pre-attach seed, command palette)
	// re-syncs both marks' FR-8 selection stores; their bindings adopt the
	// write and re-render. No manual Data mutation or layout routing.
	activeID := p.shell.ActiveExhibit.OnChange.Subscribe(func(c signal.Change[ExhibitID]) {
		if idx := exhibitIndex(c.New); idx >= 0 && p.railID.Get() != idx {
			p.railID.Set(idx)
		}
		if path := selectedPathForExhibit(c.New); path != "" && p.treeSel.Get() != path {
			p.treeSel.Set(path)
		}
	})
	p.cleanup = func() {
		p.rail.Activated.Unsubscribe(railID)
		p.treeSel.OnChange.Unsubscribe(treeSelID)
		p.shell.ActiveExhibit.OnChange.Unsubscribe(activeID)
	}
}

func (p *ExhibitIndex) OnDetach() {
	if p.cleanup != nil {
		p.cleanup()
		p.cleanup = nil
	}
}

// setActive writes the shell's ActiveExhibit store (guarded against re-entry).
// No manual layout routing: the store write re-lays the stage (structural) and,
// through the marks' FR-8 selection bindings, re-syncs the nav_rail / tree
// selection stores — their versions are tracked in the projection cache key and
// re-projected on change (RX-1 FR-3).
func (p *ExhibitIndex) setActive(id ExhibitID, source string) {
	if p.shell.ActiveExhibit.Get() == id {
		return
	}
	p.shell.ActiveExhibit.Set(id)
}

// selectedPathForExhibit returns the tree_navigator selection path
// (groupKey/leafKey) for an exhibit id, or "" when the id is unknown.
func selectedPathForExhibit(id ExhibitID) string {
	for _, e := range exhibitCatalog {
		if e.id == id {
			return e.group + "/" + string(e.id)
		}
	}
	return ""
}

// exhibitFromPath extracts the exhibit id (the leaf path segment) from a tree
// selection path.
func exhibitFromPath(path string) ExhibitID {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return ExhibitID(path[idx+1:])
	}
	return ExhibitID(path)
}

// Rail returns the nav_rail mark.
func (p *ExhibitIndex) Rail() *navigation.NavRail { return p.rail }

// Tree returns the tree_navigator mark.
func (p *ExhibitIndex) Tree() *navigation.TreeNavigator { return p.tree }

// RailIndex returns the nav_rail's active-index store.
func (p *ExhibitIndex) RailIndex() *store.ValueStore[int] { return p.railID }

// TreeSelection returns the tree_navigator's selection (path) store.
func (p *ExhibitIndex) TreeSelection() *store.ValueStore[string] { return p.treeSel }

func (p *ExhibitIndex) Base() *facet.Facet { p.BindImpl(p); return &p.Facet }
func (p *ExhibitIndex) OnActivate()        {}
func (p *ExhibitIndex) OnDeactivate()      {}
