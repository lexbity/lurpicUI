# Facet Runtime API

This document is the user-facing overview of the facet runtime engine.

## What a facet is

A facet is the runtime unit of composition. A facet can expose roles that
describe behavior in a specific subsystem.

Common roles include:

- layout
- render
- projection
- focus
- text
- tick
- viewport

Roles allow a single authored object to participate in multiple phases without
forcing a deep inheritance chain.

## Lifecycle

The runtime manages lifecycle in a strict order:

1. attach
2. activate
3. frame processing
4. deactivate
5. dispose

Attachment establishes ownership and runtime access. Activation marks the facet
as live. Disposal releases subscriptions, child links, and runtime-owned
resources.

## Tree operations

The API exposes the following tree operations:

- add a child facet to a parent
- remove a child facet
- invalidate a facet with a dirty flag
- inspect current tree state
- query projection layers and anchor snapshots

The tree is expected to be stable and rooted. Callers should prefer structured
attachment over manipulating base fields directly.

## Layout and projection

The runtime separates layout and projection:

- layout measures and arranges the tree
- projection converts layout state into render batches and hit maps
- render submission happens only after projection has produced a frame

This separation lets the runtime short-circuit work when only a subset of the
tree changes.

## Jobs and background work

Facets may schedule jobs through the runtime:

- a job must be associated with an owner facet
- results are delivered back to the owner facet when still current
- stale or cancelled results are discarded

This is the preferred way to do expensive or asynchronous work without blocking
the frame loop.

## Input and hit testing

Hit testing uses the current projection hit map and layer policies:

- `HitNormal` accepts the hit
- `HitPassThrough` continues traversal below the current layer
- `HitBlockBelow` stops lower layers from being considered
- `HitDisabled` skips the layer entirely

Input events are routed through the input system and then delivered into the
facet tree.

## Diagnostics

The runtime can expose:

- frame stats
- layer snapshots
- anchor snapshots
- hit traces
- synchronous inspector snapshots

Diagnostics are designed for tooling and testing, not for mutating runtime
state.

## Shutdown behavior

Runtime shutdown is responsible for:

- cancelling jobs
- disposing the facet tree
- clearing phase hooks
- clearing shutdown hooks
- destroying the render pipeline and backend

Runtime-owned timelines and other hook-driven components should unregister or
dispose themselves during shutdown.
