package facet

import "codeburg.org/lexbit/lurpicui/gfx"

// LayerID identifies a globally registered layer in layout participation data.
type LayerID uint64

// DensityID identifies an app-defined density scale.
type DensityID string

// WritingDirection captures layout flow direction for a frame snapshot.
type WritingDirection uint8

const (
	WritingDirectionLTR WritingDirection = iota
	WritingDirectionRTL
)

// InputModality describes the current interaction mode affecting projection and cache invalidation.
type InputModality uint8

const (
	InputModalityUnknown InputModality = iota
	InputModalityPointer
	InputModalityTouch
	InputModalityKeyboard
	InputModalityMixed
)

// Alignment places smaller children within a container or cell.
type Alignment uint8

const (
	AlignStretch Alignment = iota
	AlignStart
	AlignCenter
	AlignEnd
	AlignTopLeft
	AlignTopCenter
	AlignTopRight
	AlignCenterLeft
	AlignCenterRight
	AlignBottomLeft
	AlignBottomCenter
	AlignBottomRight
	AlignBaseline
)

// GroupLayoutKind identifies a local layout contract.
type GroupLayoutKind uint8

const (
	GroupLayoutNone GroupLayoutKind = iota
	GroupLayoutGrid
	GroupLayoutLinearHorizontal
	GroupLayoutLinearVertical
	GroupLayoutAnchor
	GroupLayoutFree
)

// OverflowPolicy governs how content outside bounds is handled.
type OverflowPolicy uint8

const (
	OverflowVisible OverflowPolicy = iota
	OverflowClip
	OverflowScroll
	OverflowWrap
)

// GroupClipPolicy governs how nested group content clips.
type GroupClipPolicy uint8

const (
	GroupClipInherit GroupClipPolicy = iota
	GroupClipBounds
	GroupClipVisible
)

// HitPolicy controls how hits traverse a layer or facet output.
type HitPolicy uint8

const (
	HitNormal HitPolicy = iota
	HitPassThrough
	HitBlockBelow
	HitDisabled
)

// ClipPolicy controls how a layer or group clips children.
type ClipPolicy uint8

const (
	ClipNone ClipPolicy = iota
	ClipToParent
	ClipToContent
	ClipToViewport
)

// DismissalTrigger identifies a dismissal input path.
type DismissalTrigger uint8

const (
	DismissalTriggerPointer DismissalTrigger = iota
	DismissalTriggerKey
	DismissalTriggerFocusLoss
)

// DismissalTriggerSet is a bitset of enabled dismissal triggers.
type DismissalTriggerSet uint8

const (
	DismissalTriggerSetPointer DismissalTriggerSet = 1 << iota
	DismissalTriggerSetKey
	DismissalTriggerSetFocusLoss
)

// OrderRange describes a half-open order range.
type OrderRange struct {
	Min int32
	Max int32
}

// DismissalScope controls layer-level outside dismissal behavior.
type DismissalScope struct {
	Enabled      bool
	BehindOrders OrderRange
	Triggers     DismissalTriggerSet
}

// MainAxisSize describes how a linear group sizes its main axis.
type MainAxisSize uint8

const (
	MainAxisAuto MainAxisSize = iota
	MainAxisMin
	MainAxisMax
)

// CrossAxisAlignment describes how linear groups align the cross axis.
type CrossAxisAlignment uint8

const (
	CrossAxisStart CrossAxisAlignment = iota
	CrossAxisCenter
	CrossAxisEnd
	CrossAxisStretch
	CrossAxisBaseline
)

// PlacementMode describes how a child asks to be arranged.
type PlacementMode uint8

const (
	PlacementGrid PlacementMode = iota
	PlacementAnchor
	PlacementFree
	PlacementLinear
)

// PlacementModeSet is a bitset of supported placement modes.
type PlacementModeSet uint16

const (
	SupportsGrid PlacementModeSet = 1 << iota
	SupportsAnchor
	SupportsFree
	SupportsLinear
)

// Has reports whether the set supports a placement mode.
func (s PlacementModeSet) Has(mode PlacementMode) bool {
	switch mode {
	case PlacementGrid:
		return s&SupportsGrid != 0
	case PlacementAnchor:
		return s&SupportsAnchor != 0
	case PlacementFree:
		return s&SupportsFree != 0
	case PlacementLinear:
		return s&SupportsLinear != 0
	default:
		return false
	}
}

// ResolvedScalar is a concrete runtime layout scalar.
type ResolvedScalar float32

// ResolvedOptionalScalar is a concrete optional layout scalar.
type ResolvedOptionalScalar struct {
	Value ResolvedScalar
	Valid bool
}

// OptionalScalar constructs a valid optional scalar.
func OptionalScalar(value float32) ResolvedOptionalScalar {
	return ResolvedOptionalScalar{Value: ResolvedScalar(value), Valid: true}
}

// GridPlacement describes numeric line-based placement.
type GridPlacement struct {
	ColStart int
	RowStart int
	ColSpan  int
	RowSpan  int
}

