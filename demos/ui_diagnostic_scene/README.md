# UI Diagnostic Scene App

Phase 1 implementation of the `ui_diagnostic_scene` product - a manual QA and debugging tool for the lurpicUI engine.

## Overview

This application serves as the primary observability surface for the engine, answering:
- What broke
- Where it broke
- Why it broke
- Whether the failure is layout, projection, input, focus, theme, animation, rendering, or runtime related

## Architecture

### Shell Structure (Phase 1)
- **TopBar**: Displays theme, backend, platform, and build metadata
- **SceneNav**: Left navigation panel for scene selection
- **SceneHost**: Center viewport where scenes render
- **DiagnosticsPanel**: Right panel for overlay controls (Phase 2+)
- **LogsPanel**: Bottom panel for event and state logging

### Scene Registry
Scenes are organized by mark family and failure mode:
- `catalog-lite`: Reduced catalog rendering
- `interaction`: Hover, press, drag, click, selection, focus
- `layout-torture`: Constraint extremes, nesting, clipping
- `theme`: Token propagation, density changes
- `animation`: Tick delivery, timeline progression
- `input-focus`: Keyboard routing, tab order, caret visibility
- `store-signal`: Invalidation, signal fanout
- `projection`: Child transforms, hit regions
- `stress`: Resize/theme/mount/unmount churn
- `text-ime`: Text entry, composing state
- `annotation`: Labels, connectors, callouts
- `chart`: Axes and scale behavior

## Building

```bash
cd demos/ui_diagnostic_scene
go build -o ui_diagnostic_scene
```

## Running

```bash
./ui_diagnostic_scene -width 1400 -height 900
```

## Phase 1 Deliverables (Complete)

- [x] Executable bootstrap
- [x] Scene host with facet-based architecture
- [x] Scene list/navigation with stable IDs
- [x] Empty diagnostics panel (placeholder for Phase 2)
- [x] Empty logs panel (functional but basic)
- [x] Reset wiring

## Phase 1 Tests (Complete)

- [x] app launches with empty registry
- [x] selecting a scene updates state
- [x] empty diagnostics/log panes do not crash
- [x] reset on uninitialized scenes fails safely

## Phase 2 Deliverables (Complete)

- [x] Overlay kinds (bounds, dirty, hit regions, anchors, focus, layers, timing)
- [x] Event model (pointer, keyboard, text, store, signal, scene, render, lifecycle)
- [x] Frame stats model with history and FPS tracking
- [x] Hit summary model
- [x] Focus summary model
- [x] Invalidation summary model
- [x] Render batch summary model
- [x] Anchor snapshot summary model
- [x] Scene capability summary model
- [x] Diagnostics adapter bridging engine to app-facing abstraction

## Phase 2 Tests (Complete)

- [x] diagnostics adapters populate views for each overlay kind
- [x] missing data degrades gracefully rather than crashing the scene host
- [x] overlay toggles do not mutate domain truth or scene state
- [x] frame stats populate on every frame even when overlays are disabled

## Phase 3 Deliverables (Complete)

- [x] `scenes/base.go` - BaseScene with common scene functionality
- [x] `scenes/catalog_lite.go` - CatalogLiteScene (basic rendering)
- [x] `scenes/interaction.go` - InteractionScene (hover, press, click, focus)
- [x] `scenes/layout.go` - LayoutScene (nesting, constraints)
- [x] `scenes/stress.go` - StressScene (performance testing)
- [x] Scene registration in main.go
- [x] 4 implemented scenes + 8 placeholder scenes for future phases

## Phase 3 Tests

- [x] scenes compile without errors
- [x] scenes implement the Scene interface correctly
- [x] scenes register with the registry
- [x] BuildRoot returns valid facet trees
- [x] ExportState/ImportState round-trip works

## Phase 4 Deliverables (Complete)

- [x] Enhanced InteractionScene with hover, press, click handling
- [x] Drag gesture tracking with visual feedback
- [x] Event logging system for interaction history
- [x] InputFocusScene with focusable elements

## Phase 5 Deliverables (Complete)

- [x] ProjectionScene with nested transforms
- [x] Rotation, scale, translation, and combined transforms
- [x] Hit test tracking for transformed elements
- [x] Diagnostic visualization layer
- [x] Transform inspection and export

## Phase 6 Deliverables (Complete)

- [x] Enhanced StressScene with churn tracking
- [x] Frame statistics tracking (min/max/avg frame times)
- [x] Resize churn simulation
- [x] Theme churn simulation
- [x] Mount/unmount churn tracking
- [x] AnimationScene with tick delivery validation
- [x] Frame jank detection (slow frame logging)
- [x] Render batch count tracking

## Remaining Phases

- **Phase 7**: Artifact export and bug report output
