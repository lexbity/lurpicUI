# Principles of the Facet Runtime Model — Core Principles

This is worth doing carefully. Principles aren't a summary of what we built — they're the why behind every decision. If you hand someone these principles and the package list, they should be able to reconstruct the architecture independently. If a principle doesn't constrain decisions, it's not a principle — it's a slogan.

## Principle 1 — Facets own translation, not truth

**A facet is a stateful projection boundary.** It observes state, owns local interaction/presentation state, and derives a local interactive spatial form from both. It does not own domain data.
This is the hardest principle to hold onto under pressure. When a graph node facet needs the node's position, the instinct is to store position in the facet. The principle says: position lives in a store. The facet subscribes and derives.
What this rules out: facets that double as data containers, facets that pass data to child facets directly instead of via stores, facets that cache domain state without version tracking.
The test: if you removed the facet from the tree and replaced it with a different facet subscribing to the same stores, would the application still have all its data? If yes, the facet is correctly stateless with respect to domain truth. If no, data has leaked into the facet.

## Principle 2 — Stores are the single source of truth

All shared observable state lives in stores. Stores are mutated only from the runtime thread. Stores carry version numbers that increment monotonically on every mutation. Stores notify subscribers via typed signals.
What "shared" means: state that more than one facet observes, or state that persists beyond a single facet's lifetime, or state that background jobs read from. Ephemeral interaction state (is this button pressed right now?) is not shared — it lives in the facet.
What this rules out: facets communicating by calling each other's methods, parent facets passing mutable state to children, background goroutines writing to any store directly, state living in global variables.
The version number is not optional. Every store mutation increments a version. Every background job snapshots the version it computed from. Every job result is validated against current versions before commit. Without versions the snapshot→work→commit pattern cannot detect stale results.

## Principle 3 — Projection is broader than rendering

A facet's projection pass produces: visual output (draw commands), hit regions, child coordinate contexts, selection geometry, and interaction affordances. Rendering is one consumer of projection output. The hit system is another. The input system is another.

This means the projection system produces a local interactive spatial form — not pixels, not a scene graph node, not a widget. The form includes everything needed to interact with and render the facet's content.

What this rules out: facets that call render functions directly, facets that know about the renderer backend, projection code that reads from the renderer's state, the renderer making decisions that belong to projection (what is visible, what is hittable, what coordinate space applies).
The compiler enforces this: facet does not import render. projection does not import render. If either import appears, the principle has been violated.

## Principle 4 — The runtime thread owns all mutable engine state

One thread — the runtime thread — is the exclusive owner of the facet tree, all stores, all signals, all interaction state, and all projection results. No other goroutine touches these directly. Ever.

Background workers read from immutable snapshots. The render thread reads from immutable frame packets. Communication from workers to the runtime thread flows through the job result channel, drained once per frame. Communication from the render thread to the runtime thread flows through the fatal error channel only.
What this rules out: stores with concurrent read-write access, signals fired from goroutines, facets updated from callbacks that arrive on non-runtime threads (platform IME callbacks, file loading completion handlers — all must be dispatched through the event queue).
The practical consequence: the runtime thread must be fast. Anything slow moves to a worker via the job system. The frame budget on the runtime thread is roughly 4ms at 60fps after layout, projection, input routing, and signal delivery. Anything that takes longer than a millisecond belongs in a background job.

## Principle 5 — Concurrency accelerates projection, not the programming model

The engine feels single-threaded to facet authors. There are no locks to acquire, no goroutines to spawn manually, no channels to select on. The concurrency is invisible behind the job system's snapshot→work→commit pattern.
The pattern has exactly three steps and they are always in this order:

Snapshot — runtime thread captures immutable inputs with their store versions
Work — worker goroutine computes against the snapshot, never touching live state
Commit — runtime thread validates versions, applies result if still current, discards if stale

There is no step 2.5 where the worker communicates back mid-computation. There is no alternative pattern. Facets that need concurrent work use this pattern or they don't use concurrency.
What this rules out: go func() inside facet code, channels as a primary communication primitive in facet code, any concurrency primitive other than job.Schedule in application-facing code.

## Principle 6 — Roles define participation, not identity

A facet's identity is its FacetID. Its roles define which engine systems it participates in. A facet with a RenderRole contributes visual output. A facet with a HitRole participates in hit testing. A facet with a TickRole receives per-frame updates. A facet with none of these roles is a valid structural container.
Roles create obligations. Attaching a RenderRole means the facet promises to produce draw commands when asked. The runtime holds facets to their promises.
What this rules out: base class hierarchies where rendering behavior is inherited, checking facet type at runtime to decide how to handle it, god-facets that implement every possible behavior, roles that modify each other's behavior through shared state.
The relationship between roles and the engine pipeline: the runtime's frame pipeline calls into roles, not into facets directly. LayoutRole.OnMeasure, RenderRole.OnCollect, HitRole.OnHitTest — the pipeline is a sequence of role invocations across the tree.

## Principle 7 — Children project within parent-defined context

