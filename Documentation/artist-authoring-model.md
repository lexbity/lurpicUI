# Artist Authoring Model (Stale — superseded by marks.md)

**This document describes the pre-rewrite model and is retained for reference
only.** The post-rewrite model is documented in `marks.md`.

The pre-rewrite model used `facet.Facet` embedding, imperative `SetX` methods,
and dual `OnCollect`/`OnProject` render paths. The post-rewrite model uses
`marks.Core` embedding, `marks.Binding[T]` config, and a single `BuildCommands`
render path.

---

This document describes the mark-based authoring model used to build UI,
annotation, chart, and navigation scenes.

## Core idea

Marks are authored objects. They describe intent:

- what kind of visual or interactive element is needed
- how it should adapt to theme tokens
- how it should host or compose children
- how it should export anchors, focus, or gesture behavior

Marks are not the runtime itself. They are authored declarations that are
resolved into facet trees and runtime behavior.

## Families

The standard library groups marks by family:

- `primitive` for leaf geometry and control, including `primitive.text` and `primitive.icon`
- `action` for interactive controls such as buttons
- `input` for text-field and editing surfaces

The broader mark model still uses the same authoring vocabulary for other
families, but iconography is intentionally split out at the primitive level:

- standard consumers use `primitive.icon`
- custom SVG authors use the documented raw SVG facet contract
- SVG support is therefore a mark contract, not just a renderer capability

Each family should own its own authored vocabulary while sharing the same core
descriptor and runtime contracts.

## Construction classes

Construction class expresses how a mark is created:

- primitive marks correspond closely to renderable geometry
- generated marks are typically produced by theme recipe resolution or composition
- composed marks may host children and attach them to specific runtime layers

For primitive marks, the iconography boundary is explicit:

- `primitive.text` is the canonical text primitive
- `primitive.icon` is the canonical SVG icon primitive
- multi-color or higher-control SVG authored geometry belongs to the raw SVG
  facet contract instead of the standard icon mark
- the raw SVG facet helper lives in `gfx/svg.SVGFacet` for authors who want
  normalized SVG geometry without adopting `primitive.icon`

This is important because it lets the runtime and tooling distinguish between
directly authored geometry and generated presentation artifacts.

## Descriptor registry

Every public mark should have a descriptor that identifies:

- family
- construction class
- type name
- whether it exports anchors
- whether it is focusable
- whether it is hit testable

The registry is a contract for tooling, diagnostics, and documentation.

## Recipe resolution

Theme recipes separate style from structure:

- slots describe semantic regions such as backgrounds, labels, icons, handles, or indicators
- a recipe resolver maps a variant key to a slot set
- the runtime can then style a mark family without the family hardcoding color and typography choices

This keeps authored mark code focused on behavior and geometry, not theme policy.

## Composition and anchoring

The authoring model relies heavily on composition:

- marks may host child marks
- anchors can be exported from a child and consumed by siblings
- structure marks help attach, transform, clip, or mount content into a target layer

Anchors are the preferred way to express relative attachment between marks because they survive layout changes better than absolute coordinates.

## Interaction design

Interactive marks should expose intent clearly:

- focusability should be explicit
- gesture participation should be part of the authored mark, not ad hoc runtime code
- selection, drag, and keyboard semantics should be resolved through the mark family

This allows the runtime input system to remain generic.

## Implementation guidance

When adding a new mark:

1. Define the descriptor first.
2. Decide whether the mark is primitive, composed, or generated.
3. Add slots and recipe support if the mark is themeable.
4. Export anchors for meaningful attachment points.
5. Add tests for hit testing, projection, and anchor behavior.
6. Keep the mark family free of unrelated sibling-family imports unless composition requires it.
7. If the mark consumes SVG, document whether it is a standard icon mark or a raw SVG facet.

The authoring model should make it easy to describe intent and hard to
accidentally embed runtime policy into the mark itself.
