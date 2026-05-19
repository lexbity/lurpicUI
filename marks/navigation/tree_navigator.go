package navigation

import (
	"sort"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	gfxsvg "codeburg.org/lexbit/lurpicui/gfx/svg"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/platform"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uinav"
)

const (
	treeNavigatorMarkIDRoot               facet.MarkID = 1
	treeNavigatorMarkIDTree               facet.MarkID = 2
	treeNavigatorMarkIDTreeItem           facet.MarkID = 3
	treeNavigatorMarkIDDisclosure         facet.MarkID = 4
	treeNavigatorMarkIDIcon               facet.MarkID = 5
	treeNavigatorMarkIDLabel              facet.MarkID = 6
	treeNavigatorMarkIDSelectionIndicator facet.MarkID = 7
	treeNavigatorMarkIDFocusRing          facet.MarkID = 8
)

// TreeNode describes one generated navigation node.
type TreeNode struct {
	Key      string
	Label    string
	IconRef  string
	Selected bool
	Expanded bool
	Disabled bool
	Children []TreeNode
}

// TreeNavigator implements the navigation.tree_navigator canonical mark.
type TreeNavigator struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole
	textRole       facet.TextRole
	viewportRole   facet.ViewportRole

	Data *store.ValueStore[[]TreeNode]

	Label    string
	Disabled bool

	hoveredPath string
	pressedPath string
	focusedPath string

	focusedVisible   bool
	focusFromPointer bool
	scrollOffset     float32

	cachedTokens           theme.Tokens
	cachedRecipe           shared.TreeNavigatorSlots
	cachedRootBounds       gfx.Rect
	cachedTreeBounds       gfx.Rect
	cachedContentBounds    gfx.Rect
	cachedVisibleNodes     []treeNavigatorVisibleNode
	cachedRowFacets        map[string]*selection.ListItem
	cachedRowBounds        []gfx.Rect
	cachedRowLeadingBounds []gfx.Rect
	cachedRowLabelBounds   []gfx.Rect
	cachedRowDepths        []int
	cachedRowPaths         []string
	cachedRowHasChildren   []bool
	cachedRowSelection     []bool
	cachedRowGap           float32
	cachedRowIndent        float32
	cachedRowDisclosure    float32
	cachedWritingDirection facet.WritingDirection
	cachedContentHeight    float32
}

type treeNavigatorVisibleNode struct {
	Path        string
	Depth       int
	Node        TreeNode
	HasChildren bool
}

var _ facet.FacetImpl = (*TreeNavigator)(nil)
var _ layout.AnchorExporter = (*TreeNavigator)(nil)

// NewTreeNavigator constructs a navigation.tree_navigator mark with canonical defaults.
func NewTreeNavigator(label string, nodes []TreeNode) *TreeNavigator {
	t := &TreeNavigator{
		Facet: facet.NewFacet(),
		Label: label,
		Data:  store.NewValueStore[[]TreeNode](cloneTreeNodes(nodes)),
	}
	t.layoutRole.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   treeNavigatorGroupPolicy{tree: t},
		Children: t,
	}
	t.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := t.measureIntrinsic(ctx, constraints)
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionClip,
			BelowMinHeight: facet.CompressionClip,
			AboveMaxWidth:  facet.ExpansionClip,
			AboveMaxHeight: facet.ExpansionClip,
		},
		Stretch: facet.StretchPolicy{
			Width:  facet.StretchWhenParentRequests,
			Height: facet.StretchNever,
		},
		Baseline: facet.BaselineNone,
	}
	t.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return t.measure(ctx, constraints)
	}
	t.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		t.layoutRole.ArrangedBounds = bounds
		t.arrange(ctx, bounds)
	}
	t.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := t.buildCommands(bounds, nil)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	t.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := t.buildCommands(t.layoutRole.ArrangedBounds, ctx.Runtime)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	t.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult { return t.hitTest(p) }
	t.inputRole.OnPointer = func(e facet.PointerEvent) bool { return t.onPointer(e) }
	t.inputRole.OnScroll = func(e facet.ScrollEvent) bool { return t.onScroll(e) }
	t.inputRole.OnKey = func(e facet.KeyEvent) bool { return t.onKey(e) }
	t.focusRole.Focusable = func() bool { return !t.Disabled && len(t.cachedVisibleNodes) > 0 }
	t.focusRole.TabIndex = 0
	t.focusRole.OnFocusGained = func() { t.onFocusGained() }
	t.focusRole.OnFocusLost = func() { t.onFocusLost() }
	t.viewportRole.Transform = gfx.Identity()
	t.textRole.IMEEnabled = false
	t.AddRole(&t.layoutRole)
	t.AddRole(&t.renderRole)
	t.AddRole(&t.projectionRole)
	t.AddRole(&t.hitRole)
	t.AddRole(&t.inputRole)
	t.AddRole(&t.focusRole)
	t.AddRole(&t.textRole)
	t.AddRole(&t.viewportRole)
	return t
}

