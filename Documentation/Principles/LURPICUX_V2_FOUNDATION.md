# LurpicUX V2 Foundation

## Status: Draft — Pre-mark-spec authoring reference

This document establishes the architectural contracts that every V2 mark spec must satisfy.
It sits between the Facet Runtime Model principles (the runtime's why) and the individual
mark schemas (the marks' what). If a mark spec contradicts anything here, the mark spec is wrong.

---

## 1. The Layer Model

### 1.1 What a Layer Is

A layer is a **viewport-wide, globally-ordered rendering and layout surface**. Every piece of
visual output in an application belongs to exactly one layer. The runtime owns layer ordering.
No mark or facet resolves its own z-position relative to another mark or facet outside of
group-local layout.

Layers are not containers in the facet tree. They are named surfaces that facets and marks
*target*. The facet tree remains hierarchical; layers provide a flat, globally-ordered
compositing model on top of it.

### 1.2 The Standard Layer Interface

The standard marks library defines a registry that enumerates the standard layers.
Any application using the standard marks library must register the standard layer set. The
compiler enforces that marks only reference declared layer IDs — unknown layer IDs are a runtime panic.

Standard layers, in render order (back to front):

```
StandardLayer_Background // behind base content: wallpapers, canvas backdrops, scene backgrounds
StandardLayer_Base       // primary application content; structured UI
StandardLayer_Spatial    // transformed coordinate space: charts, graphs, maps, canvas marks
                         // this is where pan/zoom transforms live
StandardLayer_Foreground // above base, below floating: persistent chrome (fixed toolbars,
                         // sticky headers, persistent side rails)
StandardLayer_Floating   // non-modal overlays: tooltips, hover cards, popovers
StandardLayer_Overlay    // panels, drawers, sidesheets; non-blocking but prominent
StandardLayer_Modal      // dialogs, blocking overlays; input blocked behind
StandardLayer_Status     // above modal: system-level status (connection lost banner,
                         // global progress indicator); never obscured
StandardLayer_Debug      // diagnostic overlays; never ships in production
```

Applications may define additional typed layer IDs for custom marks and facets. Custom layer
IDs are registered at application startup. The standard marks library never references custom
layer IDs — the dependency goes one direction only.

### 1.3 What a Layer Owns

Each layer owns:

- A **grid configuration**: column count, row count, gutter, margin, and track sizing modes.
  The grid is available to all children of the layer. Children may use grid placement or
  anchor-based placement; neither is mandatory, but marks that are UI layout elements must
  declare grid placement.
- A **clip boundary**: by default, the layer clips to the viewport. Layers may declare
  overflow behavior (clip, scroll, visible).
- A **coordinate space**: all layers share the viewport coordinate space unless a layer
  explicitly declares a transformed coordinate space (e.g. a canvas layer with pan/zoom).
- A **hit policy**: whether input passes through to layers behind it when no hit target
  is found (pass-through for Floating, blocked for Modal).
- A **dismissal scope**: which layers are considered "behind" this one for outside-click
  dismissal purposes.

### 1.4 Layer Hit Testing

The hit testing pipeline (`input/routing.go`, `runtime/runtime_hit.go`) is unaffected
by the global layer namespace rework. The `layeredHitMap` already sorts entries by
`RenderOrder` and `ZPriority`, applies `HitPolicy` per layer, clips to `ClipRect`,
and transforms screen coordinates to local space via inverse transform. That machinery
is correct and does not change.

What changes is the **source** of `RenderOrder` values. Currently they are parent-scoped
integers — two different parents can both declare `RenderOrder: 10` with no global
relationship between them. After the rework, `RenderOrder` is derived from the global
layer tier. `StandardLayer_Modal` always sorts above `StandardLayer_Floating` regardless
of which parent declared it. The sorting logic is identical; the values are now
globally consistent.

The four `HitPolicy` values map directly to the `layer_contract.hit_policy` field in
every mark spec:

- `HitNormal` → `hit_policy: normal` — stop at first hit
- `HitPassThrough` → `hit_policy: pass_through` — if hit, continue testing below
- `HitBlockBelow` → `hit_policy: block_below` — block traversal below this layer
- `HitDisabled` → `hit_policy: disabled` — facet skipped entirely

This is a direct correspondence. The spec field drives the runtime value with no
translation layer.

**Diagnostic implication:** The hit region diagnostic overlay must display the global
layer tier for each hit region, not just bounds and MarkID. Seeing that a tooltip's
hit region is correctly in `StandardLayer_Floating` versus accidentally in
`StandardLayer_Base` is the primary diagnostic for overlay placement bugs — exactly
the class of bug that caused V1 placement failures.

### 1.4 Layer Grid Configuration

Each layer declares its own grid. The grid is the layout policy for direct children of the
layer. Grid configuration is defined at layer registration time, not per-mark.

Grid placement for children uses the standard `Attachment.Placement` fields (col/row start,
span, alignment). Anchoring is an alternative — children that cannot express their placement
as a grid cell (e.g. a tooltip anchored to a trigger point) use anchor-based placement instead.

Mark specs must declare which placement strategy they use within a layer:
- `grid` — the mark declares col/row placement as part of its layout contract
- `anchor` — the mark declares which anchor it attaches to and its offset policy
- `free` — the mark declares its own absolute position within the layer coordinate space
  (allowed only for spatial/canvas marks)

### 1.5 Groups vs Layers

A **group** is mark-scale. It is a local composition unit within a layer. Groups have their
own internal layout policy. They do not escape their layer.

| Concern | Layer | Group |
|---|---|---|
| Scope | Viewport-wide, global | Mark-local |
| Identity | Global typed ID | Local to parent |
| Layout policy | Grid with anchoring | Grid, linear (H/V), or free/anchor |
| Z-ordering | Globally enforced by layer stack | Local to group, parent-managed |
| Clip boundary | Viewport or declared overflow | Parent bounds or declared overflow |
| Can escape parent clip? | Yes — layers are viewport-wide | No — groups live within their layer |

### 1.6 Group Composition — Groups Within Groups

Marks compose hierarchically. A `structure.card` contains an `action.button` which
contains a `primitive.text` or `primitive.icon`. Each mark in this chain has its own
group contract.
Every group is simultaneously a **child participant** in its parent's group and a
**layout owner** for its own children. Both relationships must be declared explicitly
in every mark spec.

This mirrors the CSS box model's core insight: every element is both a formatting
context for its children and a participant in its parent's formatting context.
Composability comes from declaring both sides of the contract, not just one.

**The two sides of the group contract:**

`as_parent` — how this mark arranges its own children. Declares layout policy (grid,
linear, free/anchor), configuration, and overflow behavior. `not_applicable` for
primitive marks with no children, but always present in every spec so it is explicitly
considered rather than silently omitted.

`as_child` — how this mark participates in a parent's group. Declares which placement
strategies it supports (grid cell, anchor, free), what intrinsic size it reports to
the parent for track sizing, whether it can stretch to fill a cell or only sizes to
intrinsic, whether it exports a baseline for row alignment, and what happens when a
parent attempts to place it in a context it does not support (programming contract
violation → panic in debug builds per Facet Runtime Model Principle 11).

Every mark declares both sides. A primitive with no children has `as_parent:
not_applicable` but still has a fully declared `as_child`. A composed mark like a
card has both sides fully specified.

**Why this matters for custom facets:**

Custom facets placed inside mark trees must satisfy the `as_child` contract of the
group they are placed in. A custom facet that cannot declare its placement strategy
and intrinsic size is incompatible with that group. This is the same contract a mark
satisfies — custom facets are not exempt. This is what makes marks and custom facets
interoperable: they speak the same placement language.

Primitive leaf marks are part of that same contract surface:

- `primitive.text` covers shaped text geometry
- `primitive.icon` covers the standard SVG iconography contract
- custom SVG authors use the separate raw SVG facet contract when they need richer
  geometry or paint control

**Depth is unbounded but contracts are local:**

There is no limit on group nesting depth. A card can contain a panel can contain a
list can contain a list item can contain a button can contain an icon and a text mark.
Each group only needs to know its immediate children's `as_child` contracts. The
runtime resolves the tree depth-first; no group needs to know about its grandchildren.

### 1.7 Group Layout Policies

Groups declare one of three layout policies in their mark spec:

**Grid** — children declare col/row placement. Same track sizing model as layer grids
(fixed, intrinsic, flex). Used for structured 2D composition: form layouts, card grids,
inspector panels.

**Linear** — children arrange in a single axis (horizontal or vertical). Children declare
order and alignment; the group measures the axis. Used for toolbars, tab bars, breadcrumbs,
button rows, stacked content.

**Free/Anchor** — children declare position relative to the group's bounds or relative to
named anchor points. Used for canvas-like composition within a mark: a card with a floating
badge, a chart with overlay controls, a node with port handles.

A group's layout policy is declared in its mark spec and does not change at runtime.
If a use case requires switching layout policy, it requires a different group mark, not
a runtime-configurable group.

---

## 2. The Context Model

Marks and facets consume three categories of context. The categories differ in how they
change, how they propagate, and what they invalidate.

### 2.1 Context Categories

**Category 1 — Visual theme context**
*What:* Color slots, variant materials, motion parameters, border radii, elevation values.
*Channel:* Theme context snapshot, resolved via `NearestStyleContext()` at projection time.
*Changes:* Infrequent, broad (whole-app color scheme switch, theme swap).
*Invalidates:* Projection only. Layout is unaffected (colors don't change size).
*Mark obligation:* Include resolved color/material slot versions in projection cache key.
  Re-project when theme context version changes. Do not capture raw color values at attach —
  resolve from theme context at each projection pass.

**Category 2 — Layout style context**
*What:* Density, writing direction, spacing scale, typography scale definitions.
*Channel:* Live style context, walked via `NearestStyleContext()`. Subscribable.
*Changes:* Infrequent but impactful. May be scoped (subtree density override).
*Invalidates:* Layout AND projection. Density changes measurement; writing direction
  changes arrangement; spacing changes intrinsic size.
*Mark obligation:* Subscribe to style context changes during OnAttach. On change:
  mark DirtyLayout | DirtyProjection. Include density and writing direction in layout
  cache key and projection cache key.

**Category 3 — Runtime properties**
*What:* Content scale (HiDPI multiplier), input modality (mouse/touch/pen/keyboard/gamepad).
*Channel:* Runtime context, injected by the runtime into the frame pipeline.
*Changes:* Can change during application lifetime (display change, input device change).
*Invalidates:* Layout AND projection for content scale. Projection only for input modality
  (hit region sizes may change but arrangement does not).
*Mark obligation:* Content scale must be propagated into every shaping call for text-bearing
  marks. Input modality must be included in projection cache key for marks that change
  hit region size or visible affordances based on input mode. Both must be included in
  the child projection context passed to child marks.

### 2.2 What "Static at attach" Means in V2

No context category change requires adding or removing facets from the tree. Color scheme
switches, density changes, and content scale changes are handled entirely through dirty flag
invalidation — DirtyProjection for visual-only changes, DirtyLayout | DirtyProjection for
measurement-affecting changes.

The attach phase is never repeated for a living facet in response to context changes.
Marks must not design their context consumption in a way that requires re-attaching to pick
up changes. Everything they need must be re-resolvable at layout time or projection time
from the current context, not from what was captured at attach.

The V1 pattern of baking `DefaultTokens()` at mark initialization is explicitly forbidden
in V2. The attach phase is for: subscribing to stores, subscribing to style context,
registering roles, and establishing the facet's participation contracts. It is not for
capturing context values that will be used at projection time.

### 2.3 Token Classification

Theme tokens are classified by which category they belong to, which determines what
a mark must do when they change:

| Token type | Category | Change invalidates |
|---|---|---|
| Color, material, elevation, radius, shadow | 1 — Visual | Projection |
| Spacing, density, typography scale definitions | 2 — Layout style | Layout + Projection |
| Motion duration, easing | 1 — Visual | Projection (animation parameters) |
| Writing direction, locale hints | 2 — Layout style | Layout + Projection |
| Content scale | 3 — Runtime | Layout + Projection |
| Input modality | 3 — Runtime | Projection |

Marks must declare in their `invalidation_triggers` section which token categories they
consume. This is the machine-checkable part of the spec (manual QA in V2, automated in V3).

---

## 3. Interoperability Contract

These rules apply to every mark in the standard library and every custom mark or facet
that participates in a mark tree. A mark that cannot satisfy all applicable rules for
its construction class is not a stable mark.

### Rule 1 — Every visual output belongs to exactly one layer

Marks declare their target layer using a typed standard layer ID (or a registered custom
layer ID). The runtime owns placement within that layer. No mark self-determines its
z-position relative to marks in other layers.

Marks whose visual output is entirely local (renders within parent bounds, no overlay) target
`StandardLayer_Base` implicitly. Marks that produce overlay content must explicitly declare
their target layer.

### Rule 2 — Groups own local layout; layers own global placement

A mark that composes children uses either a group (local layout) or targets a layer (global
placement). No mark spans both concerns simultaneously. A dropdown mark, for example,
places its trigger content within its parent group and places its expanded surface into
`StandardLayer_Modal` — two separate concerns, declared separately.

### Rule 3 — Visual theme context is resolved at projection time, not at attach time

Marks do not capture raw color values, material values, or variant tokens at attach. At each
projection pass, marks resolve their slots from the current theme context. The resolved slot
version is included in the projection cache key. This means theme switches invalidate
projection automatically without any additional mark code.

### Rule 4 — Layout style context is subscribed at attach time and re-measured on change

Marks that consume density, spacing scale, or writing direction subscribe during OnAttach.
On change, they mark DirtyLayout | DirtyProjection. They do not re-attach or reconstruct.
Density and writing direction are included in layout and projection cache keys.

### Rule 5 — Content scale is propagated, not captured

Text-bearing marks receive content scale from the runtime context at shaping time. They
do not capture content scale at attach. They propagate content scale into their child
projection context. Cache keys for text layout include content scale. Marks that contain
text-bearing children propagate content scale even if they do not directly shape text.

### Rule 6 — Overlay marks declare layer tier, hit policy, and dismissal contract

Any mark that produces content in a layer above `StandardLayer_Base` must declare:
- Target layer ID
- Whether it blocks input behind it (hit policy)
- What triggers dismissal (Escape key, outside-click, programmatic, non-dismissable)
- Whether focus is trapped within it while open
- Where focus returns on close

This is not optional for overlay marks. A tooltip that doesn't declare dismissal is
an incomplete spec.

### Rule 7 — Text-bearing marks follow the typography contract

All text rendered by a mark is shaped by the text subsystem (TextRole). Marks do not
guess glyph metrics or hardcode line heights. Typography scale (size, weight, line height)
comes from resolved theme slots (Category 1). Content scale comes from runtime context
(Category 3). Overflow behavior (truncate, wrap, clip, scroll) is declared in the mark
spec and does not change based on available space — the mark declares one policy and
the layout system enforces it.

### Rule 8 — Icon-bearing marks use the asset system

Icons are not embedded path data. They are references into the asset registry. The mark
owns an asset reference and a resolved color slot. The asset system resolves the SVG
glyph. The mark's projection produces a draw command referencing the resolved glyph,
not raw path data. Icon color is a resolved theme slot, not a hardcoded value.

### Rule 9 — Anchor exports declare update triggers

A mark that exports anchors must declare the complete set of conditions under which those
anchors change value: bounds change, content metrics change, value change, density change,
projection context change. Consumers of anchors may only assume an anchor is valid until
one of its declared triggers fires. This is the contract that makes tooltip positioning,
dropdown alignment, and handle attachment predictable.

### Rule 10 — Custom facets in mark trees follow the same placement contract as marks

A custom facet placed as a child within a mark's layer or group is not exempt from the
placement contract. It must declare its layer attachment and placement hints (grid, anchor,
or free) using the same `ChildAttachment` fields as marks. A custom facet that cannot
satisfy the placement contract for the layer or group it is placed in does not belong there.
This is a programming contract violation (panic in debug builds).

### Rule 11 — Marks communicate through stores and signals

A mark that needs to react to another mark's state subscribes to the store that mark
writes to, or listens for the signal that mark emits. There are no direct facet-to-facet
calls as a composition mechanism. This applies equally to custom facets alongside marks.

### Rule 12 — Every mark declares its completeness tier

Marks carry a `stability` field with one of four values:

- `draft` — contract is written, implementation not started or incomplete
- `specified` — all numeric/dimensional values filled, contract internally consistent,
  all invalidation triggers declared, no open questions remaining in the spec
- `verified` — golden tests pass for all declared golden states; replay scenarios pass
- `stable` — API is frozen; changes require migration path and version bump

Marks shipped in the standard library may not be `stable` before reaching `verified`.
The `verified` tier is the gate, not `specified`.

---

## 4. What This Means for the Mark Schema

The V2 mark schema (the YAML format) must be updated to make these contracts explicit.
Sections that change:

**`layer_contract` (new top-level section)**
Replaces the ad-hoc layer declarations in `projection_contract.layers`. Every mark
declares:
- `target_layer`: typed standard layer ID or `implicit_base`
- `hit_policy`: normal / pass_through / block_below / disabled
- `dismissal_policy`: required for any mark targeting Floating or above
- `focus_trap`: boolean, required for Modal-layer marks
- `focus_restore`: where focus returns on close

**`context_contract` (new top-level section)**
Every mark declares which context categories it consumes:
- `visual_theme`: boolean (does projection read color/material slots?)
- `layout_style`: list of properties consumed (density, writing_direction, spacing_scale)
- `runtime_properties`: list of properties consumed (content_scale, input_modality)
And for each consumed property: what it invalidates (layout, projection, or both).

**`group_contract` (replaces parts of `layout_contract`)**
Marks that compose children as a group declare:
- `layout_policy`: grid / linear_horizontal / linear_vertical / free_anchor
- `grid_config`: if grid, the column/row/gap/track configuration
- `linear_config`: if linear, the axis, gap, and overflow behavior
- `overflow`: clip / scroll / visible

**`invalidation_triggers` (promoted, made mandatory)**
Currently buried in `projection_contract`. In V2 this is a top-level mandatory section
that lists every condition that causes re-layout, re-projection, or both. Incomplete
invalidation triggers is a spec defect, not an open question.

**`stability`**
The existing field gains enforced meaning via the four-tier definition above. A mark
cannot be declared `verified` without passing golden tests. A mark cannot be declared
`stable` without a migration policy.

---

## 5. What Is Out of Scope for V2 Marks

These are explicitly deferred, not forgotten:

- Automated schema validation (V2 is manual QA; V3 may introduce tooling)
- Date picker, color picker, file input marks
- iOS platform support
- WebAssembly target
- Accessibility role/name platform mapping (infrastructure present, mapping deferred)
- Formal performance SLOs per mark
