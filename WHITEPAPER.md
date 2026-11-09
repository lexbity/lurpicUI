# lurpicUI: A Retained-Mode UI Engine for Realtime, Human-and-Machine-Operable Applications

**Draft v0.1 — [NEED DATA: benchmarks, dates, versions to be filled]**
**Author:** Prestin Liu (lexbit) and contributors
**Date:** [NEED DATA: publication date]

---

## 1. TL;DR

lurpicUI is a retained-mode UI framework in Go for realtime interactive applications (Linux desktop, Android) whose core claim is that **automation and human interaction can share one input pipeline without a test-only back door**, and that this can be *proven* rather than merely asserted, by making every drive verb a GUI twin measured against the real gesture. The engine keeps a single-writer runtime thread, a ten-phase frame loop, and a retained facet tree whose projection produces draw commands, hit regions, and child coordinate contexts together. The strongest evidence is structural rather than numerical: a differential equivalence corpus (`input/equivalence`) that measures verb-vs-gesture parity, a software renderer that acts as the correctness oracle for a Rust/Vulkan GPU backend, and a custom static analyzer (`lurpiclint`, rules LL001–LL039) that enforces framework contracts at compile time. The biggest caveat: several CMake test targets reference deleted packages and do not run, and the GPU backend has no published rendering-equivalence numbers in this repository, so performance and visual-fidelity claims beyond "it builds and runs" are currently [NEED DATA].

---

## 2. The Problem

Realtime UI frameworks today split into two camps, and both break down at the same point.

**Camp 1: native GUI toolkits** (Qt, GTK, Flutter). They are built for human input. Automating them means reaching into widget internals, synthesizing OS events, or maintaining a parallel "test mode" that bypasses the input pipeline. The automation path and the user path diverge, and bugs in one do not reproduce in the other.

**Camp 2: web stacks** (React, etc.). They are drivable via DOM selectors, but the drive path (Playwright/Cypress) runs out-of-band: it pokes state and reads the DOM, not the rendered frame. Timing, animation, focus, and hit-testing are reconstructed approximations.

What's actually broken: there is no framework where **a script can do exactly what a user can do, provably**, because "what a user can do" includes timing (frame boundaries), focus traversal, gesture capture, and hit-test resolution — none of which are exposed by a clean API. The usual answer is a test hook, which is precisely the back door that makes the equivalence claim vacuous.

A second, independent problem: **realtime UIs are hard to reason about**. State lives in widgets, in parents, in globals. Concurrency is bolted on. The frame order is implicit. The fix is usually "read the code."

---

## 3. Background

**Retained mode vs. immediate mode.** In immediate-mode UI (Dear ImGui), the UI is rebuilt every frame from code; there is no persistent tree. In retained mode (Qt, this engine), a tree of nodes persists and is invalidated selectively. lurpicUI is retained mode, but with a twist: the tree nodes ("facets") do not own data.

**Facet.** A node in the retained tree. It observes state, owns ephemeral interaction/presentation state, and projects into a local spatial form. It never owns domain data (Principle 1 of the Facet Runtime Model).

**Store.** An observable state cell with a monotonic version number. The single source of truth (Principle 2). Mutations happen only on the runtime thread.

**Projection (broader than rendering).** A facet's projection pass produces: draw commands, hit regions, child coordinate contexts, selection geometry, and interaction affordances. Rendering is one consumer. This is Principle 3 — the reason the renderer backend never makes layout decisions.

**Job.** Background work following snapshot → work → commit. The worker computes against an immutable snapshot; the runtime thread validates versions and discards stale results.

**Cut.** The boundary between frame N's submit and frame N+1's phase 1. All LDP mutations apply at a cut, not mid-frame.

---

## 4. Novelty Map

| Idea | New / Recombination / Known | Closest prior art | What's actually different |
|---|---|---|---|
| Single-writer runtime thread + snapshot→work→commit jobs | Recombination | Actor model (Erlang/Akka), Elm architecture | Applied to a UI frame loop with version-fenced commits and explicit frame phases; no locks in facet code |
| Projection broader than rendering (draw + hit + coords + selection) | Recombination | Retained-mode scene graphs, display lists | The projection system is the *only* producer of hit regions; renderer is a pure consumer |
| LDP drive seam: actuation with GUI twins proven equivalent by measurement | **New combination** | Playwright/Cypress (out-of-band), XCUITest (in-process) | Every verb has a GUI twin; equivalence is measured via a differential corpus, not shared code; human input preempts scripted gestures mid-flight |
| Virtual time as a first-class clock mode | Recombination | Game engines, deterministic replay | Integrated with the frame-cut model; only the time-grant holder advances frames; input-only clients never advance implicitly |
| Software renderer as correctness oracle for GPU backend | Known pattern | Reference renderers, golden images | Combined with a PSNR-gated differential corpus (`render/equivalence`) |
| Consumer-side interfaces at every seam | Known (Go idiom) | — | Applied consistently: platform, render, log, drive are all defined in the consuming package |
| Static analyzer enforcing framework contracts (LL001–LL039) | Recombination | lint, custom AST tools | Rules encode framework invariants (e.g., LL010: facet/projection may not import render; LL038: no re-declared runtime providers) |
| Marks as declarative widgets via `Binding[T]` reflection | Recombination | React hooks, data binding | "The struct declaration is the entire binding contract"; binding fields reassigned before attach only |
| Layer system with global typed layer IDs | Recombination | CSS z-index, stacking contexts | Render order derived from global layer tier, not parent-scoped integers |
| Asset pipeline with LOD and budgets | Recombination | Asset streaming systems | Demand-loaded, reference-counted, evicted on budget pressure |