// Base satisfies facet.FacetImpl.
func (t *TreeNavigator) Base() *facet.Facet {
	t.Facet.BindImpl(t)
	return &t.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (t *TreeNavigator) AccessibilityRole() string { return "tree" }

// AccessibleName reports the semantic name source required by the spec.
func (t *TreeNavigator) AccessibleName() string {
	if t == nil {
		return ""
	}
	return t.Label
}

// SetLabel updates the authored label.
func (t *TreeNavigator) SetLabel(label string) {
	if t == nil || t.Label == label {
		return
	}
	t.Label = label
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetNodes updates the canonical tree data store.
func (t *TreeNavigator) SetNodes(nodes []TreeNode) {
	if t == nil {
		return
	}
	if t.Data == nil {
		t.Data = store.NewValueStore[[]TreeNode](cloneTreeNodes(nodes))
		t.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return
	}
	t.Data.Set(cloneTreeNodes(nodes))
}

// SetDisabled toggles disabled state.
func (t *TreeNavigator) SetDisabled(disabled bool) {
	if t == nil || t.Disabled == disabled {
		return
	}
	t.Disabled = disabled
	if disabled {
		t.hoveredPath = ""
		t.pressedPath = ""
		t.focusedVisible = false
		t.focusFromPointer = false
	}
	t.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// SetSelectedPath updates the selected node path in the store data.
func (t *TreeNavigator) SetSelectedPath(path string) {
	t.mutateTree(func(nodes []TreeNode) {
		clearSelection(nodes)
		if path != "" {
			setSelectionByPath(nodes, path, true)
		}
	})
}

// SetExpandedPath updates the expanded state for a node in the store data.
func (t *TreeNavigator) SetExpandedPath(path string, expanded bool) {
	t.mutateTree(func(nodes []TreeNode) {
		setExpandedByPath(nodes, path, expanded)
	})
}

// ExportAnchors publishes the tree anchor set.
func (t *TreeNavigator) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if t == nil {
		return nil
	}
	bounds := t.layoutRole.ArrangedBounds
	if bounds.IsEmpty() && !ctx.ResolvedLayer.Bounds.IsEmpty() {
		bounds = ctx.ResolvedLayer.Bounds
	}
	if bounds.IsEmpty() {
		return nil
	}
	out := layout.AnchorSet{
		"bounds_center":       gfx.Point{X: (bounds.Min.X + bounds.Max.X) * 0.5, Y: (bounds.Min.Y + bounds.Max.Y) * 0.5},
		"bounds_top_left":     bounds.Min,
		"bounds_top_right":    gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y},
		"bounds_bottom_left":  gfx.Point{X: bounds.Min.X, Y: bounds.Max.Y},
		"bounds_bottom_right": gfx.Point{X: bounds.Max.X, Y: bounds.Max.Y},
	}
	if len(t.cachedRowBounds) > 0 {
		idx := t.focusedRowIndex()
		if idx < 0 {
			idx = 0
		}
		if idx >= 0 && idx < len(t.cachedRowBounds) {
			rect := t.cachedRowBounds[idx]
			if !rect.IsEmpty() {
				out["baseline"] = gfx.Point{X: rect.Min.X, Y: rect.Min.Y}
			}
		}
	}
	return out
}

// Children returns the facet's immediate child list.
func (t *TreeNavigator) Children() []facet.GroupChild {
	if t == nil {
		return nil
	}
	t.rebuildVisibleNodes()
	out := make([]facet.GroupChild, 0, len(t.cachedVisibleNodes))
	for i := range t.cachedVisibleNodes {
		row := t.rowFacetForPath(t.cachedVisibleNodes[i].Path)
		if row == nil {
			continue
		}
		base := row.Base()
		layoutRole := base.LayoutRole()
		if layoutRole == nil {
			continue
		}
		out = append(out, facet.GroupChild{
			FacetID: base.ID(),
			MarkID:  treeNavigatorMarkIDTreeItem,
			Attachment: facet.Attachment{
				Placement: facet.Placement{
					Mode:   facet.PlacementLinear,
					Linear: facet.LinearPlacement{Order: i, CrossAxisAlign: facet.CrossAxisStretch},
				},
			},
			Layout:   layoutRole,
			Contract: layoutRole.Child,
		})
	}
	return out
}

// OnAttach wires store invalidation for the bound tree data store.
func (t *TreeNavigator) OnAttach(ctx facet.AttachContext) {
	if t.Data == nil {
		t.Data = store.NewValueStore[[]TreeNode](nil)
	}
	facet.Store(facet.Subscribe(t), &t.Data.OnChange, t.Data.Version, func(signal.Change[[]TreeNode]) {
		t.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	})
}

// OnActivate is unused.
func (t *TreeNavigator) OnActivate() {}

// OnDeactivate is unused.
func (t *TreeNavigator) OnDeactivate() {}

// OnDetach clears cached projection state.
func (t *TreeNavigator) OnDetach() {
	t.cachedTokens = theme.Tokens{}
	t.cachedRecipe = shared.TreeNavigatorSlots{}
	t.cachedRootBounds = gfx.Rect{}
	t.cachedTreeBounds = gfx.Rect{}
	t.cachedContentBounds = gfx.Rect{}
	t.cachedVisibleNodes = nil
	t.cachedRowBounds = nil
	t.cachedRowLeadingBounds = nil
	t.cachedRowLabelBounds = nil
	t.cachedRowDepths = nil
	t.cachedRowPaths = nil
	t.cachedRowHasChildren = nil
	t.cachedRowSelection = nil
	t.cachedRowFacets = nil
	t.cachedRowGap = 0
	t.cachedRowIndent = 0
	t.cachedRowDisclosure = 0
	t.cachedContentHeight = 0
}

func (t *TreeNavigator) invalidate(flags facet.DirtyFlags) {
	if t == nil {
		return
	}
	t.Base().Invalidate(flags)
}

func (t *TreeNavigator) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uinav.ResolveTreeNavigatorRecipe(style)
	t.cachedTokens = resolved.TokenSet()
	t.cachedRecipe = slots
	t.cachedWritingDirection = ctx.WritingDirection
	t.cachedRowGap = float32(resolved.Spacing(theme.SpacingXS))
	t.cachedRowIndent = maxFloat(resolved.Density.Scale(12), float32(resolved.Spacing(theme.SpacingM))*1.4)
	t.cachedRowDisclosure = maxFloat(resolved.Density.Scale(12), resolved.Density.Scale(16))
	t.rebuildVisibleNodes()
	t.syncRowFacets()
	contentWidth := float32(0)
	contentHeight := float32(0)
	rowHeights := make([]float32, len(t.cachedVisibleNodes))
	rowWidths := make([]float32, len(t.cachedVisibleNodes))
	for i := range t.cachedVisibleNodes {
		row := t.rowFacetForPath(t.cachedVisibleNodes[i].Path)
		if row == nil {
			continue
		}
		row.SetDisabled(t.Disabled || t.cachedVisibleNodes[i].Node.Disabled)
		row.SetSelected(t.cachedVisibleNodes[i].Node.Selected)
		row.ShowContainer = false
		row.ShowSelectionIndicator = true
		row.ShowFocusRing = false
		row.ShowLeadingIcon = t.cachedVisibleNodes[i].HasChildren || strings.TrimSpace(t.cachedVisibleNodes[i].Node.IconRef) != ""
		if t.cachedVisibleNodes[i].HasChildren {
			if t.cachedVisibleNodes[i].Node.Expanded {
				row.SetLeadingIconRef("chevron-down")
			} else {
				row.SetLeadingIconRef("chevron-right")
			}
		} else {
			row.SetLeadingIconRef(strings.TrimSpace(t.cachedVisibleNodes[i].Node.IconRef))
		}
		row.SetLabel(t.cachedVisibleNodes[i].Node.Label)
		layoutRole := row.Base().LayoutRole()
		if layoutRole == nil {
			continue
		}
		rowSize := layoutRole.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: maxFloat(0, constraints.MaxSize.W-float32(t.cachedVisibleNodes[i].Depth)*t.cachedRowIndent), H: constraints.MaxSize.H}})
		rowHeights[i] = rowSize.Size.H
		rowWidths[i] = float32(t.cachedVisibleNodes[i].Depth)*t.cachedRowIndent + rowSize.Size.W
		if rowWidths[i] > contentWidth {
			contentWidth = rowWidths[i]
		}
		contentHeight += rowSize.Size.H
		if i < len(t.cachedVisibleNodes)-1 {
			contentHeight += t.cachedRowGap
		}
	}
	t.cachedContentHeight = contentHeight
	width := maxFloat(resolved.Density.Scale(240), contentWidth)
	height := maxFloat(resolved.Density.Scale(120), contentHeight)
	if constraints.MaxSize.W > 0 {
		width = minFloat(width, constraints.MaxSize.W)
	}
	if constraints.MaxSize.H > 0 {
		height = minFloat(height, constraints.MaxSize.H)
	}
	measured := constraints.Constrain(gfx.Size{W: width, H: height})
	t.layoutRole.MeasuredSize = measured
	t.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	t.textRole.Layout = nil
	return t.layoutRole.MeasuredResult
}

