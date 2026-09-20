package studio

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// ExhibitIndex is the exhibit-index pane: exactly ONE exhibit-selection
// surface — a tree_navigator of concept-grouped exhibits (FR-13). The nav_rail
// duplicate listing was removed; the nav_rail mark is demonstrated in the E6
// Navigation playground (coverage stays whole). The tree binds the shell's
// ActiveExhibit through its FR-8 selection store (treeSel), so a store write —
// from the tree, the stage, the command palette, or a pre-attach seed —
// re-syncs the control through the mark's own contract. It is a bespoke
// vertical host because the framework Card does not attach its content to the
// facet tree (F-card-content) — a tree_navigator inside a Card would not
// receive pointer input.
type ExhibitIndex struct {
	facet.Facet
	layout facet.LayoutRole

	shell   *ShellState
	tree    *navigation.TreeNavigator
	treeSel *store.ValueStore[string]

	rt      facet.RuntimeServices
	cleanup func()
}

// NewExhibitIndex builds the index pane over the shared shell state. The tree
// mark binds the shell's ActiveExhibit through its FR-8 selection store
// (treeSel), so a store write re-syncs the tree through the mark's own
// contract.
func NewExhibitIndex(shell *ShellState) *ExhibitIndex {
	p := &ExhibitIndex{
		shell:   shell,
		treeSel: store.NewValueStore(selectedPathForExhibit(shell.ActiveExhibit.Get())),
	}
	p.Facet = facet.NewFacet()

	p.tree = navigation.NewTreeNavigator("Exhibit tree", indexTreeNodes(shell.ActiveExhibit.Get()), p.treeSel)

	p.AddChild(p.tree.Base()) //lurpiclint:ignore LL021 -- the index pane hosts a navigational mark as a regular child, not an overlay (LL021 over-fires)

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

// The per-app selection plumbing FR-8 obsoleted (a build-time snapshot plus
// manual Data mutation on every change) now lives inside the tree_navigator
// mark's own contract: attach-time sync, re-sync on every store change, and
// write-back with an echo guard. The index pane binds the mark to ActiveExhibit
// via selectedPathForExhibit / exhibitFromPath above.

func (p *ExhibitIndex) measure(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
	if role := p.tree.Base().LayoutRole(); role != nil {
		role.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: c.MaxSize.W}})
	}
	return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
}

func (p *ExhibitIndex) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if bounds.IsEmpty() {
		if role := p.tree.Base().LayoutRole(); role != nil {
			role.Arrange(ctx, gfx.Rect{})
		}
		return
	}
	p.tree.Base().LayoutRole().Arrange(ctx, bounds)
}

func (p *ExhibitIndex) OnAttach(ctx facet.AttachContext) {
	p.rt = ctx.Runtime

	// A tree user selection publishes its path to treeSel → the exhibit switch.
	treeSelID := p.treeSel.OnChange.Subscribe(func(c signal.Change[string]) {
		if id := exhibitFromPath(c.New); id != "" {
			p.setActive(id, "index.tree_navigator")
		}
	})
	// An external ActiveExhibit write (stage, pre-attach seed, command palette)
	// re-syncs the tree mark's FR-8 selection store; its binding adopts the
	// write and re-renders. No manual Data mutation or layout routing.
	activeID := p.shell.ActiveExhibit.OnChange.Subscribe(func(c signal.Change[ExhibitID]) {
		if path := selectedPathForExhibit(c.New); path != "" && p.treeSel.Get() != path {
			p.treeSel.Set(path)
		}
	})
	p.cleanup = func() {
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
// through the tree mark's FR-8 selection binding, re-syncs the tree selection
// store — its version is tracked in the projection cache key and re-projected
// on change (RX-1 FR-3).
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

// Tree returns the tree_navigator mark.
func (p *ExhibitIndex) Tree() *navigation.TreeNavigator { return p.tree }

// TreeSelection returns the tree_navigator's selection (path) store.
func (p *ExhibitIndex) TreeSelection() *store.ValueStore[string] { return p.treeSel }

func (p *ExhibitIndex) Base() *facet.Facet { p.BindImpl(p); return &p.Facet }
func (p *ExhibitIndex) OnActivate()        {}
func (p *ExhibitIndex) OnDeactivate()      {}
