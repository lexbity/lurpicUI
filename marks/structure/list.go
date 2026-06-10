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
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
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
	marks.Core

	Activated signal.Signal[int]

	Label         marks.Binding[string]
	SectionHeader marks.Binding[string]
	EmptyState    marks.Binding[string]
	Disabled      marks.Binding[bool]
	ItemVariant   marks.Binding[uiinput.ListItemVariant]
	Data          *store.ValueStore[[]ListEntry]
	scrollRegion  *ScrollRegion

	cachedDataSub signal.SubscriptionID

	textRole facet.TextRole

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
var _ marks.Mark = (*List)(nil)

// NewList constructs a structure.list mark with canonical defaults.
func NewList(label string, entries []ListEntry) *List {
	l := &List{
		Label:           marks.Const(label),
		SectionHeader:   marks.Const(""),
		EmptyState:      marks.Const(""),
		Disabled:        marks.Const(false),
		ItemVariant:     marks.Const(uiinput.ListItemStandard),
		Data:            store.NewValueStore(cloneListEntries(entries)),
		scrollRegion:    NewScrollRegion(label),
		cachedRows:      make(map[string]*selection.ListItem),
		cachedRowBounds: make(map[string]gfx.Rect),
	}
	l.Facet = facet.NewFacet()
	l.AddBinding(l.Label)
	l.AddBinding(l.SectionHeader)
	l.AddBinding(l.EmptyState)
	l.AddBinding(l.Disabled)
	l.AddBinding(l.ItemVariant)

	l.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   listGroupPolicy{list: l},
		Children: l,
	}
	l.Layout.Child = facet.GroupChildContract{
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
	l.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return l.measure(ctx, constraints)
	}
	l.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		l.Layout.ArrangedBounds = bounds
		l.arrange(ctx, bounds)
	}
	l.Render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := l.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	l.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return l.buildCommands(l.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	l.textRole.IMEEnabled = false
	l.RegisterRoles()
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
	l.BindImpl(l)
	return &l.Facet
}

// Descriptor satisfies marks.Mark.
func (l *List) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "structure", TypeName: "list"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (l *List) AccessibilityRole() string { return "list" }

// AccessibleName reports the semantic name source required by the spec.
func (l *List) AccessibleName() string {
	if l == nil {
		return ""
	}
	if strings.TrimSpace(l.Label.Get()) != "" {
		return strings.TrimSpace(l.Label.Get())
	}
	if strings.TrimSpace(l.SectionHeader.Get()) != "" {
		return strings.TrimSpace(l.SectionHeader.Get())
	}
	items := l.entries()
	if len(items) > 0 {
		return strings.TrimSpace(items[0].Label)
	}
	return ""
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
		out["section_header"] = rectCenter(l.cachedHeaderBounds)
	}
	if !l.cachedEmptyBounds.IsEmpty() {
		out["empty_state"] = rectCenter(l.cachedEmptyBounds)
	}
	for key, b := range l.cachedRowBounds {
		if b.IsEmpty() {
			continue
		}
		out[layout.AnchorID("item_"+key)] = rectCenter(b)
	}
	return out
}

func (l *List) OnAttach(ctx facet.AttachContext) { l.Core.OnAttach() }
func (l *List) OnActivate()                      { l.Core.OnActivate() }
func (l *List) OnDeactivate()                    { l.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (l *List) OnDetach() {
	l.Core.OnDetach()
	if l.Data != nil && l.cachedDataSub != 0 {
		l.Data.OnChange.Unsubscribe(l.cachedDataSub)
	}
	if l.scrollRegion != nil {
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
	l.Invalidate(flags)
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
		l.scrollRegion = NewScrollRegion("")
	}
	l.scrollRegion.Label = marks.Const(l.Label.Get())
	l.scrollRegion.Disabled = marks.Const(l.Disabled.Get())
	if strings.TrimSpace(l.SectionHeader.Get()) != "" {
		if l.cachedHeaderMark == nil {
			l.cachedHeaderMark = primitive.NewText(marks.Const(strings.TrimSpace(l.SectionHeader.Get())))
		} else {
			l.cachedHeaderMark.Content = marks.Const(strings.TrimSpace(l.SectionHeader.Get()))
			l.cachedHeaderMark.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
		}
		l.cachedHeaderMark.Typography = marks.Const(theme.TextLabelM)
		l.cachedHeaderMark.Foreground = marks.Const(theme.ColorTextSecondary)
		l.cachedHeaderMark.Overflow = marks.Const(primitive.TextOverflowTruncate)
		if l.cachedWritingDirection == facet.WritingDirectionRTL {
			l.cachedHeaderMark.Alignment = marks.Const(text.AlignRight)
		} else {
			l.cachedHeaderMark.Alignment = marks.Const(text.AlignLeft)
		}
	} else {
		l.cachedHeaderMark = nil
	}

	entries := l.entries()
	if len(entries) == 0 {
		if strings.TrimSpace(l.EmptyState.Get()) != "" {
			if l.cachedEmptyMark == nil {
				l.cachedEmptyMark = primitive.NewText(marks.Const(strings.TrimSpace(l.EmptyState.Get())))
			} else {
				l.cachedEmptyMark.Content = marks.Const(strings.TrimSpace(l.EmptyState.Get()))
				l.cachedEmptyMark.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
			}
			l.cachedEmptyMark.Typography = marks.Const(theme.TextBodyS)
			l.cachedEmptyMark.Foreground = marks.Const(theme.ColorTextSecondary)
			l.cachedEmptyMark.Overflow = marks.Const(primitive.TextOverflowTruncate)
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
			row = selection.NewListItem(marks.Const(entry.Label))
		}
		row.Label = marks.Const(entry.Label)
		row.SupportingText = marks.Const(entry.SupportingText)
		row.LeadingIconRef = marks.Const(entry.LeadingIconRef)
		row.Selected = marks.Const(entry.Selected)
		row.Active = marks.Const(entry.Active)
		row.Disabled = marks.Const(l.Disabled.Get() || entry.Disabled)
		row.Variant = marks.Const(l.ItemVariant.Get())
		row.ShowContainer = marks.Const(false)
		row.ShowSelectionIndicator = marks.Const(false)
		row.ShowFocusRing = marks.Const(false)
		row.ShowLeadingIcon = marks.Const(entry.LeadingIconRef != "")
		row.ShowLabel = marks.Const(true)
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
	l.scrollRegion.Gap = marks.Const(l.cachedGap)
	l.scrollRegion.children = content
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
		l.scrollRegion.Label = marks.Const(l.Label.Get())
		l.scrollRegion.Disabled = marks.Const(l.Disabled.Get())
		l.scrollRegion.Gap = marks.Const(l.cachedGap)
		result := l.scrollRegion.Layout.Measure(ctx, constraints)
		l.Layout.MeasuredSize = result.Size
		l.Layout.MeasuredResult = result
		l.textRole.Layout = nil
		return result
	}
	size := constraints.Constrain(gfx.Size{})
	l.Layout.MeasuredSize = size
	l.Layout.MeasuredResult = facet.MeasureResult{
		Size:        size,
		Intrinsic:   facet.IntrinsicSize{Min: size, Preferred: size, Max: size},
		Constraints: constraints,
	}
	l.textRole.Layout = nil
	return l.Layout.MeasuredResult
}

func (l *List) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	l.cachedBounds = bounds
	l.Layout.ArrangedBounds = bounds
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
