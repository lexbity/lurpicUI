package layout

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// WritingDirection captures the resolved text/layout flow direction.
type WritingDirection uint8

const (
	WritingDirectionLTR WritingDirection = iota
	WritingDirectionRTL
)

func (d WritingDirection) String() string {
	switch d {
	case WritingDirectionRTL:
		return "rtl"
	case WritingDirectionLTR:
		fallthrough
	default:
		return "ltr"
	}
}

// LayerLayoutKind identifies the root layout policy kind for a layer.
type LayerLayoutKind uint8

const (
	LayerLayoutNone LayerLayoutKind = iota
	LayerLayoutGrid
	LayerLayoutAnchor
	LayerLayoutFree
)

func (k LayerLayoutKind) String() string {
	switch k {
	case LayerLayoutGrid:
		return "grid"
	case LayerLayoutAnchor:
		return "anchor"
	case LayerLayoutFree:
		return "free"
	case LayerLayoutNone:
		fallthrough
	default:
		return "none"
	}
}

// GroupLayoutKind identifies the local group layout policy kind.
type GroupLayoutKind uint8

const (
	GroupLayoutNone GroupLayoutKind = iota
	GroupLayoutGrid
	GroupLayoutLinearHorizontal
	GroupLayoutLinearVertical
	GroupLayoutAnchor
	GroupLayoutFree
)

func (k GroupLayoutKind) String() string {
	switch k {
	case GroupLayoutGrid:
		return "grid"
	case GroupLayoutLinearHorizontal:
		return "linear_horizontal"
	case GroupLayoutLinearVertical:
		return "linear_vertical"
	case GroupLayoutAnchor:
		return "anchor"
	case GroupLayoutFree:
		return "free"
	case GroupLayoutNone:
		fallthrough
	default:
		return "none"
	}
}

// OverflowPolicy governs how content outside bounds is handled.
type OverflowPolicy uint8

const (
	OverflowVisible OverflowPolicy = iota
	OverflowClip
	OverflowScroll
	OverflowWrap
)

func (p OverflowPolicy) String() string {
	switch p {
	case OverflowClip:
		return "clip"
	case OverflowScroll:
		return "scroll"
	case OverflowWrap:
		return "wrap"
	case OverflowVisible:
		fallthrough
	default:
		return "visible"
	}
}

// GroupClipPolicy governs how nested group content clips.
type GroupClipPolicy uint8

const (
	GroupClipInherit GroupClipPolicy = iota
	GroupClipBounds
	GroupClipVisible
)

func (p GroupClipPolicy) String() string {
	switch p {
	case GroupClipBounds:
		return "bounds"
	case GroupClipVisible:
		return "visible"
	case GroupClipInherit:
		fallthrough
	default:
		return "inherit"
	}
}

// MainAxisSize describes how a linear group sizes its main axis.
type MainAxisSize uint8

const (
	MainAxisAuto MainAxisSize = iota
	MainAxisMin
	MainAxisMax
)

func (s MainAxisSize) String() string {
	switch s {
	case MainAxisMin:
		return "min"
	case MainAxisMax:
		return "max"
	case MainAxisAuto:
		fallthrough
	default:
		return "auto"
	}
}

// CrossAxisAlignment describes how linear groups align the cross axis.
type CrossAxisAlignment uint8

const (
	CrossAxisStart CrossAxisAlignment = iota
	CrossAxisCenter
	CrossAxisEnd
	CrossAxisStretch
	CrossAxisBaseline
)

func (a CrossAxisAlignment) String() string {
	switch a {
	case CrossAxisCenter:
		return "center"
	case CrossAxisEnd:
		return "end"
	case CrossAxisStretch:
		return "stretch"
	case CrossAxisBaseline:
		return "baseline"
	case CrossAxisStart:
		fallthrough
	default:
		return "start"
	}
}

// ResolvedScalar is a concrete runtime layout scalar.
type ResolvedScalar float32

// Float32 returns the scalar as a float32.
func (s ResolvedScalar) Float32() float32 { return float32(s) }

// IsZero reports whether the scalar is zero.
func (s ResolvedScalar) IsZero() bool { return s == 0 }

// ResolvedOptionalScalar is a concrete optional layout scalar.
type ResolvedOptionalScalar struct {
	Value ResolvedScalar
	Valid bool
}

