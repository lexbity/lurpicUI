# Marks, Animation, and Theme API

This document covers the user-facing APIs for marks, animation, and themes.

## Marks API

The `marks` package defines the shared vocabulary used by all standard mark
families.

Important concepts:

- `Family` identifies the broad authored family.
- `ConstructionClass` distinguishes primitive, generated, and composed marks.
- `Descriptor` describes the authored contract for a mark type.
- `Mark` is the common authored interface.
- `FocusableMark`, `AnchorExportingMark`, and `CustomizableMark` are capability interfaces.

The standard families provide concrete authored types under:

- `marks/basic`
- `marks/structure`
- `marks/annotation`
- `marks/uiinput`
- `marks/uinav`
- `marks/uinotification`
- `marks/chart`

### How to use marks

- choose the smallest family that matches the intent
- prefer theme recipes for stylistic variation
- prefer anchors for relative attachment
- keep behavioral logic inside the family that owns the interaction

## Animation API

The animation package provides interpolation and time-based transition support.

Main pieces:

- `Interpolatable[T]` for types that can lerp between values
- `EasingRegistry` for named easing curves
- `AnimatedValue[T]` for source-driven transitions
- `Timeline` for frame-ticked playback
- `KeyframeSequence[T]` for authored keyframe evaluation

### Animation rules

- animations should be driven by a source of truth, not by per-frame imperative mutation
- transitions should snap cleanly when duration is zero
- runtime-bound timelines should register and unregister with runtime lifecycle
- standalone timelines should be disposable and test-friendly

## Theme API

The theme package defines:

- tokens
- material data
- style contexts
- style derivation
- recipe resolution

Theme is intentionally split between:

- token data, which is stable and sharable
- materials and interpolatable presentation state, which may animate
- recipe resolution, which maps semantic slots to actual theme values

### Theme flow

1. Tokens define the global palette, spacing, typography, and motion values.
2. Styles derive from the current style context and interaction state.
3. Recipes map semantic slots to concrete presentation data.
4. Animated values can interpolate theme-friendly types such as colors,
   transforms, and materials.

## Package relationships

- marks describe authored intent
- theme describes presentation intent
- animation interpolates values used by marks and theme
- runtime orchestrates the frame loop that keeps them all moving

The intended layering is:

`marks -> theme/animation -> runtime -> render`

with `facet` and `layout` providing the engine contracts that connect them.