**Honest assessment:** most of this is recombination, not invention. The genuine novelty is the **LDP drive seam and its equivalence proof** — the idea that actuation is a first-class framework seam (like platform and render), with typed verbs, at-most-once ledger, cut boundaries, and measured GUI-twin parity. The rest is disciplined application of known patterns.

---

## 5. Architecture

### 5.1 The frame loop (ten phases)

Every frame executes phases in fixed order (Principle 9), enforced by guards:

1. Drain job results
2. Collect platform events
3. Tick hover
4. Tick active facets
5. Deliver input (drive activation boundary: one command per frame)
6. Deliver signals (batched, ≤16 iterations or panic)
7. Layout (dirty roots) + FlushDerived
8. Projection (dirty facets only)
9. Assemble (layered batches, dirty regions, overlay injector)
10. Submit (sync or async + frame credit) → seal cut

The order is not arbitrary: job results before input (committed results change what's hittable), input before signals, signals before layout, layout before projection, projection before assembly. Invalidations accumulate, they don't cascade — layout and projection run once per frame with the full dirty set.

### 5.2 Threading model

One runtime thread owns the facet tree, all stores, all signals, all interaction state, all projection results. No other goroutine touches these. Background workers read immutable snapshots and commit via the job system. The render thread reads immutable frame packets. Drive server goroutines only submit into a bounded inbox. This is Principle 4 — the practical consequence is that the runtime thread must be fast (~4ms budget at 60fps after layout/projection/input/signal delivery).

### 5.3 The facet tree and projection

Facets do not own truth. Domain state lives in stores; facets subscribe and derive. Projection produces the local interactive spatial form. The projection system has gates: empty-bounds prune, mount gate (unmounted layer children skipped entirely), cache keyed on bounds+transform+viewport+subscribed versions, poison skip. Forked projection walks sibling subtrees above a threshold of 8 in parallel, merged pre-order.

### 5.4 Layers

Overlays/split panes are ordered bands: Base < Content < Popover < Modal < Tooltip < Toast. The layer registry is immutable (standard layers 1–9, orders 1000…9000). Render order is derived from the global layer tier, not parent-scoped integers — this fixes the V1 bug where two parents could both declare `RenderOrder: 10` with no global relationship.

### 5.5 Marks and bindings

A mark is a reusable widget built on `marks.Core` with typed `Binding[T]` fields. The struct declaration is the binding contract: fields are reflected at attach. Bindings can be `Const`, `FromStore`, or `FromDerived`. Two-way bindings (`ScalarBinding`) handle attach-sync, re-sync, write-back, and echo guard. Marks declare their `as_parent` (layout policy for children) and `as_child` (participation in parent's group) contracts. Shared plumbing (fonts, icons, interaction state) must be used via marks-root helpers — enforced by lint rule LL038.

### 5.6 Theme

Tokens → Material → MarkStyle.Resolve → family recipes → ResolvedContext → layout recipe lookup. StyleContext is itself a store, derived via `ctx.Derive(override)` with incremented Depth. Theme switches invalidate projection automatically because marks resolve slots at projection time, not attach time.

### 5.7 LDP (the drive seam)

LDP is a versioned wire protocol (major 2, hard break, no shims) over a Unix socket. Key invariants:

- **Grants** baked in at launch, immutable per session: read, input, command, time, admin.
- **At-most-once ledger**: bounded (1024 entries / 4 MiB), contiguous ID admission, retained → replay, changed payload → conflict, evicted → expired.
- **Cut boundaries**: all mutations apply at a cut; replies become observable after seal.
- **No direct store mutation**: actuation is GUI-backed `input.*` and `cmd.execute` only.
- **Virtual time**: the time-grant holder acquires a lease; only they advance frames; mode switch refused during a gesture.
- **Human preemption**: any platform event severs an in-flight scripted gesture; ownership flips to human.

The trust boundary: private (0700) parent dir, 0600 socket/token, exclusive no-follow token creation, constant-time comparison. Android: compiles only in debuggable builds.

---

## 6. Features and Abilities

### Feature: Retained facet tree with projection-as-spatial-form

- **Mechanism:** Facets are stateful projection boundaries; projection produces draw + hit + coords + selection together, so the hit system never re-derives geometry.
- **Capability:** A single projection pass drives rendering, hit-testing, and input routing with consistent geometry.
- **Evidence:** Structural (Principle 3 enforced by import guards — `facet` and `projection` may not import `render`, LL010).
- **Limits:** Projection runs on the runtime thread; heavy projection must move to jobs.

### Feature: Single-writer concurrency with version-fenced commits

- **Mechanism:** Runtime thread sole writer; workers read snapshots with store versions; commit validates versions, discards stale.
- **Capability:** Deterministic, lock-free facet code; stale job results cannot corrupt state.
- **Evidence:** `job.Snapshot.StillValid`, `DrainOrdered` in virtual mode.
- **Limits:** Runtime thread budget ~4ms; anything slower must be a job.

### Feature: LDP actuation with measured GUI-twin equivalence

- **Mechanism:** Every drive verb has a GUI twin; equivalence proven by a differential corpus (`input/equivalence`), never by shared code.
- **Capability:** A script can do exactly what a user can do, and the claim is measurable, not asserted.
- **Evidence:** `test-drive` CMake target; `TestEveryEventLiteralNamesItsSource` AST gate for provenance.
- **Limits:** Automation is slower than a native API; every verb is doubled.

### Feature: Software renderer as correctness oracle

- **Mechanism:** CPU software renderer produces reference frames; Vulkan backend compared via PSNR-gated differential corpus (`render/equivalence`).
- **Capability:** GPU backend correctness is measurable, not assumed.
- **Evidence:** `render/equivalence/corpus_test.go`.
- **Limits:** [NEED DATA: PSNR thresholds, corpus size, pass rates].

### Feature: Static contract enforcement

- **Mechanism:** `lurpiclint` rules LL001–LL039 encode framework invariants; runs in CI gate.
- **Capability:** Framework-contract violations caught before merge.
- **Evidence:** `cmake --build build --target lint`.
- **Limits:** LL005–LL009 unused; baseline file stores absolute host path (portability issue).

### Feature: Asset pipeline with LOD and budgets

- **Mechanism:** Cook → `.pak` → mmap zero-copy; demand-loaded, reference-counted, evicted on budget pressure.
- **Capability:** Bounded memory for large asset sets; hot-reload in dev via DevFS.
- **Evidence:** `assets/manager.go`, `assets/job.go`.
- **Limits:** [NEED DATA: budget defaults wiring from TOML to runtime is unclear — see devdocs §7].

### Feature: Data-visualization scale library

- **Mechanism:** Pure-core scale primitives (linear/log/pow/time/band/zoom) with reactive store integration; banned imports enforced by `scale/architecture_test.go`.
- **Capability:** Viz scales isolated from rendering; testable without a window.
- **Evidence:** `scale/` package.
- **Limits:** [NEED DATA: benchmark coverage].

---

## 7. Evidence Summary

| Claim | Evidence type | Result | Confidence |
|---|---|---|---|
| Human and machine share one input pipeline | Structural + differential corpus | `test-drive` target exists; equivalence corpus present | Medium (no published parity numbers) |
| GPU backend matches CPU reference | Differential corpus | `render/equivalence` runs in `test-unit` | Medium (no published PSNR numbers) |
| Framework contracts enforced | Static analysis | `lurpiclint check` gate | High |
| Single-writer determinism | Principle + code + tests | `store` mutations assert runtime thread under debug tag | Medium |
| Virtual time drives deterministic frames | Implementation + docs | `time.advance`/`frame.run` verbs | Medium (no test counts published) |

**Methodology note:** this repository's test infrastructure is CMake-orchestrated Go tests. Several CMake targets (`test-pure`, `test-headless`, `test-integration`, `bench`, `bench-graph`, and consequently `test-all`, `test-race`) reference deleted packages (`./graph/index/...`, `./marks/integration/...`) and do not run. Only `test-unit` and `test-drive` are reliable. Any performance or coverage claim must cite those two.

---

## 8. Threats to Validity

- **Benchmark staleness / missing numbers:** No published benchmark numbers exist in this repo for rendering throughput, projection cost, or frame budget adherence. Claims of performance are currently unsupported.
- **Equivalence corpus scope:** The differential corpus proves verb-vs-gesture parity for *implemented* verbs. Unadvertised methods (11 of them) are not on the wire and not measured end-to-end.
- **Contamination of the "no back door" claim:** The in-process testkit seam exposes unadvertised methods (`store.read`, `tree.dump`, etc.) that a trusted in-process client can use. These bypass the wire but not the input pipeline. Whether this counts as a "back door" depends on the trust model; it is documented, not hidden.
- **Config sensitivity:** `[assets]` TOML values may not reach `runtime.Config` (unclear wiring). Defaults are hardcoded.
- **Small samples:** No published run counts, seeds, or variance for any measurement.
- **Cherry-picking risk:** The developer guide is generated from the codebase; numbers in it are descriptive, not experimental.

---

## 9. Limitations and Failure Modes

- **Platform support:** Linux desktop (X11) and Android only. No macOS, Windows, iOS, WebAssembly. Linux `ContentScale()` is hardcoded to 1 (no HiDPI).
- **No desktop audio backend.**
- **Broken test targets:** `test-pure`, `test-headless`, `test-integration`, `bench`, `bench-graph` (and `test-all`, `test-race`) do not run.
- **Documentation rot:** `Documentation/README.md` is missing but linked; `.github/` and `devdocs/` are gitignored (no CI on fresh clone); `main.go` usage text lists nonexistent verbs.
- **LDP working-tree caveat:** `input.*` methods were flipped to advertised in uncommitted changes; S4 acceptance tests do not exist yet; `Documentation/drive.md` may lag the registry.
- **Asset wiring unclear:** TOML → runtime config mapping unverified.
- **No database / persistence beyond cooked assets and crash reports.**
- **Accessibility role/name platform mapping deferred.**

---

## 10. Related Work

- **Flutter:** retained-mode, cross-platform, but automation is via integration tests that drive the widget tree out-of-band; no measured GUI-twin equivalence. Flutter has broader platform support.
- **Qt/QML:** mature, but automation (Qt Test) bypasses the input pipeline in many cases; no virtual-time seam.
- **Dear ImGui:** immediate mode; no retained tree; automation is trivially scriptable but state model differs.
- **React + Playwright:** DOM-driven automation; timing and rendering are approximated.
- **Godot/Unity:** game engines with editor automation; not general UI frameworks.

lurpicUI's positioning: narrower platform support than Flutter/Qt, but a stronger claim on **deterministic, provably-equivalent automation** and on **inspectability** (single writer, named invalidation sources, no magic). Whether that tradeoff is worth it is [NEED DATA: user-study or adoption data].

---

## 11. Reproducibility

- **Build:** CMake presets; `cmake --build build --target lint` (lint), `test-unit`, `test-drive`.
- **Module:** `codeburg.org/lexbit/lurpicui`, Go 1.27.2+.
- **Key docs:** `docs/Principles/LurpicUI-FacetRuntime-Principles.md`, `docs/Principles/LURPICUX_V2_FOUNDATION.md`, `Documentation/drive.md`, `devdocs/DEVELOPER_GUIDE.md`.
- **Commits:** latest `fceacf9` (fix live hangs, drive protocol). [NEED DATA: tag/version for the paper's baseline].
- **Toolchain:** Go 1.27.2, CMake, Rust (for Vulkan), Android SDK/NDK for Android builds.

---

## 12. References

- Facet Runtime Model Principles — `docs/Principles/LurpicUI-FacetRuntime-Principles.md` (in-repo).
- LurpicUX V2 Foundation — `docs/Principles/LURPICUX_V2_FOUNDATION.md` (in-repo).
- LDP specification — `Documentation/drive.md` (in-repo, parity-tested against registry).
- Developer Guide — `devdocs/DEVELOPER_GUIDE.md` (in-repo, gitignored, generated).
- [NEED DATA: external references — Go language spec, HarfBuzz, XCB, Vulkan spec, etc., to be added with access dates.]

---

## Open items

1. [NEED DATA] Published benchmark numbers for rendering throughput, projection cost, frame budget adherence.
2. [NEED DATA] PSNR thresholds, corpus size, and pass rates for `render/equivalence`.
3. [NEED DATA] Parity numbers for the drive equivalence corpus (`input/equivalence`).
4. [NEED DATA] Test counts per package and coverage percentages.
5. [NEED DATA] Whether `[assets]` TOML values reach `runtime.Config` (wiring verification).
6. [NEED DATA] Version/tag for the paper's baseline commit.
7. [NEED DATA] External references with access dates.
8. [UNVERIFIED] Claim that no other framework offers measured GUI-twin equivalence — needs prior-art search.
9. [UNVERIFIED] Performance comparisons against Flutter/Qt — no data.
10. Downgraded novelty claims: most architecture is recombination, not invention; only LDP drive seam qualifies as a genuinely new combination.