// OptionalScalar constructs a valid optional scalar.
func OptionalScalar(value float32) ResolvedOptionalScalar {
	return ResolvedOptionalScalar{Value: ResolvedScalar(value), Valid: true}
}

// Float32 returns the optional value and whether it is valid.
func (s ResolvedOptionalScalar) Float32() (float32, bool) {
	if !s.Valid {
		return 0, false
	}
	return s.Value.Float32(), true
}

// OrZero returns the concrete value or zero when invalid.
func (s ResolvedOptionalScalar) OrZero() float32 {
	if !s.Valid {
		return 0
	}
	return s.Value.Float32()
}

// ResolvedGridConfig describes a concrete grid layout contract.
type ResolvedGridConfig struct {
	Columns   int
	Rows      int
	ColumnGap ResolvedScalar
	RowGap    ResolvedScalar
	Margin    gfx.Insets
}

// ResolvedAnchorConfig describes a concrete anchor placement contract.
type ResolvedAnchorConfig struct {
	Gap     ResolvedScalar
	OffsetX ResolvedScalar
	OffsetY ResolvedScalar
}

// ResolvedFreeConfig describes a concrete free-placement contract.
type ResolvedFreeConfig struct {
	X      ResolvedScalar
	Y      ResolvedScalar
	Width  ResolvedOptionalScalar
	Height ResolvedOptionalScalar
}

// ResolvedAnchorFreeConfig groups anchor and free placement contracts.
type ResolvedAnchorFreeConfig struct {
	Anchor ResolvedAnchorConfig
	Free   ResolvedFreeConfig
}

// ResolvedLinearConfig describes a concrete linear group contract.
type ResolvedLinearConfig struct {
	Horizontal     bool
	Gap            ResolvedScalar
	CrossAxisAlign CrossAxisAlignment
	MainAxisSize   MainAxisSize
}

// ResolvedLayerLayoutRecipe is the resolved root layout config for a layer.
type ResolvedLayerLayoutRecipe struct {
	PolicyKind LayerLayoutKind
	Grid       ResolvedGridConfig
	Anchor     ResolvedAnchorConfig
	Free       ResolvedFreeConfig
	Insets     gfx.Insets
	Clip       ClipPolicy
}

// ResolvedGroupLayoutRecipe is the resolved local layout config for a group.
type ResolvedGroupLayoutRecipe struct {
	PolicyKind GroupLayoutKind
	Grid       ResolvedGridConfig
	Linear     ResolvedLinearConfig
	AnchorFree ResolvedAnchorFreeConfig
	Overflow   OverflowPolicy
	Clipping   GroupClipPolicy
	Insets     gfx.Insets
}

// GroupLayoutRecipeRef points at an app/theme group layout recipe.
type GroupLayoutRecipeRef struct {
	Family string
	Name   string
}

// String returns a stable textual key for the group recipe reference.
func (r GroupLayoutRecipeRef) String() string {
	if r.Family == "" && r.Name == "" {
		return ""
	}
	if r.Family == "" {
		return r.Name
	}
	if r.Name == "" {
		return r.Family
	}
	return fmt.Sprintf("%s/%s", r.Family, r.Name)
}

// IsZero reports whether the recipe reference is empty.
func (r GroupLayoutRecipeRef) IsZero() bool {
	return r.Family == "" && r.Name == ""
}

// DefaultGridConfig returns the foundation's 5x5 grid fallback.
func DefaultGridConfig() ResolvedGridConfig {
	return ResolvedGridConfig{
		Columns: 5,
		Rows:    5,
	}
}

// DefaultLayerLayoutRecipe returns the 5x5 grid fallback layer recipe.
func DefaultLayerLayoutRecipe() ResolvedLayerLayoutRecipe {
	return ResolvedLayerLayoutRecipe{
		PolicyKind: LayerLayoutGrid,
		Grid:       DefaultGridConfig(),
	}
}

// DefaultGroupLayoutRecipe returns the empty group contract.
func DefaultGroupLayoutRecipe() ResolvedGroupLayoutRecipe {
	return ResolvedGroupLayoutRecipe{
		PolicyKind: GroupLayoutNone,
	}
}