func (t *TreeNavigator) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return t.measure(ctx, constraints).Size
}

func (t *TreeNavigator) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	t.cachedRootBounds = bounds
	t.cachedTreeBounds = bounds
	t.cachedRowBounds = nil
	t.cachedRowLeadingBounds = nil
	t.cachedRowLabelBounds = nil
	t.cachedRowDepths = nil
	t.cachedRowPaths = nil
	t.cachedRowHasChildren = nil
	t.cachedRowSelection = nil
	t.layoutRole.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	t.rebuildVisibleNodes()
	t.syncRowFacets()
	contentBounds := bounds
	y := contentBounds.Min.Y - t.scrollOffset
	t.cachedContentBounds = contentBounds
	for i := range t.cachedVisibleNodes {
		row := t.rowFacetForPath(t.cachedVisibleNodes[i].Path)
		if row == nil {
			continue
		}
		layoutRole := row.Base().LayoutRole()
		if layoutRole == nil {
			continue
		}
		depth := t.cachedVisibleNodes[i].Depth
		rowBounds := gfx.RectFromXYWH(contentBounds.Min.X+float32(depth)*t.cachedRowIndent, y, maxFloat(0, contentBounds.Width()-float32(depth)*t.cachedRowIndent), layoutRole.MeasuredSize.H)
		layoutRole.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementLinear, Linear: facet.LinearPlacement{Order: i}}}, rowBounds)
		t.cachedRowBounds = append(t.cachedRowBounds, rowBounds)
		t.cachedRowDepths = append(t.cachedRowDepths, depth)
		t.cachedRowPaths = append(t.cachedRowPaths, t.cachedVisibleNodes[i].Path)
		t.cachedRowHasChildren = append(t.cachedRowHasChildren, t.cachedVisibleNodes[i].HasChildren)
		t.cachedRowSelection = append(t.cachedRowSelection, t.cachedVisibleNodes[i].Node.Selected)
		leadW := t.cachedRowDisclosure
		if !t.cachedVisibleNodes[i].HasChildren && strings.TrimSpace(t.cachedVisibleNodes[i].Node.IconRef) == "" {
			leadW = 0
		}
		t.cachedRowLeadingBounds = append(t.cachedRowLeadingBounds, gfx.RectFromXYWH(rowBounds.Min.X+float32(2), rowBounds.Min.Y+rowBounds.Height()*0.5-leadW*0.5, leadW, leadW))
		t.cachedRowLabelBounds = append(t.cachedRowLabelBounds, rowBounds)
		y += rowBounds.Height()
		if i < len(t.cachedVisibleNodes)-1 {
			y += t.cachedRowGap
		}
	}
	t.viewportRole.WorldBounds = bounds
}

