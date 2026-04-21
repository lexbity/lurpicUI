# Framework Runtime Principles

This document describes the runtime model that underpins lurpicUI.

## Runtime as an orchestrator

The runtime is not the owner of visual state. It is the orchestrator that:

- advances time
- applies input
- resolves layout
- reconciles anchors
- runs projection
- submits render batches
- captures diagnostics

The runtime should remain deterministic for a given input stream, frame timing,
and tree state. Every subsystem should be able to run from the same tree
snapshot without hidden cross-package side effects.

## Facet tree model

The engine is organized around a facet tree:

- each facet has a base object that stores lifecycle state and roles
- roles attach behavior without forcing a rigid inheritance hierarchy
- the tree is authoritative for attachment, activation, dirty propagation, and disposal

The tree model is intentionally explicit. A facet may expose:

- layout behavior
- projection behavior
- render behavior
- focus behavior
- text behavior
- tick behavior
- viewport or projection metadata

## Dirty-state propagation

Dirty flags are used to constrain work:

- layout dirtiness should remeasure only the affected subtree when possible
- projection dirtiness should rerun the projection stage without forcing unrelated layout
- anchor changes should invalidate dependent descendants, not the whole tree
- render-only changes should not rebuild layout or input routing

The runtime tracks dirty sources to aid debugging and diagnostics.

## Layer and anchor reconciliation

Layers define composition boundaries and placement policy.

Anchor export is separate from layout measurement:

- an anchor exporter provides named positions in resolved space
- anchor consumers reference those names through attachments
- the runtime resolves exporter output, caches the resulting positions, and invalidates dependent children when anchors move

This separation keeps authored marks composable while avoiding a hard dependency on pixel coordinates.

## Input and gesture routing

Input is processed before layout and projection:

- the runtime collects platform events
- the input system converts events into routing and gesture operations
- hit testing uses the current projection hit map
- focus and pointer state are reset on platform focus loss

Gesture routing is layered on top of the facet tree so recognizers can be attached to authored nodes instead of a separate input scene graph.

## Frame timing

Frame timing is explicit and visible:

- the frame timer controls pacing
- runtime phase-1 hooks run at the start of a frame
- timelines and other time-based systems register against phase-1
- the runtime shuts down those hooks with the runtime lifecycle

Time-based animation should not outlive the runtime that drives it unless the caller intentionally uses a standalone registry path.

## Diagnostics

Diagnostics should be passive:

- they may observe the tree, layers, anchors, and hit traces
- they should not mutate runtime state
- inspectors should be cheap to call and safe to use from external tooling

## Implementation rule of thumb

If a feature can be expressed as a facet role, a layout policy, a projection layer,
or a mark composition, keep it there. The runtime should coordinate, not
accumulate feature-specific business logic.
