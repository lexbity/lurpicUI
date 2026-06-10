package studio

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/status"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/signal"
	storepkg "codeburg.org/lexbit/lurpicui/store"
)

type SourcesPanel struct {
	facet.Facet
	layout      facet.LayoutRole
	navRail     *navigation.NavRail
	scrollRegion *structure.ScrollRegion
	treeNav     *navigation.TreeNavigator
	listItems   []*selection.ListItem
	statusLight *status.StatusLight
	badge       *status.Badge
	appState    *state.AppState
	connectionSub signal.SubscriptionID
}

func newSourcesTreeNodes(rows []dataset.Row) []navigation.TreeNode {
	regionSet := make(map[string]bool)
	var regions []string
	for _, r := range rows {
		if !regionSet[r.Region] {
			regionSet[r.Region] = true
			regions = append(regions, r.Region)
		}
	}
	nodes := make([]navigation.TreeNode, 0, len(regions))
	for _, reg := range regions {
		var count int
		for _, r := range rows {
			if r.Region == reg {
				count++
			}
		}
		nodes = append(nodes, navigation.TreeNode{
			Key:      reg,
			Label:    regionLabel(reg),
			IconRef:  "folder",
			Expanded: false,
			Children: []navigation.TreeNode{
				{Key: reg, Label: fmt.Sprintf("%s (%d rows)", regionLabel(reg), count), IconRef: "description"},
			},
		})
	}
	return nodes
}

func regionLabel(region string) string {
	switch region {
	case "NA":
		return "North America"
	case "EU":
		return "Europe"
	case "APAC":
		return "APAC"
	case "LATAM":
		return "LATAM"
	default:
		return region
	}
}

func NewSourcesPanel(appState *state.AppState) *SourcesPanel {
	p := &SourcesPanel{appState: appState}
	p.Facet = facet.NewFacet()

	p.navRail = navigation.NewNavRail("Sources Nav", []navigation.NavRailItem{
		{Key: "sources", Label: "Sources", IconRef: "folder_open"},
		{Key: "filter", Label: "Filter", IconRef: "filter_list"},
		{Key: "search", Label: "Search", IconRef: "search"},
	})
	p.navRail.ActiveIndex = marks.Const(0)

	allRows := appState.Rows.All()
	treeData := newSourcesTreeNodes(allRows)
	p.treeNav = navigation.NewTreeNavigator("Source Tree", treeData)

	regionRowCount := map[string]int{}
	for _, r := range allRows {
		regionRowCount[r.Region]++
	}

	p.listItems = make([]*selection.ListItem, 0, 4)
	for _, reg := range []string{"NA", "EU", "APAC", "LATAM"} {
		item := selection.NewListItem(marks.Const(regionLabel(reg)))
		item.SupportingText = marks.Const(fmt.Sprintf("%d rows", regionRowCount[reg]))
		item.Activated.Subscribe(func(signal.Unit) {
			appState.SelectedSource.Set(reg)
		})
		p.listItems = append(p.listItems, item)
	}

	p.statusLight = status.NewStatusLight("Disconnected")
	p.badge = status.NewBadge("0")

	p.scrollRegion = structure.NewScrollRegion("Sources")
	treeChild := structure.ScrollRegionChild{Facet: p.treeNav, MarkID: 2, Placement: facet.Placement{Mode: facet.PlacementGrid}}
	listChildren := make([]structure.ScrollRegionChild, 0, len(p.listItems)+2)
	listChildren = append(listChildren, treeChild)
	for _, li := range p.listItems {
		listChildren = append(listChildren, structure.ScrollRegionChild{
			Facet:  li,
			MarkID: 3,
			Placement: facet.Placement{Mode: facet.PlacementGrid},
		})
	}
	p.scrollRegion.SetChildren(listChildren)

	p.Facet.AddChild(p.navRail.Base())
	p.Facet.AddChild(p.scrollRegion.Base())
	for _, li := range p.listItems {
		p.scrollRegion.Base().AddChild(li.Base())
	}
	p.scrollRegion.Base().AddChild(p.treeNav.Base())

	p.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
			width := constraints.MaxSize.W
			if width <= 0 {
				width = 200
			}
			height := constraints.MaxSize.H
			if height <= 0 {
				height = 600
			}

			p.navRail.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: width, H: 48}})
			navH := p.navRail.Base().LayoutRole().MeasuredSize.H
			if navH <= 0 {
				navH = 48
			}

			p.scrollRegion.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: width, H: height - navH}})

			treeH := height - navH - 80
			if treeH < 40 {
				treeH = 40
			}
			p.treeNav.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: width - 8, H: treeH}})
			for _, li := range p.listItems {
				li.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: width - 8, H: 40}})
			}

			return facet.MeasureResult{Size: gfx.Size{W: width, H: height}}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			width := bounds.Width()
			if width <= 0 {
				width = 200
			}

			navH := p.navRail.Base().LayoutRole().MeasuredSize.H
			if navH <= 0 {
				navH = 48
			}
			p.navRail.Base().LayoutRole().OnArrange(facet.ArrangeContext{}, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, width, navH))
			p.navRail.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, width, navH)

			scrollY := bounds.Min.Y + navH
			scrollH := bounds.Height() - navH
			p.scrollRegion.Base().LayoutRole().OnArrange(facet.ArrangeContext{}, gfx.RectFromXYWH(bounds.Min.X, scrollY, width, scrollH))
			p.scrollRegion.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(bounds.Min.X, scrollY, width, scrollH)

			treeH := p.treeNav.Base().LayoutRole().MeasuredSize.H
			if treeH <= 0 {
				treeH = scrollH - 160
			}
			if treeH > scrollH-80 {
				treeH = scrollH - 80
			}
			p.treeNav.Base().LayoutRole().OnArrange(facet.ArrangeContext{}, gfx.RectFromXYWH(bounds.Min.X+4, scrollY, width-8, treeH))
			p.treeNav.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(bounds.Min.X+4, scrollY, width-8, treeH)

			itemY := scrollY + treeH + 4
			for _, li := range p.listItems {
				li.Base().LayoutRole().OnArrange(facet.ArrangeContext{}, gfx.RectFromXYWH(bounds.Min.X+4, itemY, width-8, 36))
				li.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(bounds.Min.X+4, itemY, width-8, 36)
				itemY += 38
			}
		},
		Child: facet.GroupChildContract{
			SupportedPlacement: facet.SupportsFree | facet.SupportsGrid | facet.SupportsLinear,
			Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
				return facet.IntrinsicSize{
					Min:       gfx.Size{W: 200, H: 100},
					Preferred: gfx.Size{W: constraints.MaxSize.W, H: constraints.MaxSize.H},
					Max:       gfx.Size{W: constraints.MaxSize.W, H: constraints.MaxSize.H},
				}
			},
			Stretch: facet.StretchPolicy{Width: facet.StretchAlways, Height: facet.StretchAlways},
		},
	}
	p.AddRole(&p.layout)

	p.updateConnectedState()
	p.connectionSub = appState.Connection.OnChange.Subscribe(func(c signal.Change[state.ConnState]) {
		p.onConnectionChanged(c.New)
	})

	return p
}