func (t *TreeNavigator) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.TreeNavigatorSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: t.cachedTokens}, t.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, t.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uinav.ResolveTreeNavigatorRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: t.cachedTokens}, t.cachedRecipe
}

func (t *TreeNavigator) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if t == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := t.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	root := slots.Root.Resolve(theme.StateDefault, tokens)
	tree := slots.Tree.Resolve(theme.StateDefault, tokens)
	item := slots.TreeItem.Resolve(theme.StateDefault, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)
	disclosure := slots.Disclosure.Resolve(theme.StateDefault, tokens)
	selectionMat := slots.SelectionIndicator.Resolve(theme.StateSelected, tokens)
	cmds := make([]gfx.Command, 0, 128)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(tree) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), tree)...)
	}
	for i := range t.cachedVisibleNodes {
		if i >= len(t.cachedRowBounds) || t.cachedRowBounds[i].IsEmpty() {
			continue
		}
		row := t.cachedRowBounds[i]
		rowPath := t.cachedRowPaths[i]
		node := t.cachedVisibleNodes[i]
		state := theme.StateDefault
		switch {
		case t.Disabled || node.Node.Disabled:
			state = theme.StateDisabled
		case rowPath == t.pressedPath:
			state = theme.StatePressed
		case rowPath == t.hoveredPath:
			state = theme.StateHover
		case rowPath == t.focusedPath && t.focusedVisible:
			state = theme.StateFocused
		case node.Node.Selected:
			state = theme.StateSelected
		}
		if !isTransparentMaterial(item) && (state == theme.StateHover || state == theme.StatePressed || state == theme.StateFocused) {
			cmds = append(cmds, materialCommands(gfx.RoundedRectPath(row.Inset(2, 2), float32(tokens.Radius.MD)), item)...)
		}
		if node.HasChildren && !isTransparentMaterial(disclosure) && i < len(t.cachedRowLeadingBounds) && !t.cachedRowLeadingBounds[i].IsEmpty() {
			icon := "chevron-right"
			if node.Node.Expanded {
				icon = "chevron-down"
			}
			if iconCmds := iconCommandsForRef(runtime, icon, t.cachedRowLeadingBounds[i], disclosure); len(iconCmds) > 0 {
				cmds = append(cmds, iconCmds...)
			}
		}
		if !node.HasChildren && strings.TrimSpace(node.Node.IconRef) != "" && i < len(t.cachedRowLeadingBounds) && !t.cachedRowLeadingBounds[i].IsEmpty() {
			if iconCmds := iconCommandsForRef(runtime, node.Node.IconRef, t.cachedRowLeadingBounds[i], disclosure); len(iconCmds) > 0 {
				cmds = append(cmds, iconCmds...)
			}
		}
		if rowCmds := t.rowFacetForPath(rowPath).Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: row, ContentScale: 1}); rowCmds != nil {
			cmds = append(cmds, rowCmds.Commands...)
		}
		if node.Node.Selected && !isTransparentMaterial(selectionMat) {
			selRect := row.Inset(0, 0)
			cmds = append(cmds, materialCommands(gfx.RectPath(selRect), selectionMat)...)
		}
		if rowPath == t.focusedPath && t.focusedVisible && !isTransparentMaterial(focus) {
			inset := maxFloat(1, row.Height()*0.08)
			cmds = append(cmds, materialCommands(gfx.RoundedRectPath(row.Inset(-inset, -inset), float32(tokens.Radius.MD)+inset), focus)...)
		}
	}
	return cmds
}

func (t *TreeNavigator) hitTest(p gfx.Point) facet.HitResult {
	if t == nil || t.layoutRole.ArrangedBounds.IsEmpty() || !t.layoutRole.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := t.cursorShape()
	if t.focusedVisible && t.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: treeNavigatorMarkIDFocusRing, Cursor: cursor}
	}
	for i := range t.cachedRowBounds {
		if !t.cachedRowBounds[i].Contains(p) {
			continue
		}
		if i < len(t.cachedRowLeadingBounds) && t.cachedRowLeadingBounds[i].Contains(p) {
			return facet.HitResult{Hit: true, MarkID: treeNavigatorMarkIDDisclosure, Cursor: cursor}
		}
		if t.cachedRowSelection[i] {
			return facet.HitResult{Hit: true, MarkID: treeNavigatorMarkIDSelectionIndicator, Cursor: cursor}
		}
		return facet.HitResult{Hit: true, MarkID: treeNavigatorMarkIDLabel, Cursor: cursor}
	}
	if t.cachedTreeBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: treeNavigatorMarkIDTree, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: treeNavigatorMarkIDRoot, Cursor: cursor}
}

