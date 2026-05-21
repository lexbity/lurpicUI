package structure

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	layoutlinear "codeburg.org/lexbit/lurpicui/layout/linear"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uistruct"
)

const (
	listMarkIDRoot                  facet.MarkID = 1
	listMarkIDListContainer         facet.MarkID = 2
	listMarkIDListItems             facet.MarkID = 3
	listMarkIDSectionHeaderOptional facet.MarkID = 4
	listMarkIDEmptyStateOptional    facet.MarkID = 5
)

// ListEntry describes one generated list row.
type ListEntry struct {
	Key            string
	Label          string
	SupportingText string
	LeadingIconRef string
	Selected       bool
	Active         bool
	Disabled       bool
}

// List implements the structure.list canonical mark.
type List struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	textRole       facet.TextRole

	Activated signal.Signal[int]

	Label         string
	SectionHeader string
	EmptyState    string
	Disabled      bool
	ItemVariant   uiinput.ListItemVariant
	Data          *store.ValueStore[[]ListEntry]
	scrollRegion  *ScrollRegion

	cachedDataSub signal.SubscriptionID

	cachedTokens           theme.Tokens
	cachedRecipe           shared.ListSlots
	cachedBounds           gfx.Rect
	cachedContainerBounds  gfx.Rect
	cachedItemBounds       gfx.Rect
	cachedHeaderBounds     gfx.Rect
	cachedEmptyBounds      gfx.Rect
	cachedRowBounds        map[string]gfx.Rect
	cachedRowOrder         []string
	cachedRows             map[string]*selection.ListItem
	cachedHeaderMark       *primitive.Text
	cachedEmptyMark        *primitive.Text
	cachedStoreVersion     store.Version
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedWritingDirection facet.WritingDirection
}

var _ facet.FacetImpl = (*List)(nil)
var _ layout.AnchorExporter = (*List)(nil)

// NewList constructs a structure.list mark with canonical defaults.
func NewList(label string, entries []ListEntry) *List {
	l := &List{
		Facet:           facet.NewFacet(),
		Label:           label,
		ItemVariant:     uiinput.ListItemStandard,
		Data:            store.NewValueStore(cloneListEntries(entries)),
		scrollRegion:    NewScrollRegion(label),
		cachedRows:      make(map[string]*selection.ListItem),
		cachedRowBounds: make(map[string]gfx.Rect),
	}
	l.scrollRegion.SetDirection(ScrollDirectionVertical)
	l.layoutRole.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   listGroupPolicy{list: l},
		Children: l,
	}
	l.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := l.measure(ctx, constraints).Size
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionTruncate,
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
	l.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return l.measure(ctx, constraints)
	}
	l.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		l.layoutRole.ArrangedBounds = bounds
		l.arrange(ctx, bounds)
	}
	l.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := l.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	l.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := l.buildCommands(l.layoutRole.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	l.textRole.IMEEnabled = false
	l.AddRole(&l.layoutRole)
	l.AddRole(&l.renderRole)
	l.AddRole(&l.projectionRole)
	l.AddRole(&l.textRole)
	if l.Data != nil {
		l.cachedDataSub = l.Data.OnChange.Subscribe(func(_ signal.Change[[]ListEntry]) {
			l.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		})
	}
	l.syncChildren()
	return l
}