func (p *SourcesPanel) Base() *facet.Facet { p.Facet.BindImpl(p); return &p.Facet }
func (p *SourcesPanel) OnAttach(ctx facet.AttachContext)  {}
func (p *SourcesPanel) OnDetach()                         { p.appState.Connection.OnChange.Unsubscribe(p.connectionSub) }
func (p *SourcesPanel) OnActivate()                       {}
func (p *SourcesPanel) OnDeactivate()                     {}

func (p *SourcesPanel) onConnectionChanged(cs state.ConnState) {
	switch cs {
	case state.ConnConnected:
		p.statusLight.Label = marks.Const("Connected")
	case state.ConnConnecting:
		p.statusLight.Label = marks.Const("Connecting...")
	default:
		p.statusLight.Label = marks.Const("Disconnected")
	}
	p.statusLight.Base().Invalidate(facet.DirtyProjection)
}

func (p *SourcesPanel) updateConnectedState() {
	p.onConnectionChanged(p.appState.Connection.Get())
}

func (p *SourcesPanel) NavRail() *navigation.NavRail       { return p.navRail }
func (p *SourcesPanel) TreeNav() *navigation.TreeNavigator { return p.treeNav }
func (p *SourcesPanel) ListItems() []*selection.ListItem   { return p.listItems }
func (p *SourcesPanel) StatusLight() *status.StatusLight   { return p.statusLight }
func (p *SourcesPanel) Badge() *status.Badge               { return p.badge }

func (p *SourcesPanel) UpdateBadge(selectedSource string) {
	var rowCount int
	if selectedSource != "" {
		for _, r := range p.appState.Rows.All() {
			if r.Region == selectedSource {
				rowCount++
			}
		}
	}
	p.badge.Label = marks.Const(fmt.Sprintf("%d", rowCount))
	p.badge.Base().Invalidate(facet.DirtyProjection)
}

func (p *SourcesPanel) UpdateSelection(selectedSource string) {
	for _, li := range p.listItems {
		label := li.Label.Get()
		isSelected := false
		switch selectedSource {
		case "NA":
			isSelected = label == "North America"
		case "EU":
			isSelected = label == "Europe"
		case "APAC":
			isSelected = label == "APAC"
		case "LATAM":
			isSelected = label == "LATAM"
		}
		li.Selected = marks.Const(isSelected)
		li.Active = marks.Const(isSelected)
		li.Base().Invalidate(facet.DirtyProjection)
	}
}

func (p *SourcesPanel) ConnectStore() *storepkg.ValueStore[string] {
	return p.appState.SelectedSource
}
