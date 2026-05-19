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

The current standard authored packages are:

- `marks/primitive`
- `marks/action`
- `marks/input`

The primitive family is where leaf geometry lives. It currently includes
`primitive.text` and defines `primitive.icon` as the canonical standard icon
mark. That split matters:

- ordinary consumers should use `primitive.icon`
- custom SVG authors should use the documented raw SVG facet contract
- renderer details should not leak into the public mark vocabulary

### How to use marks

- choose the smallest family that matches the intent
- prefer theme recipes for stylistic variation
- prefer anchors for relative attachment
- keep behavioral logic inside the family that owns the interaction
- treat SVG iconography as a mark contract, not as a renderer-only concern

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

Iconography follows the same split:

- `primitive.icon` resolves source, size, color slot, and density behavior through
  the standard mark contract
- the raw SVG facet contract exists for custom marks that need richer SVG control
  through `gfx/svg.SVGFacet`
- theme remains responsible for token resolution, not SVG parsing

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
- raw SVG facets describe lower-level vector geometry contracts for custom marks

The intended layering is:

`marks -> theme/animation -> runtime -> render`

with `facet` and `layout` providing the engine contracts that connect them.