// Base satisfies facet.FacetImpl.
func (l *List) Base() *facet.Facet {
	l.Facet.BindImpl(l)
	return &l.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (l *List) AccessibilityRole() string { return "list" }

// AccessibleName reports the semantic name source required by the spec.
func (l *List) AccessibleName() string {
	if l == nil {
		return ""
	}
	if strings.TrimSpace(l.Label) != "" {
		return strings.TrimSpace(l.Label)
	}
	if strings.TrimSpace(l.SectionHeader) != "" {
		return strings.TrimSpace(l.SectionHeader)
	}
	items := l.entries()
	if len(items) > 0 {
		return strings.TrimSpace(items[0].Label)
	}
	return ""
}

// SetLabel updates the authored accessible label.
func (l *List) SetLabel(label string) {
	if l == nil || l.Label == label {
		return
	}
	l.Label = label
	if l.scrollRegion != nil {
		l.scrollRegion.SetLabel(label)
	}
	l.invalidate(facet.DirtyProjection)
}

// SetSectionHeader updates the authored section header.
func (l *List) SetSectionHeader(header string) {
	if l == nil || l.SectionHeader == header {
		return
	}
	l.SectionHeader = header
	l.syncChildren()
	l.invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// SetEmptyState updates the authored empty-state text.
func (l *List) SetEmptyState(text string) {
	if l == nil || l.EmptyState == text {
		return
	}
	l.EmptyState = text
	l.syncChildren()
	l.invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// SetDisabled toggles disabled state.
func (l *List) SetDisabled(disabled bool) {
	if l == nil || l.Disabled == disabled {
		return
	}
	l.Disabled = disabled
	if l.scrollRegion != nil {
		l.scrollRegion.SetDisabled(disabled)
	}
	l.syncChildren()
	l.invalidate(facet.DirtyProjection)
}

// SetEntries replaces the canonical data-store contents.
func (l *List) SetEntries(entries []ListEntry) {
	if l == nil {
		return
	}
	if l.Data == nil {
		l.Data = store.NewValueStore(cloneListEntries(entries))
		l.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return
	}
	l.Data.Set(cloneListEntries(entries))
}

// Children returns the immediate child list.
func (l *List) Children() []facet.GroupChild {
	if l == nil {
		return nil
	}
	if l.scrollRegion == nil {
		return nil
	}
	l.syncChildren()
	return []facet.GroupChild{listGroupChild(l.scrollRegion.Base(), listMarkIDListContainer, 0)}
}

// ExportAnchors publishes the list anchor set.
func (l *List) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if l == nil || l.scrollRegion == nil {
		return nil
	}
	out := l.scrollRegion.ExportAnchors(ctx)
	if !l.cachedHeaderBounds.IsEmpty() {
		out["section_header"] = gfx.Point{X: (l.cachedHeaderBounds.Min.X + l.cachedHeaderBounds.Max.X) * 0.5, Y: (l.cachedHeaderBounds.Min.Y + l.cachedHeaderBounds.Max.Y) * 0.5}
	}
	if !l.cachedEmptyBounds.IsEmpty() {
		out["empty_state"] = gfx.Point{X: (l.cachedEmptyBounds.Min.X + l.cachedEmptyBounds.Max.X) * 0.5, Y: (l.cachedEmptyBounds.Min.Y + l.cachedEmptyBounds.Max.Y) * 0.5}
	}
	for key, b := range l.cachedRowBounds {
		if b.IsEmpty() {
			continue
		}
		out[layout.AnchorID("item_"+key)] = gfx.Point{X: (b.Min.X + b.Max.X) * 0.5, Y: (b.Min.Y + b.Max.Y) * 0.5}
	}
	return out
}

// OnAttach is unused.
func (l *List) OnAttach(ctx facet.AttachContext) {}

// OnActivate is unused.
func (l *List) OnActivate() {}

// OnDeactivate is unused.
func (l *List) OnDeactivate() {}

// OnDetach clears cached projection state.
func (l *List) OnDetach() {
	if l != nil && l.Data != nil && l.cachedDataSub != 0 {
		l.Data.OnChange.Unsubscribe(l.cachedDataSub)
	}
	if l != nil && l.scrollRegion != nil {
		l.scrollRegion.OnDetach()
	}
	l.cachedTokens = theme.Tokens{}
	l.cachedRecipe = shared.ListSlots{}
	l.cachedBounds = gfx.Rect{}
	l.cachedContainerBounds = gfx.Rect{}
	l.cachedItemBounds = gfx.Rect{}
	l.cachedHeaderBounds = gfx.Rect{}
	l.cachedEmptyBounds = gfx.Rect{}
	l.cachedRowBounds = nil
	l.cachedRowOrder = nil
	l.cachedRows = nil
	l.cachedHeaderMark = nil
	l.cachedEmptyMark = nil
	l.cachedDataSub = 0
}

func (l *List) invalidate(flags facet.DirtyFlags) {
	if l == nil {
		return
	}
	l.Base().Invalidate(flags)
}

func (l *List) entries() []ListEntry {
	if l == nil || l.Data == nil {
		return nil
	}
	return cloneListEntries(l.Data.Get())
}

func (l *List) syncChildren() {
	if l == nil {
		return
	}
	if l.scrollRegion == nil {
		l.scrollRegion = NewScrollRegion(l.Label)
	}
	l.scrollRegion.SetLabel(l.Label)
	l.scrollRegion.SetDisabled(l.Disabled)
	l.scrollRegion.SetDirection(ScrollDirectionVertical)
	if strings.TrimSpace(l.SectionHeader) != "" {
		if l.cachedHeaderMark == nil {
			l.cachedHeaderMark = primitive.NewText(strings.TrimSpace(l.SectionHeader))
		} else {
			l.cachedHeaderMark.SetContent(strings.TrimSpace(l.SectionHeader))
		}
		l.cachedHeaderMark.SetTypography(theme.TextLabelM)
		l.cachedHeaderMark.SetForeground(theme.ColorTextSecondary)
		l.cachedHeaderMark.SetOverflow(primitive.TextOverflowTruncate)
	} else {
		l.cachedHeaderMark = nil
	}

	entries := l.entries()
	if len(entries) == 0 {
		if strings.TrimSpace(l.EmptyState) != "" {
			if l.cachedEmptyMark == nil {
				l.cachedEmptyMark = primitive.NewText(strings.TrimSpace(l.EmptyState))
			} else {
				l.cachedEmptyMark.SetContent(strings.TrimSpace(l.EmptyState))
			}
			l.cachedEmptyMark.SetTypography(theme.TextBodyS)
			l.cachedEmptyMark.SetForeground(theme.ColorTextSecondary)
			l.cachedEmptyMark.SetOverflow(primitive.TextOverflowTruncate)
		}
	} else {
		l.cachedEmptyMark = nil
	}

	nextRows := make(map[string]*selection.ListItem, len(entries))
	nextOrder := make([]string, 0, len(entries))
	for i := range entries {
		entry := entries[i]
		key := stableListKey(entry, i)
		row := l.cachedRows[key]
		if row == nil {
			row = selection.NewListItem(entry.Label)
		}
		row.SetLabel(entry.Label)
		row.SetSupportingText(entry.SupportingText)
		row.SetLeadingIconRef(entry.LeadingIconRef)
		row.SetSelected(entry.Selected)
		row.SetActive(entry.Active)
		row.SetDisabled(l.Disabled || entry.Disabled)
		row.Variant = l.ItemVariant
		row.ShowContainer = false
		row.ShowSelectionIndicator = false
		row.ShowFocusRing = false
		row.ShowLeadingIcon = entry.LeadingIconRef != ""
		row.ShowLabel = true
		nextRows[key] = row
		nextOrder = append(nextOrder, key)
	}
	l.cachedRows = nextRows
	l.cachedRowOrder = nextOrder
	content := make([]ScrollRegionChild, 0, len(nextOrder)+2)
	if l.cachedHeaderMark != nil {
		content = append(content, listScrollChild(l.cachedHeaderMark, listMarkIDSectionHeaderOptional, len(content)))
	}
	for _, key := range nextOrder {
		row := nextRows[key]
		if row == nil {
			continue
		}
		content = append(content, listScrollChild(row, listMarkIDListItems, len(content)))
	}
	if l.cachedEmptyMark != nil {
		content = append(content, listScrollChild(l.cachedEmptyMark, listMarkIDEmptyStateOptional, len(content)))
	}
	l.scrollRegion.SetGap(l.cachedGap)
	l.scrollRegion.SetChildren(content)
}

func listScrollChild(mark facet.FacetImpl, markID facet.MarkID, order int) ScrollRegionChild {
	base := childFacetID(mark)
	if base == 0 {
		return ScrollRegionChild{}
	}
	return ScrollRegionChild{
		Facet:  mark,
		MarkID: markID,
		Placement: facet.Placement{
			Mode: facet.PlacementLinear,
			Linear: facet.LinearPlacement{
				Order:          order,
				CrossAxisAlign: facet.CrossAxisStretch,
			},
		},
	}
}

func stableListKey(entry ListEntry, index int) string {
	key := strings.TrimSpace(entry.Key)
	if key != "" {
		return key
	}
	key = strings.TrimSpace(entry.Label)
	if key != "" {
		return key
	}
	return fmt.Sprintf("item-%d", index)
}

func (l *List) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uistruct.ResolveListRecipe(style)
	l.cachedTokens = resolved.TokenSet()
	l.cachedRecipe = slots
	l.cachedWritingDirection = ctx.WritingDirection
	l.cachedGap = float32(resolved.Spacing(theme.SpacingS))
	l.syncChildren()
	if l.scrollRegion != nil {
		l.scrollRegion.SetLabel(l.Label)
		l.scrollRegion.SetDisabled(l.Disabled)
		l.scrollRegion.SetDirection(ScrollDirectionVertical)
		l.scrollRegion.SetGap(l.cachedGap)
		result := l.scrollRegion.layoutRole.Measure(ctx, constraints)
		l.layoutRole.MeasuredSize = result.Size
		l.layoutRole.MeasuredResult = result
		l.textRole.Layout = nil
		return result
	}
	size := constraints.Constrain(gfx.Size{})
	l.layoutRole.MeasuredSize = size
	l.layoutRole.MeasuredResult = facet.MeasureResult{
		Size:        size,
		Intrinsic:   facet.IntrinsicSize{Min: size, Preferred: size, Max: size},
		Constraints: constraints,
	}
	l.textRole.Layout = nil
	return l.layoutRole.MeasuredResult
}

func (l *List) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	l.cachedBounds = bounds
	l.layoutRole.ArrangedBounds = bounds
	l.cachedRowBounds = make(map[string]gfx.Rect)
	if l.scrollRegion == nil || bounds.IsEmpty() {
		return
	}
	l.scrollRegion.arrange(ctx, bounds)
	if l.scrollRegion != nil {
		l.cachedHeaderBounds = l.scrollRegion.childBoundsForProjection(childFacetID(l.cachedHeaderMark))
		l.cachedEmptyBounds = l.scrollRegion.childBoundsForProjection(childFacetID(l.cachedEmptyMark))
		for key, row := range l.cachedRows {
			if row == nil {
				continue
			}
			if b := l.scrollRegion.childBoundsForProjection(row.Base().ID()); !b.IsEmpty() {
				l.cachedRowBounds[key] = b
			}
		}
	}
}