func (t *TreeNavigator) onPointer(e facet.PointerEvent) bool {
	if t.Disabled {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		t.hoveredPath = t.pathAt(e.Position)
		t.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		t.hoveredPath = ""
		if t.pressedPath == "" {
			t.focusFromPointer = false
		}
		t.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		path, disclosure := t.pathAtWithDisclosure(e.Position)
		if path == "" {
			return false
		}
		t.hoveredPath = path
		t.pressedPath = path
		t.focusFromPointer = true
		t.focusedVisible = false
		t.invalidate(facet.DirtyProjection)
		if disclosure {
			return true
		}
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		path := t.pressedPath
		t.pressedPath = ""
		t.invalidate(facet.DirtyProjection)
		if path == "" {
			return false
		}
		releasePath, disclosure := t.pathAtWithDisclosure(e.Position)
		if releasePath == path && !t.isDisabledPath(path) {
			if disclosure {
				t.toggleExpanded(path)
			} else {
				t.selectPath(path)
			}
			return true
		}
		return true
	case platform.PointerMove:
		path := t.pathAt(e.Position)
		if path != t.hoveredPath {
			t.hoveredPath = path
			t.invalidate(facet.DirtyProjection)
		}
		return true
	default:
		return false
	}
}

func (t *TreeNavigator) onScroll(e facet.ScrollEvent) bool {
	if t.Disabled || len(t.cachedRowBounds) == 0 {
		return false
	}
	if e.DeltaY == 0 {
		return false
	}
	t.scrollOffset -= e.DeltaY
	maxOffset := maxFloat(0, t.cachedContentHeight-t.layoutRole.ArrangedBounds.Height())
	t.scrollOffset = clampFloat(t.scrollOffset, 0, maxOffset)
	t.invalidate(facet.DirtyProjection)
	return true
}

func (t *TreeNavigator) onKey(e facet.KeyEvent) bool {
	if t.Disabled || len(t.cachedVisibleNodes) == 0 {
		return false
	}
	switch e.Key {
	case platform.KeyUp, platform.KeyDown, platform.KeyLeft, platform.KeyRight, platform.KeyHome, platform.KeyEnd, platform.KeySpace, platform.KeyEnter, platform.KeyPageUp, platform.KeyPageDown:
		if e.Kind != platform.KeyPress && e.Kind != platform.KeyRepeat {
			if e.Kind == platform.KeyRelease && (e.Key == platform.KeySpace || e.Key == platform.KeyEnter) {
				return true
			}
			return false
		}
		switch e.Key {
		case platform.KeyUp:
			t.moveFocus(-1)
			return true
		case platform.KeyDown:
			t.moveFocus(1)
			return true
		case platform.KeyHome:
			t.focusFirst()
			return true
		case platform.KeyEnd:
			t.focusLast()
			return true
		case platform.KeyLeft:
			return t.collapseOrParent()
		case platform.KeyRight:
			return t.expandOrChild()
		case platform.KeySpace, platform.KeyEnter:
			if path := t.focusedRowPath(); path != "" {
				if node, ok := t.visibleNodeByPath(path); ok {
					if node.HasChildren {
						t.toggleExpanded(path)
					} else {
						t.selectPath(path)
					}
					return true
				}
			}
		}
		return true
	default:
		if e.Kind == platform.KeyPress && t.typeahead(e.Key) {
			return true
		}
	}
	return false
}

func (t *TreeNavigator) onFocusGained() {
	t.focusedVisible = !t.focusFromPointer
	t.focusFromPointer = false
	if t.focusedPath == "" {
		t.focusedPath = t.firstVisiblePath()
	}
	t.invalidate(facet.DirtyProjection)
}

func (t *TreeNavigator) onFocusLost() {
	t.focusedVisible = false
	t.pressedPath = ""
	t.focusFromPointer = false
	t.invalidate(facet.DirtyProjection)
}

func (t *TreeNavigator) rootState() theme.InteractionState {
	if t.Disabled {
		return theme.StateDisabled
	}
	if t.pressedPath != "" {
		return theme.StatePressed
	}
	if t.hoveredPath != "" {
		return theme.StateHover
	}
	if t.focusedVisible {
		return theme.StateFocused
	}
	return theme.StateDefault
}

func (t *TreeNavigator) pointInFocusRing(p gfx.Point) bool {
	if !t.focusedVisible {
		return false
	}
	idx := t.focusedRowIndex()
	if idx < 0 || idx >= len(t.cachedRowBounds) {
		return false
	}
	row := t.cachedRowBounds[idx]
	if row.IsEmpty() || !row.Contains(p) {
		return false
	}
	inset := maxFloat(1, row.Height()*0.08)
	return !row.Inset(inset, inset).Contains(p)
}

