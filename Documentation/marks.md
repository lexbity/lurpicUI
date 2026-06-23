# Marks

> Maturity banner: PRM / BETA. This page describes current package shape only.
> It is not a stable authoring tutorial, and it should not be treated as a
> copy-paste recipe until Lurpic Studio validates the rewrite end-to-end.

`marks.md` is the current shape reference for the post-rewrite mark system.
Use it to understand the package surface, not to assume the API is frozen.

## Verified Shape

The package centers on:

- `marks.Core` for role wiring and binding subscriptions.
- `marks.Binding[T]` for config values that are either literals or store-backed.
- `marks.Describe` for derived capability flags.
- `marks.Descriptor` for the `Family` + `TypeName` identity pair.

## Core Pattern

At a shape level, a mark family typically:

- embeds `marks.Core`
- declares family-specific `marks.Binding[T]` fields or equivalent config inputs
- implements `Descriptor()` with `Family` and `TypeName`
- wires roles and bindings through package-specific setup

Those steps describe the current contract shape, not a stable tutorial.

The current authored families in the tree are:

| Package | Marks |
|---|---|
| `marks/primitive` | text, icon |
| `marks/action` | button, icon_button, split_button, menu_button, toolbar, ribbon, action_bar, action_group, radial_menu, command_palette, popup_palette |
| `marks/input` | text_field, number_field, color_picker |
| `marks/selection` | checkbox, radio_group, slider, switch, dropdown_select, button_group, list_item, turn_dial |
| `marks/navigation` | breadcrumbs, nav_drawer, nav_rail, pagination, tabs, tree_navigator |
| `marks/feedback` | alert, dialog, notification, tooltip |
| `marks/status` | badge, progress_bar, progress_ring, status_light |
| `marks/structure` | card, list, scroll_region, table |
| `marks/viz` | rule, axis, point, line, area, bar |
| `marks/data` | CollectionBinder, DataMark, RegionFromBounds, Pt |

## Construction Shape

Constructor shape is family-specific. In the current tree, some constructors
take `marks.Binding[T]` inputs, while others still use plain values or package-
specific config helpers. That asymmetry is inferred from the package APIs and
should be rechecked before copying a pattern into a new mark family.

## What This Page Does Not Promise

- No stable tutorials.
- No guarantee that a family has reached the same maturity as the others.
- No promise that authoring ergonomics are settled.
- No promise that `Binding[T]` usage is uniform across every constructor.

## Related Docs

- [marks-animation-theme-api.md](marks-animation-theme-api.md) is stale reference
  material for the pre-rewrite model.
- [Principles/README.md](Principles/README.md) explains the engine principles
  that the package is meant to fit.