// AnchorID is a stable semantic name for an exported anchor.
type AnchorID string

// AnchorSide describes the placement side relative to an anchor.
type AnchorSide uint8

const (
	AnchorAbove AnchorSide = iota
	AnchorBelow
	AnchorLeft
	AnchorRight
	AnchorCenter
)

// AnchorPlacement describes placement relative to an anchor.
type AnchorPlacement struct {
	AnchorRef AnchorID
	Side      AnchorSide
	Gap       ResolvedScalar
	OffsetX   ResolvedScalar
	OffsetY   ResolvedScalar
}

// FreePlacement describes absolute placement in a parent coordinate space.
type FreePlacement struct {
	X      ResolvedScalar
	Y      ResolvedScalar
	Width  ResolvedOptionalScalar
	Height ResolvedOptionalScalar
}

// LinearPlacement describes order/alignment participation in a linear group.
type LinearPlacement struct {
	Order          int
	CrossAxisAlign CrossAxisAlignment
	MainAxisSize   MainAxisSize
}

// Placement selects the arrangement contract for a child.
type Placement struct {
	Mode   PlacementMode
	Grid   GridPlacement
	Anchor AnchorPlacement
	Free   FreePlacement
	Linear LinearPlacement
	Align  Alignment
}

// StretchAxisPolicy controls stretch behavior per axis.
type StretchAxisPolicy uint8

const (
	StretchNever StretchAxisPolicy = iota
	StretchWhenParentRequests
	StretchAlways
)

// StretchPolicy describes stretch behavior for both axes.
type StretchPolicy struct {
	Width  StretchAxisPolicy
	Height StretchAxisPolicy
}

// CompressionBehavior describes how to resolve content below minimum size.
type CompressionBehavior uint8

const (
	CompressionTruncate CompressionBehavior = iota
	CompressionWrap
	CompressionCollapse
	CompressionOverflow
	CompressionClip
	CompressionRefuse
)

// ExpansionBehavior describes how to resolve content above maximum size.
type ExpansionBehavior uint8

const (
	ExpansionClip ExpansionBehavior = iota
	ExpansionOverflow
	ExpansionRefuse
)

// ConstraintPolicy describes how a child responds to parent constraints.
type ConstraintPolicy struct {
	BelowMinWidth  CompressionBehavior
	BelowMinHeight CompressionBehavior
	AboveMaxWidth  ExpansionBehavior
	AboveMaxHeight ExpansionBehavior
}

// BaselinePolicy reserves a baseline contract for future text/inline support.
type BaselinePolicy uint8

const (
	BaselineNone BaselinePolicy = iota
)

// IntrinsicSize captures a child’s intrinsic size hints.
type IntrinsicSize struct {
	Min       gfx.Size
	Preferred gfx.Size
	Max       gfx.Size
}

// IntrinsicSizeFunc resolves a child’s intrinsic size hints.
type IntrinsicSizeFunc func(ctx MeasureContext, constraints Constraints) IntrinsicSize

// MeasureContext carries the current layout snapshot for measurement.
type MeasureContext struct {
	Runtime          RuntimeServices
	Theme            any
	Layer            LayerContext
	ParentGroup      GroupParentContract
	ChildGroup       GroupChildContract
	ContentScale     float32
	Density          DensityID
	WritingDirection WritingDirection
}

// ArrangeContext carries the current layout snapshot for arrangement.
type ArrangeContext struct {
	Runtime     RuntimeServices
	Theme       any
	Layer       LayerContext
	ParentGroup GroupParentContract
	ChildGroup  GroupChildContract
	Placement   Placement
}

// MeasureResult is the concrete measurement result returned by LayoutRole.
type MeasureResult struct {
	Size        gfx.Size
	Intrinsic   IntrinsicSize
	Constraints Constraints
}

// LayerContext carries resolved layer metadata into layout callbacks.
type LayerContext struct {
	ID         LayerID
	HitPolicy  HitPolicy
	ClipPolicy ClipPolicy
	Dismissal  DismissalScope
	Order      int32
}

// GroupMeasureContext carries parent group snapshot data during measurement.
type GroupMeasureContext struct {
	MeasureContext
	Bounds gfx.Rect
}

// GroupArrangeContext carries parent group snapshot data during arrangement.
type GroupArrangeContext struct {
	ArrangeContext
	Bounds gfx.Rect
}

// GroupMeasureResult is the result of measuring a group.
type GroupMeasureResult struct {
	Size gfx.Size
}

// ArrangedGroupChild captures a child’s arranged bounds and diagnostics.
type ArrangedGroupChild struct {
	FacetID   FacetID
	MarkID    MarkID
	Bounds    gfx.Rect
	Placement Placement
	ZPriority int32
	Contract  GroupChildContract
}

// GroupChild is the narrow view of an immediate child inside a local group.
type GroupChild struct {
	FacetID    FacetID
	MarkID     MarkID
	Attachment Attachment
	Layout     *LayoutRole
	Contract   GroupChildContract
}