func (l *List) linearChildren(children []facet.GroupChild) []layoutlinear.Child {
	out := make([]layoutlinear.Child, 0, len(children))
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		if !child.Contract.SupportedPlacement.Has(facet.PlacementLinear) {
			continue
		}
		out = append(out, layoutlinear.Child{
			FacetID:    child.FacetID,
			Attachment: child.Attachment,
			Layout:     child.Layout,
			Contract:   child.Contract,
		})
	}
	return out
}

func (l *List) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if l == nil || bounds.IsEmpty() || l.scrollRegion == nil {
		return nil
	}
	return l.scrollRegion.buildCommands(bounds, runtime, contentScale)
}

func (l *List) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.ListSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: l.cachedTokens}, l.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, l.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uistruct.ResolveListRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: l.cachedTokens}, l.cachedRecipe
}

type listGroupPolicy struct {
	list *List
}

func (listGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (p listGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.list == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := p.list.measure(ctx.MeasureContext, facet.Constraints{
		MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()},
	}).Size
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p listGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.list == nil {
		return nil, nil
	}
	p.list.arrange(ctx.ArrangeContext, ctx.Bounds)
	arranged := make([]facet.ArrangedGroupChild, 0, len(children))
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		arranged = append(arranged, facet.ArrangedGroupChild{
			FacetID:   child.FacetID,
			MarkID:    child.MarkID,
			Bounds:    child.Layout.ArrangedBounds,
			Placement: child.Attachment.Placement,
			ZPriority: child.Attachment.ZPriority,
			Contract:  child.Contract,
		})
	}
	sort.SliceStable(arranged, func(i, j int) bool {
		if arranged[i].Placement.Linear.Order != arranged[j].Placement.Linear.Order {
			return arranged[i].Placement.Linear.Order < arranged[j].Placement.Linear.Order
		}
		if arranged[i].ZPriority != arranged[j].ZPriority {
			return arranged[i].ZPriority > arranged[j].ZPriority
		}
		return arranged[i].FacetID < arranged[j].FacetID
	})
	return arranged, nil
}

func listGroupChild(base *facet.Facet, markID facet.MarkID, order int) facet.GroupChild {
	if base == nil || base.LayoutRole() == nil {
		return facet.GroupChild{}
	}
	return facet.GroupChild{
		FacetID: base.ID(),
		MarkID:  markID,
		Attachment: facet.Attachment{
			Placement: facet.Placement{
				Mode:   facet.PlacementLinear,
				Linear: facet.LinearPlacement{Order: order, CrossAxisAlign: facet.CrossAxisStretch},
			},
		},
		Layout:   base.LayoutRole(),
		Contract: base.LayoutRole().Child,
	}
}

func childFacetID(mark facet.FacetImpl) facet.FacetID {
	if mark == nil {
		return 0
	}
	value := reflect.ValueOf(mark)
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return 0
	}
	base := mark.Base()
	if base == nil {
		return 0
	}
	return base.ID()
}

func cloneListEntries(entries []ListEntry) []ListEntry {
	return append([]ListEntry(nil), entries...)
}
