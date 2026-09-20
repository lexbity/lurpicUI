package studio

import (
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// playNavFamily is the Navigation playground: nav_rail, nav_drawer,
// tree_navigator, pagination, and breadcrumbs (the family-switching tabs
// themselves are the exhibit's host; see e6_playground.go). The rail item
// click, drawer item click, tree node select, page click, and crumb click each
// land in a store (the navigation family's distinctive behavior:
// structure-driven navigation landing in current/state stores). The nav_rail
// mark is demonstrated here since RX-1 FR-13 removed the index pane's duplicate
// rail listing (the wide index hosts the tree_navigator alone).
type playNavFamily struct {
	scroll *demoList

	rail       *navigation.NavRail
	railActive *store.ValueStore[int]
	railSelect *store.ValueStore[int]

	drawer     *navigation.NavDrawer
	drawerOpen *store.ValueStore[bool]
	current    *store.ValueStore[int]
	lastItem   *store.ValueStore[int]

	tree *navigation.TreeNavigator

	pager         *navigation.Pagination
	pageIndex     *store.ValueStore[int]
	pageActivated *store.ValueStore[int]

	crumbs         *navigation.Breadcrumbs
	crumbIndex     *store.ValueStore[int]
	crumbActivated *store.ValueStore[int]
}

// newPlayNavFamily builds the Navigation family playground.
func newPlayNavFamily() *playNavFamily {
	f := &playNavFamily{
		railActive:     store.NewValueStore(0),
		railSelect:     store.NewValueStore(-1),
		drawerOpen:     store.NewValueStore(true),
		current:        store.NewValueStore(0),
		lastItem:       store.NewValueStore(-1),
		pageIndex:      store.NewValueStore(1),
		pageActivated:  store.NewValueStore(-1),
		crumbIndex:     store.NewValueStore(0),
		crumbActivated: store.NewValueStore(-1),
	}

	f.rail = navigation.NewNavRail("Catalog groups", []navigation.NavRailItem{
		{Key: "catalog", Label: "Catalog", IconRef: iconCapabilities},
		{Key: "realtime", Label: "Realtime", IconRef: iconRealtime},
		{Key: "shell", Label: "Shell", IconRef: iconLayers},
	}, f.railActive)

	f.drawer = navigation.NewNavDrawer("Sources", []navigation.NavDrawerSection{
		{Label: "Enterprise", Items: []navigation.NavDrawerItem{
			{Key: "sources", Label: "Sources"},
			{Key: "hub", Label: "Hub"},
			{Key: "streams", Label: "Streams"},
		}},
		{Label: "Developer", Items: []navigation.NavDrawerItem{
			{Key: "pipelines", Label: "Pipelines"},
			{Key: "console", Label: "Console"},
		}},
	}, f.drawerOpen, f.current)
	f.drawer.Subtitle = marks.Const("lurpic studio places")

	f.tree = navigation.NewTreeNavigator("Families", []navigation.TreeNode{
		{
			Key:   "action",
			Label: "Action",
			Children: []navigation.TreeNode{
				{Key: "action/bar", Label: "action_bar"},
				{Key: "action/ribbon", Label: "ribbon"},
			},
		},
		{
			Key:   "selection",
			Label: "Selection",
			Children: []navigation.TreeNode{
				{Key: "selection/checkbox", Label: "checkbox"},
				{Key: "selection/slider", Label: "slider"},
			},
		},
		{
			Key:   "status",
			Label: "Status",
		},
	}, nil)

	f.pager = navigation.NewPagination("Pages", []navigation.PaginationItem{
		{Key: "1", Label: "1"},
		{Key: "2", Label: "2"},
		{Key: "3", Label: "3"},
		{Key: "4", Label: "4"},
	}, f.pageIndex)

	f.crumbs = navigation.NewBreadcrumbs("Path", []navigation.BreadcrumbItem{
		{Label: "Lurpic Studio"},
		{Label: "Exhibits"},
		{Label: "Realtime Data"},
	}, f.crumbIndex)

	f.scroll = newDemoList(listGap,
		playgroundCard("nav_rail — click a destination", f.rail),
		playgroundCard("nav_drawer — click a destination", f.drawer),
		playgroundCard("tree_navigator — click a family", f.tree),
		playgroundCard("pagination — flip pages", f.pager),
		playgroundCard("breadcrumbs — trace the path", f.crumbs),
	)
	return f
}

// wire subscribes the navigation family's activations.
func (f *playNavFamily) wire() func() {
	railSubID := f.rail.Activated.Subscribe(func(index int) {
		f.railSelect.Set(index)
	})
	drawerID := f.drawer.Activated.Subscribe(func(index int) {
		f.lastItem.Set(index)
		f.drawerOpen.Set(false)
	})
	pagerID := f.pager.Activated.Subscribe(func(index int) {
		f.pageActivated.Set(index)
	})
	pageID := f.pageIndex.OnChange.Subscribe(func(signal.Change[int]) {
		f.pageActivated.Set(f.pageIndex.Get())
	})
	crumbID := f.crumbs.Activated.Subscribe(func(index int) {
		f.crumbActivated.Set(index)
		f.crumbIndex.Set(index)
	})
	return func() {
		f.rail.Activated.Unsubscribe(railSubID)
		f.drawer.Activated.Unsubscribe(drawerID)
		f.pager.Activated.Unsubscribe(pagerID)
		f.pageIndex.OnChange.Unsubscribe(pageID)
		f.crumbs.Activated.Unsubscribe(crumbID)
	}
}

// Rail returns the nav_rail mark.
func (f *playNavFamily) Rail() *navigation.NavRail { return f.rail }

// RailSelect returns the nav_rail activation store.
func (f *playNavFamily) RailSelect() *store.ValueStore[int] { return f.railSelect }

// RailActive returns the nav_rail active-index store.
func (f *playNavFamily) RailActive() *store.ValueStore[int] { return f.railActive }

// CrumbActivated returns the breadcrumb activation store.
func (f *playNavFamily) CrumbActivated() *store.ValueStore[int] { return f.crumbActivated }

// Crumbs returns the breadcrumbs mark.
func (f *playNavFamily) Crumbs() *navigation.Breadcrumbs { return f.crumbs }