func (t *TreeNavigator) cursorShape() facet.CursorShape {
	if t.Disabled {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (t *TreeNavigator) rebuildVisibleNodes() {
	nodes := t.nodesSnapshot()
	out := make([]treeNavigatorVisibleNode, 0, len(nodes))
	t.walkVisible(nodes, 0, "", &out)
	t.cachedVisibleNodes = out
	if t.focusedPath == "" {
		t.focusedPath = t.firstVisiblePath()
	}
}

func (t *TreeNavigator) syncRowFacets() {
	if t.cachedRowFacets == nil {
		t.cachedRowFacets = make(map[string]*selection.ListItem, len(t.cachedVisibleNodes))
	}
	for i := range t.cachedVisibleNodes {
		path := t.cachedVisibleNodes[i].Path
		row := t.rowFacetForPath(path)
		if row == nil {
			continue
		}
		row.SetLabel(t.cachedVisibleNodes[i].Node.Label)
		row.SetDisabled(t.Disabled || t.cachedVisibleNodes[i].Node.Disabled)
		row.SetSelected(t.cachedVisibleNodes[i].Node.Selected)
		row.ShowLabel = true
		row.ShowContainer = false
		row.ShowLeadingIcon = t.cachedVisibleNodes[i].HasChildren || strings.TrimSpace(t.cachedVisibleNodes[i].Node.IconRef) != ""
		row.ShowSelectionIndicator = true
		row.ShowFocusRing = false
		if t.cachedVisibleNodes[i].HasChildren {
			if t.cachedVisibleNodes[i].Node.Expanded {
				row.SetLeadingIconRef("chevron-down")
			} else {
				row.SetLeadingIconRef("chevron-right")
			}
		} else {
			row.SetLeadingIconRef(strings.TrimSpace(t.cachedVisibleNodes[i].Node.IconRef))
		}
	}
}

func (t *TreeNavigator) rowFacetForPath(path string) *selection.ListItem {
	if t.cachedRowFacets == nil {
		t.cachedRowFacets = make(map[string]*selection.ListItem)
	}
	if row := t.cachedRowFacets[path]; row != nil {
		return row
	}
	row := selection.NewListItem("")
	row.ShowContainer = false
	row.ShowSelectionIndicator = true
	row.ShowFocusRing = false
	t.cachedRowFacets[path] = row
	return row
}

func (t *TreeNavigator) nodesSnapshot() []TreeNode {
	if t.Data == nil {
		return nil
	}
	nodes := t.Data.Get()
	return cloneTreeNodes(nodes)
}

func (t *TreeNavigator) walkVisible(nodes []TreeNode, depth int, parentPath string, out *[]treeNavigatorVisibleNode) {
	for i := range nodes {
		node := nodes[i]
		key := strings.TrimSpace(node.Key)
		if key == "" {
			key = "node_" + strings.ReplaceAll(strings.TrimSpace(node.Label), " ", "_")
			if key == "node_" {
				key = "node"
			}
			key = key + "_" + strings.TrimSpace(parentPath)
		}
		path := key
		if parentPath != "" {
			path = parentPath + "/" + key
		}
		entry := treeNavigatorVisibleNode{
			Path:        path,
			Depth:       depth,
			Node:        node,
			HasChildren: len(node.Children) > 0,
		}
		*out = append(*out, entry)
		if node.Expanded && len(node.Children) > 0 {
			t.walkVisible(node.Children, depth+1, path, out)
		}
	}
}

func (t *TreeNavigator) visibleNodeByPath(path string) (treeNavigatorVisibleNode, bool) {
	for i := range t.cachedVisibleNodes {
		if t.cachedVisibleNodes[i].Path == path {
			return t.cachedVisibleNodes[i], true
		}
	}
	return treeNavigatorVisibleNode{}, false
}

func (t *TreeNavigator) firstVisiblePath() string {
	if len(t.cachedVisibleNodes) == 0 {
		return ""
	}
	return t.cachedVisibleNodes[0].Path
}

func (t *TreeNavigator) focusedRowPath() string {
	if t.focusedPath != "" {
		return t.focusedPath
	}
	return t.firstVisiblePath()
}

func (t *TreeNavigator) focusedRowIndex() int {
	path := t.focusedRowPath()
	for i := range t.cachedVisibleNodes {
		if t.cachedVisibleNodes[i].Path == path {
			return i
		}
	}
	return -1
}

func (t *TreeNavigator) firstEnabledIndex() int {
	for i := range t.cachedVisibleNodes {
		if !t.isDisabledPath(t.cachedVisibleNodes[i].Path) {
			return i
		}
	}
	return -1
}

func (t *TreeNavigator) isDisabledPath(path string) bool {
	for i := range t.cachedVisibleNodes {
		if t.cachedVisibleNodes[i].Path == path {
			return t.Disabled || t.cachedVisibleNodes[i].Node.Disabled
		}
	}
	return true
}

func (t *TreeNavigator) pathAt(p gfx.Point) string {
	path, _ := t.pathAtWithDisclosure(p)
	return path
}

func (t *TreeNavigator) pathAtWithDisclosure(p gfx.Point) (string, bool) {
	for i := range t.cachedRowBounds {
		if !t.cachedRowBounds[i].Contains(p) {
			continue
		}
		path := t.cachedRowPaths[i]
		if i < len(t.cachedRowLeadingBounds) && t.cachedRowLeadingBounds[i].Contains(p) {
			return path, true
		}
		return path, false
	}
	return "", false
}

func (t *TreeNavigator) selectPath(path string) {
	t.mutateTree(func(nodes []TreeNode) {
		clearSelection(nodes)
		setSelectionByPath(nodes, path, true)
	})
	t.focusedPath = path
	t.focusedVisible = true
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func (t *TreeNavigator) toggleExpanded(path string) {
	t.mutateTree(func(nodes []TreeNode) {
		toggleExpandedByPath(nodes, path)
	})
	t.focusedPath = path
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func (t *TreeNavigator) mutateTree(mutator func([]TreeNode)) {
	if t == nil {
		return
	}
	if t.Data == nil {
		t.Data = store.NewValueStore[[]TreeNode](nil)
	}
	nodes := cloneTreeNodes(t.Data.Get())
	mutator(nodes)
	t.Data.Set(nodes)
}

func (t *TreeNavigator) moveFocus(delta int) {
	if len(t.cachedVisibleNodes) == 0 {
		return
	}
	start := t.focusedRowIndex()
	if start < 0 {
		start = 0
	}
	for step := 1; step <= len(t.cachedVisibleNodes); step++ {
		next := start + delta*step
		for next < 0 {
			next += len(t.cachedVisibleNodes)
		}
		next %= len(t.cachedVisibleNodes)
		if !t.isDisabledPath(t.cachedVisibleNodes[next].Path) {
			t.focusedPath = t.cachedVisibleNodes[next].Path
			t.invalidate(facet.DirtyProjection)
			return
		}
	}
}

func (t *TreeNavigator) focusFirst() {
	if idx := t.firstEnabledIndex(); idx >= 0 {
		t.focusedPath = t.cachedVisibleNodes[idx].Path
		t.invalidate(facet.DirtyProjection)
	}
}

func (t *TreeNavigator) focusLast() {
	for i := len(t.cachedVisibleNodes) - 1; i >= 0; i-- {
		if !t.isDisabledPath(t.cachedVisibleNodes[i].Path) {
			t.focusedPath = t.cachedVisibleNodes[i].Path
			t.invalidate(facet.DirtyProjection)
			return
		}
	}
}

func (t *TreeNavigator) collapseOrParent() bool {
	path := t.focusedRowPath()
	node, ok := t.visibleNodeByPath(path)
	if !ok {
		return false
	}
	if node.Node.Expanded && node.HasChildren {
		t.toggleExpanded(path)
		return true
	}
	if parent := parentPath(path); parent != "" {
		t.focusedPath = parent
		t.invalidate(facet.DirtyProjection)
		return true
	}
	return false
}

func (t *TreeNavigator) expandOrChild() bool {
	path := t.focusedRowPath()
	node, ok := t.visibleNodeByPath(path)
	if !ok {
		return false
	}
	if node.HasChildren && !node.Node.Expanded {
		t.toggleExpanded(path)
		return true
	}
	if node.HasChildren && node.Node.Expanded {
		for i := range t.cachedVisibleNodes {
			if parentPath(t.cachedVisibleNodes[i].Path) == path {
				t.focusedPath = t.cachedVisibleNodes[i].Path
				t.invalidate(facet.DirtyProjection)
				return true
			}
		}
	}
	return false
}

func (t *TreeNavigator) typeahead(key platform.Key) bool {
	if key < platform.KeyA || key > platform.KeyZ || len(t.cachedVisibleNodes) == 0 {
		return false
	}
	target := strings.ToLower(string(rune('a' + int(key-platform.KeyA))))
	start := t.focusedRowIndex() + 1
	for offset := 0; offset < len(t.cachedVisibleNodes); offset++ {
		i := (start + offset) % len(t.cachedVisibleNodes)
		label := strings.ToLower(t.cachedVisibleNodes[i].Node.Label)
		if strings.HasPrefix(label, target) && !t.isDisabledPath(t.cachedVisibleNodes[i].Path) {
			t.focusedPath = t.cachedVisibleNodes[i].Path
			t.invalidate(facet.DirtyProjection)
			return true
		}
	}
	return false
}

func parentPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[:idx]
	}
	return ""
}

func clearSelection(nodes []TreeNode) {
	for i := range nodes {
		nodes[i].Selected = false
		if len(nodes[i].Children) > 0 {
			clearSelection(nodes[i].Children)
		}
	}
}

func setSelectionByPath(nodes []TreeNode, path string, selected bool) bool {
	segments := splitPath(path)
	if len(segments) == 0 {
		return false
	}
	return setSelectionByPathSegments(nodes, segments, selected)
}

func setExpandedByPath(nodes []TreeNode, path string, expanded bool) bool {
	segments := splitPath(path)
	if len(segments) == 0 {
		return false
	}
	return setExpandedByPathSegments(nodes, segments, expanded)
}

func toggleExpandedByPath(nodes []TreeNode, path string) bool {
	segments := splitPath(path)
	if len(segments) == 0 {
		return false
	}
	return toggleExpandedByPathSegments(nodes, segments)
}

func setSelectionByPathSegments(nodes []TreeNode, segments []string, selected bool) bool {
	if len(segments) == 0 {
		return false
	}
	for i := range nodes {
		if strings.TrimSpace(nodes[i].Key) != segments[0] {
			continue
		}
		if len(segments) == 1 {
			nodes[i].Selected = selected
			return true
		}
		if len(nodes[i].Children) > 0 && setSelectionByPathSegments(nodes[i].Children, segments[1:], selected) {
			return true
		}
	}
	return false
}

func setExpandedByPathSegments(nodes []TreeNode, segments []string, expanded bool) bool {
	if len(segments) == 0 {
		return false
	}
	for i := range nodes {
		if strings.TrimSpace(nodes[i].Key) != segments[0] {
			continue
		}
		if len(segments) == 1 {
			nodes[i].Expanded = expanded
			return true
		}
		if len(nodes[i].Children) > 0 && setExpandedByPathSegments(nodes[i].Children, segments[1:], expanded) {
			return true
		}
	}
	return false
}

func toggleExpandedByPathSegments(nodes []TreeNode, segments []string) bool {
	if len(segments) == 0 {
		return false
	}
	for i := range nodes {
		if strings.TrimSpace(nodes[i].Key) != segments[0] {
			continue
		}
		if len(segments) == 1 {
			nodes[i].Expanded = !nodes[i].Expanded
			return true
		}
		if len(nodes[i].Children) > 0 && toggleExpandedByPathSegments(nodes[i].Children, segments[1:]) {
			return true
		}
	}
	return false
}

func splitPath(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	raw := strings.Split(path, "/")
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func cloneTreeNodes(nodes []TreeNode) []TreeNode {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]TreeNode, len(nodes))
	copy(out, nodes)
	for i := range out {
		if len(out[i].Children) > 0 {
			out[i].Children = cloneTreeNodes(out[i].Children)
		}
		out[i].Key = strings.TrimSpace(out[i].Key)
		out[i].Label = strings.TrimSpace(out[i].Label)
		out[i].IconRef = strings.TrimSpace(out[i].IconRef)
	}
	return out
}

func resolveIconAsset(runtime any, ref string) (runtimepkg.IconAsset, bool) {
	type iconProvider interface {
		IconResolver() runtimepkg.IconResolver
	}
	if runtime == nil {
		return runtimepkg.IconAsset{}, false
	}
	if provider, ok := runtime.(iconProvider); ok {
		if resolver := provider.IconResolver(); resolver != nil {
			return resolver.ResolveIcon(ref)
		}
	}
	if resolver, ok := runtime.(interface {
		ResolveIcon(string) (runtimepkg.IconAsset, bool)
	}); ok {
		return resolver.ResolveIcon(ref)
	}
	return runtimepkg.IconAsset{}, false
}

func clampFloat(v, minV, maxV float32) float32 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func iconCommandsForRef(runtime any, ref string, bounds gfx.Rect, material theme.Material) []gfx.Command {
	if bounds.IsEmpty() || isTransparentMaterial(material) || strings.TrimSpace(ref) == "" {
		return nil
	}
	asset, ok := resolveIconAsset(runtime, ref)
	if !ok || len(asset.Path.Segments) == 0 {
		return nil
	}
	box := asset.ViewBox
	if box.IsEmpty() {
		box = gfxsvg.Bounds(asset.Path)
	}
	if box.IsEmpty() || box.Width() == 0 || box.Height() == 0 {
		return nil
	}
	sx := bounds.Width() / box.Width()
	sy := bounds.Height() / box.Height()
	scale := minFloat(sx, sy)
	if scale <= 0 {
		return nil
	}
	target := gfxsvg.Transformed(asset.Path, gfx.Identity().Multiply(gfx.Translation(bounds.Min.X-box.Min.X*scale, bounds.Min.Y-box.Min.Y*scale)).Multiply(gfx.Scale(scale, scale)))
	return []gfx.Command{gfx.FillPath{Path: target, Brush: gfx.SolidBrush(materialColor(material))}}
}

type treeNavigatorGroupPolicy struct {
	tree *TreeNavigator
}

func (treeNavigatorGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (p treeNavigatorGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.tree == nil || len(children) == 0 {
		return facet.GroupMeasureResult{}, nil
	}
	ordered := orderedTreeNavigatorChildren(children)
	width := float32(0)
	height := float32(0)
	for i, idx := range ordered {
		child := children[idx]
		if child.Layout == nil {
			continue
		}
		size := child.Layout.MeasuredSize
		if size == (gfx.Size{}) {
			size = child.Layout.Measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}}).Size
		}
		if size.W > width {
			width = size.W
		}
		height += size.H
		if i < len(ordered)-1 {
			height += p.tree.cachedRowGap
		}
	}
	return facet.GroupMeasureResult{Size: gfx.Size{W: width, H: height}}, nil
}