// ChildSource enumerates the immediate child source for a group parent.
type ChildSource interface {
	Children() []GroupChild
}

// GroupLayoutPolicy arranges children within a facet-local group.
type GroupLayoutPolicy interface {
	Kind() GroupLayoutKind
	MeasureGroup(ctx GroupMeasureContext, children []GroupChild) (GroupMeasureResult, error)
	ArrangeGroup(ctx GroupArrangeContext, children []GroupChild) ([]ArrangedGroupChild, error)
}

// GroupParentContract describes how a facet arranges its immediate children.
type GroupParentContract struct {
	Kind     GroupLayoutKind
	Policy   GroupLayoutPolicy
	Overflow OverflowPolicy
	Clipping GroupClipPolicy
	Children ChildSource
}

// GroupChildContract describes how a facet participates in its parent group.
type GroupChildContract struct {
	SupportedPlacement PlacementModeSet
	Intrinsic          IntrinsicSizeFunc
	Constraints        ConstraintPolicy
	Stretch            StretchPolicy
	Baseline           BaselinePolicy
}

// Attachment records the layer/placement contract for a child.
type Attachment struct {
	LayerID   LayerID
	Placement Placement
	ZPriority int32
}

// LayoutRole participates in measurement and arrangement inside the resolved layer contract.
type LayoutRole struct {
	Constraints    Constraints
	MeasuredResult MeasureResult
	MeasuredSize   gfx.Size
	ArrangedBounds gfx.Rect

	OnMeasure func(ctx MeasureContext, constraints Constraints) MeasureResult
	OnArrange func(ctx ArrangeContext, bounds gfx.Rect)

	Parent GroupParentContract
	Child  GroupChildContract
}

// Measure updates the cached measurement and returns the measurement result.
func (r *LayoutRole) Measure(ctx MeasureContext, c Constraints) MeasureResult {
	if r == nil {
		return MeasureResult{}
	}
	r.Constraints = c
	if ctx.ParentGroup == (GroupParentContract{}) {
		ctx.ParentGroup = r.Parent
	}
	if isZeroGroupChildContract(ctx.ChildGroup) {
		ctx.ChildGroup = r.Child
	}
	if r.OnMeasure == nil {
		r.MeasuredResult = MeasureResult{Constraints: c}
		r.MeasuredSize = gfx.Size{}
		return r.MeasuredResult
	}
	result := r.OnMeasure(ctx, c)
	result.Constraints = c
	r.MeasuredResult = result
	r.MeasuredSize = result.Size
	return result
}

// Arrange updates the arranged bounds and notifies the callback.
func (r *LayoutRole) Arrange(ctx ArrangeContext, bounds gfx.Rect) {
	if r == nil {
		return
	}
	if ctx.ParentGroup == (GroupParentContract{}) {
		ctx.ParentGroup = r.Parent
	}
	if isZeroGroupChildContract(ctx.ChildGroup) {
		ctx.ChildGroup = r.Child
	}
	if ctx.ChildGroup.SupportedPlacement != 0 && !ctx.ChildGroup.SupportedPlacement.Has(ctx.Placement.Mode) {
		panic("facet contract violation: unsupported placement mode; guidance: update SupportedPlacement to include the requested placement mode")
	}
	r.ArrangedBounds = bounds
	if r.OnArrange != nil {
		r.OnArrange(ctx, bounds)
	}
}

func (r *LayoutRole) onAttach(f *Facet) {
	if r.OnMeasure == nil {
		panic("facet contract violation: layout role requires OnMeasure; guidance: provide a measurement callback before attaching the facet")
	}
	if r.Parent.Kind != GroupLayoutNone && r.Parent.Policy == nil {
		panic("facet contract violation: parent group requires Policy when Parent.Kind is not GroupLayoutNone; guidance: supply a group policy")
	}
	if r.Parent.Kind != GroupLayoutNone && r.Child.SupportedPlacement == 0 {
		panic("facet contract violation: parent group requires a child placement contract when Parent.Kind is not GroupLayoutNone; guidance: set Child.SupportedPlacement")
	}
}

func (r *LayoutRole) onActivate(f *Facet)   {}
func (r *LayoutRole) onDeactivate(f *Facet) {}
func (r *LayoutRole) onDispose(f *Facet) {
	r.OnMeasure = nil
	r.OnArrange = nil
	r.Parent = GroupParentContract{}
	r.Child = GroupChildContract{}
	r.MeasuredResult = MeasureResult{}
	r.MeasuredSize = gfx.Size{}
	r.ArrangedBounds = gfx.Rect{}
}

func isZeroGroupChildContract(c GroupChildContract) bool {
	return c.SupportedPlacement == 0 && c.Intrinsic == nil && c.Constraints == (ConstraintPolicy{}) && c.Stretch == (StretchPolicy{}) && c.Baseline == BaselineNone
}
