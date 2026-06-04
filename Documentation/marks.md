# Marks — Unified Authored Contract

This document describes the post-rewrite mark system (PRM). It supersedes
`artist-authoring-model.md` and `marks-animation-theme-api.md`, which describe
the pre-rewrite model.

## Overview

A **mark** is a `facet.FacetImpl` that satisfies `marks.Mark`. Every concrete
mark type provides:

- `marks.Core` embedding (role wiring, binding subscription, default anchors)
- Config fields using `marks.Binding[T]` (not raw fields with setters)
- A single `BuildCommands` render path (not dual `OnCollect`/`OnProject`)
- A `Descriptor()` returning `Family` + `TypeName`

## Core Pattern

```go
type MyMark struct {
    marks.Core
    Label      marks.Binding[string]
    Disabled   marks.Binding[bool]
    // ...
}

func NewMyMark(label marks.Binding[string]) *MyMark {
    m := &MyMark{
        Label:    label,
        Disabled: marks.Const(false),
    }
    m.Core.Facet = facet.NewFacet()
    m.AddBinding(m.Label)
    m.AddBinding(m.Disabled)

    m.Layout.OnMeasure = func(...)
    m.Layout.OnArrange = func(...)
    m.Hit.OnHitTest = func(...)
    m.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
        return m.buildCommands(m.Layout.ArrangedBounds)
    }
    m.RegisterRoles()
    return m
}

func (m *MyMark) Base() *facet.Facet       { m.Facet.BindImpl(m); return &m.Facet }
func (m *MyMark) Descriptor() marks.Descriptor { return marks.Descriptor{Family: "my", TypeName: "mark"} }
func (m *MyMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *MyMark) OnDetach()                         { m.Core.OnDetach() }
func (m *MyMark) OnActivate()                       { m.Core.OnActivate() }
func (m *MyMark) OnDeactivate()                     { m.Core.OnDeactivate() }
func (m *MyMark) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
    return m.DefaultAnchors(m.Layout.ArrangedBounds, ctx)
}
```

## Binding[T]

`marks.Binding[T]` replaces imperative `SetX` methods. A binding is a reference
to truth — either an immutable literal (`marks.Const(v)`) or a reference to a
`store.ValueStore`/`Derived` (`marks.FromStore(s, dirtyFlags)`).

The concrete mark adds each binding to the Core subscription list via
`AddBinding`. Core subscribes store-backed bindings in `OnAttach` and
invalidates the facet on every store change.

## Descriptor & Describe

`marks.Descriptor` carries static metadata. Authors declare only `Family` and
`TypeName`; capability flags are derived by `marks.Describe(m)` via static
type assertion:

```go
d := marks.Describe(m)
// d.Focusable     — if m implements marks.Focusable
// d.ExportsAnchors — if m implements layout.AnchorExporter
// d.Accessible    — if m implements marks.Accessible
// d.HostsChildren — if m implements marks.Composite
// d.HitTestable   — if m.Base().HitRole() != nil
// d.DataBound     — if m implements marks.DataBound
```

## Families

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

## Migration Status

All marks have been migrated to the unified contract. No `SetX` config setters
remain. All marks use `marks.Core` + `Binding[T]` + `BuildCommands`.