func (p treeNavigatorGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.tree == nil || len(children) == 0 {
		return nil, nil
	}
	ordered := orderedTreeNavigatorChildren(children)
	y := ctx.Bounds.Min.Y
	arranged := make([]facet.ArrangedGroupChild, 0, len(ordered))
	for i, idx := range ordered {
		child := children[idx]
		if child.Layout == nil {
			continue
		}
		size := child.Layout.MeasuredSize
		rect := gfx.RectFromXYWH(ctx.Bounds.Min.X, y, ctx.Bounds.Width(), size.H)
		child.Layout.Arrange(facet.ArrangeContext{Placement: child.Attachment.Placement}, rect)
		arranged = append(arranged, facet.ArrangedGroupChild{
			FacetID:   child.FacetID,
			MarkID:    child.MarkID,
			Bounds:    rect,
			Placement: child.Attachment.Placement,
			ZPriority: child.Attachment.ZPriority,
			Contract:  child.Contract,
		})
		y += rect.Height()
		if i < len(ordered)-1 {
			y += p.tree.cachedRowGap
		}
	}
	return arranged, nil
}

func orderedTreeNavigatorChildren(children []facet.GroupChild) []int {
	indices := make([]int, len(children))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(i, j int) bool {
		left := children[indices[i]]
		right := children[indices[j]]
		if left.Attachment.Placement.Linear.Order != right.Attachment.Placement.Linear.Order {
			return left.Attachment.Placement.Linear.Order < right.Attachment.Placement.Linear.Order
		}
		if left.Attachment.ZPriority != right.Attachment.ZPriority {
			return left.Attachment.ZPriority > right.Attachment.ZPriority
		}
		return false
	})
	return indices
}