Parent-child relationships in the facet tree are about projection context, not just visual containment. A parent facet with a layer contract defines the coordinate space that its children inhabit. A ViewportRole defines a local transform inside that contract. A layout container defines size constraints and arranged bounds. Its children project within those bounds.
Children are subordinate projection boundaries inside a parent-defined local world. This is why a graph canvas facet can contain overlay controls — those controls project within the canvas's world-space coordinate context.
What this rules out: children that query their parent's state directly (they subscribe to the same stores instead), parents that reach into children's projection output (the runtime assembles the frame from all outputs), siblings that communicate by sharing state through their common parent.
The propagation rule follows from this principle: layout dirty propagates upward (parent needs to re-layout if child size changes) and downward (children need new bounds). Projection dirty propagates downward only (children re-project within updated parent context, but parent doesn't re-project just because a child did).

## Principle 8 — State has four kinds and they must not be confused

Every piece of state in the engine belongs to exactly one of

four categories:
Domain state — the application's source of truth. 

Graph structure, metrics, annotations, relationships. Lives in stores. Never in facets.

View state — how domain data is currently being presented.

Zoom level, pan offset, collapsed groups, sort mode, active filters. Lives in stores. Owned by the application, observed by facets.

Interaction state — ephemeral UI state. 

Currently hovered entity, active drag, pointer capture, focused input, gesture in progress. Lives in facet internals or runtime systems. Never in stores (it changes too frequently and doesn't need persistence or sharing).

Render state — derived artifacts for rendering efficiency.

Spatial index, glyph run cache, tessellated edge geometry, cluster representations, dirty rect history. Lives in facet projection internals. Never confused with domain state — it's rebuilt when domain state changes.

The test for each: Domain state — would losing it require the user to re-enter or re-fetch data? View state — would losing it require the user to manually re-configure their view? Interaction state — would losing it be unnoticeable after the current interaction completes? Render state — can it be fully reconstructed from domain and view state?

## Principle 9 — The frame pipeline has a fixed phase order

Every frame executes phases in this order, without exception:
1.  Drain job results
2.  Collect platform events  
3.  Tick hover
4.  Tick active facets
5.  Deliver input events
6.  Deliver signals (batched)
7.  Run layout (dirty subtrees only)
8.  Run projection (dirty facets only)
9.  Assemble render frame
10. Submit to render thread

This order is not arbitrary — each phase depends on the previous. Job results before input because committed results change what's hittable. Input before signals because input handlers emit signals. Signals before layout because signal handlers may invalidate layout. Layout before projection because projection reads arranged bounds. Projection before frame assembly because assembly reads projection output.
Invalidations accumulate, they don't cascade. When a store changes during signal delivery, the affected facets are marked dirty. Layout and projection do not run immediately — they run once in phases 7 and 8 with the full set of dirty facets. This is the batching mechanism that prevents redundant recomputation when multiple stores change in one frame.

## Principle 10 — Interfaces at every seam that will vary

Every boundary between subsystems that has more than one implementation — now or planned — is defined as a Go interface. The concrete implementations are never directly imported except at the wiring point (app).
Current boundaries with multiple implementations:

platform.App / platform.Window / platform.Surface — linux vs android vs testkit
render.Backend — software vs vulkan vs testkit null
runtime.Logger — stderr vs slog vs testkit vs nop

What this gives you: the testkit works because runtime.New takes interfaces. The Vulkan backend slots in because render.Backend is an interface. Android support requires only a new platform implementation, not touching runtime, projection, or facet code. The interfaces are the seams along which the engine can evolve without breaking its consumers.
The corollary: interfaces are defined in the package that uses them, not the package that implements them. render.Backend is defined in render, not in render/software. platform.App is defined in platform, not in platform/linux. This is Go's standard interface ownership pattern and it's correct.

## Principle 11 — Errors are classified, not handled uniformly

Four error kinds, four responses, no exceptions:

Programming contract violations → panic with a message naming the contract and what to do instead
Initialization failures → wrapped error returned to app.Run and then main
Recoverable runtime conditions → silent discard or debug log, execution continues
System failures → recovery attempt via defined path, then clean shutdown via fatalCh

The classification matters more than the handling. A stale job result that panics destroys the application for a normal operating condition. A nil required argument that returns an error gives the caller the option to ignore a bug. Getting the classification right prevents both failure modes.

## Principle 12 — The engine is observable but not magic

The engine's behavior is inspectable at runtime via the diagnostics system. Facet dirty state, hit regions, projection cache hits, frame timing, invalidation sources — all visible when diagnostics are enabled.

But the engine achieves this observability through explicit instrumentation, not through reflection, code generation, or hidden dependency tracking. Every subscription is explicit. Every invalidation has a named source. Every signal is typed. Every job has a stable ID. The programmer can read the code and understand what will happen — there is no framework magic that makes behavior emerge from annotations or naming conventions.

What this rules out: string-based signal routing, reflection-based role discovery, implicit reactive dependency tracking (where the framework silently records what you read during a function and auto-subscribes), code generation for facet boilerplate.

The tradeoff this acknowledges: explicit wiring is more verbose than magic. A facet's OnAttach method manually subscribes to each store it cares about. This is intentional. The verbosity is the documentation — reading OnAttach tells you exactly what this facet observes. Magic hides this and makes the system harder to reason about, especially when debugging unexpected re-projections or missed invalidations.

Principle 13:

Assets are demand-loaded, reference-counted, and evicted on budget pressure — never eagerly released and never retained beyond their consumer's lifetime. An asset handle is a weak reference to cooked data. The asset system, not the facet, owns the decoded resource. Facets hold handles; the runtime holds memory.


## The principles as a decision filter

These thirteen principles are most useful when a design decision is ambiguous. 

Apply them as a filter:

Should this state live in a store or a facet? → Principle 1 and 8.
Should this work happen on the runtime thread or in a job? → Principle 4 and 5.
Should this produce a new package or live in an existing one? → Principle 10 (is there a seam here that will vary?) and Principle 3 (is this projection or rendering?).
Why is this facet reprojecing when I didn't expect it to? → Principle 9 (what phase triggered the invalidation?) and Principle 12 (check the diagnostics, the source is named).
Is this a programming error or a recoverable condition? → Principle 11 (did the caller violate a contract, or did the runtime encounter an expected failure mode?).
Should child facets know about their parent's state? → Principle 7 (children subscribe to stores, not to parents).
