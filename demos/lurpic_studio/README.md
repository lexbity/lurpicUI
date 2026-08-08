# Lurpic Studio

Realtime interactable documentation for the lurpicUI framework: a gallery of
live exhibits, each demonstrating one framework capability interactively.
See `devdocs/plans/lurpic-studio-redesign.md` for the full specification.

## Status

Slice P10 of the multi-slice plan is in place (P0–P9 done previously):

- **Responsive shell (`studio/root.go` + `studio/root_narrow.go` + `studio/responsive_test.go`)** —
  FR-resp. The shell collapses below the 960dp breakpoint (content-scale aware):
  the wide 3-pane split becomes a full-width stage with the exhibit index
  re-hosted as a nav_drawer + bottom action bar and the inspector as a bottom
  sheet. The two arrangements are **different mark instances bound to the same
  `ShellState` stores**, so a breakpoint crossing preserves store-version
  continuity and value equality (`responsive_test.go`), and a wiring-equivalence
  test asserts both trees reference the same stores (R-resp).
- **Exhibit index (`studio/pane_index.go` + `studio/catalog.go`)** — the wide
  index pane hosts `nav_rail` + `tree_navigator`, both driving the shared
  `ActiveExhibit` store (FR-nav). The catalog is the single source of the
  exhibit list shared by the index, the narrow drawer/rail, the command palette,
  and the status bar.
- **Command palette (`studio/command_palette.go`)** — FR-cmd. A shell command
  registry (switch exhibit, toggle the feed, toggle the narrow sheets) behind
  the `command_palette` mark; Ctrl+K (root focus) and the chrome ⌘K button open
  it, and running a command mutates observable state.
- **Status bar wiring (`studio/status_bar.go`)** — FR-status. The `status_light`
  reflects the feed connection, `progress_bar`/`progress_ring` track the
  streaming job progress in lock-step, the `badge` reflects the live row count,
  and the caption names the active exhibit.
- **Coverage audit (`studio/coverage_test.go` + `studio/coverage_distinct_test.go`)** —
  FR-coverage and FR-coverage-distinct. A live-tree walk asserts the multiset of
  `(Family, TypeName)` reaches **48/48 standard marks** (the three §2.8 traps
  filtered); a companion review encodes each mark's distinctive behavior. This
  required placing the previously-unplaced action/feedback/navigation marks
  (split_button, menu_button, radial_menu, popup_palette, standalone toolbar,
  notification, tooltip, breadcrumbs, list_item, icon, list) with genuine homes
  in E6 and E1.
- **Framework feedback (P10):**
  - `F-resp` (spec) — the responsive contract uses store identity, never mark
    pointers: the wide and narrow arrangements are distinct mark instances bound
    to the same `ShellState` stores, so a crossing preserves state without
    re-parenting a live mark.
  - `F-signal-queue-race` *(framework fix, NFR-race)* — the forked projection
    system runs subtrees in parallel goroutines; a Derived recompute during a
    forked projection emits OnChange via `store.enqueueSignal` →
    `rt.queueSignal`, and two forked goroutines (a `viz.Line` via a
    Derived-backed `ReactiveScale` and a `StatusLight` via
    `marks.FromDerived[bool]`) appended to the unsynchronized `rt.signalQueue`
    concurrently — the same fork-race family the spec narrowed for `BindImpl`
    and the harfbuzz call, but on the signal-queue path. Fixed by guarding
    `rt.signalQueue` with `signalMu` in `queueSignal`/`deliverSignals`
    (`runtime/signals.go`), the same narrow `*Mu` pattern as the existing
    recovery/phase-hook guards. `go test ./demos/lurpic_studio/... -race` is
    clean under repeated runs (NFR-race / AC-7).
  - `F-rail-shape` — the `nav_rail` mark lays its items out vertically and
    cannot be re-hosted as a horizontal bottom bar; the narrow bottom action bar
    is a bespoke horizontal icon-bar host bound to the same `ActiveExhibit`
    store (the "nav_rail → bottom action bar" re-host in spirit).
  - `F-dirtylayout-routing` (from P9) — tab/store-driven layout must route
    through `RuntimeServices.Invalidate`; the shell's pane switching and the
    narrow overlays follow this.
- **E9/prior slice work** — E6 Mark Playground (tabs + interactive families),
  Capability Index, E1–E5, and the framework feedback from those slices
  (F-scroll-content, F-card-content, F-e6-internal, F-tabs-host).
- **F-overlay-precedent (resolved)** — E5's dirty-node highlighting now renders
  through the framework's `diagnostics.Overlay` (`HighlightDirty` +
  `DirtyFlagColor`), so the dirty-highlight drawing is a single source and E5
  no longer authors a parallel dirty-highlight renderer. Enforced by
  `TestE5_overlayPrecedentReuse`.
- **F-P7-five-waves (resolved)** — E5's wave suite now covers all five AC-10
  waves: E1 feed tick, E1 cell edit, E1 brush, **E2 layer toggle**
  (`TestE5_propagationWave_layerToggle`), and E4 policy resize.
- **F-window-pause-gesture (resolved)** — FR-window's "pan/zoom MUST set
  Paused" was both under- and over-triggered: a wheel zoom mutated the domain
  without pausing (so the next feed tick's `AnchorLiveWindow` overwrote it), and
  a plain `selectAt` left-click paused the feed. `chart_canvas.go` now pauses
  only on a real gesture — a pan once the drag passes the 4px threshold, or a
  wheel zoom — via an idempotent `pauseLive()`. Enforced by
  `TestRealtime_liveTailPauseAndJumpToLive` (now driving the real gestures) and
  `TestRealtime_selectionClickDoesNotPause`.
- **F-radial-reshape (resolved)** — the §3.3 E1 placement listed
  `radial_menu(chart reshape · radial layout)`, but the radial menu was only
  placed in E6 and unbound. E1 now hosts a `radial_menu` chart-reshape dial in
  the bottom strip: four `icon_button` radial children (one per chart type)
  write `ChartType`, re-projecting the canvas series. `icon_button` children
  are required because the radial policy arranges children with Radial
  placement, which the plain `button` mark's contract does not declare.
  Enforced by `TestRealtime_radialReshapeChangesChartType` and the coverage
  audit's E1 placement assertion.
- **Framework additions (the two NG-2 exceptions):**
  - `runtime/diagnostics_sink.go` + `frame.go` — `DirtySnapshotSink`
    (F-dirtysources): an opt-in `DiagnosticsHook` capability, discovered by type
    assertion (never widening `DiagnosticsHook` or `facet.RuntimeServices`),
    receiving the frame's dirty set + invalidation sources at the snapshot
    point.
  - `app/config.go` + `app/run.go` — `app.Config.Diagnostics` + startup
    `EnableDiagnostics` (F-diag-access).
  - The A/B neutrality test (`runtime/frame_neutrality_test.go`) proves the
    observer does not perturb the observed frame (median within 10%, p90 within
    25%).
- **Seed wired (F-seed-wired):** `main.go` embeds + parses `metrics.csv` and
  the shell's `AppState` is seeded; the shell golden now shows the seeded E1.
- **E1 (P5/P6)** — the live chart + streaming feed + live-tail window (FR-rt
  proven), the editable spreadsheet over `CollectionBinder` + linked brushing.
- Exhibit stage + E4 (P4), viz probe → ChartCanvas (P3), gallery shell (P2),
  seed/state (P1), app entry + debt (P0), and the framework feedback from those
  slices.

## Run

```
go run ./demos/lurpic_studio
```

Opens a ≥1280×800 window rendering the gallery shell. Software backend only
(no GPU driver variance; deterministic goldens).

## Test

```
go test ./demos/lurpic_studio/... -race
```

Uses `internal/testkit` (the repo's headless facet-driving harness) so tests
run without a window. `go run` and tests share the same embedded NotoSans
faces, so production and golden renders are glyph-identical.

## Slice P1 — design decisions and findings

- **F-row-id** — the monotonic-counter identify for `Rows` requires the
  counter to live on the row: `CollectionStore` re-derives item ids from
  items after removals, so identity must be deterministic per row and stable
  across edits. The spec's 3-field `Row` sketch omitted it; `dataset.Row.ID`
  (a `uint64`, never parsed from the seed) closes that gap. A data-derived id
  would collide on edit, which is exactly why the spec chose a counter.
- **F-derived-independence** — `VisibleRows`/`YDomain`/`BarBuckets` each list
  their sources explicitly and compute from the raw stores through shared pure
  helpers rather than reading sibling deriveds. A probe of `store.Derived`
  confirmed that a chained derived returns a *stale* cached value when its
  upstream sibling is marked dirty but not yet re-`Get()`'d, so chaining would
  make the topology correct only under a fragile call order. Independent
  source sets keep every derived correct in a single `Get()`.
- **F-users** — the `users` CSV column is validated strictly (non-negative
  integer) but not carried by `Row`; the flagship's metric is `revenue`, so
  `users` is unused by design (per spec §2.1 lock) and logged here in case a
  later exhibit wants it.
- **F-collection-evict** (spec) — the framework's `CollectionStore` has no cap
  API; `TrimToMax` hand-rolls bounded retention (evict oldest by `minID`). It
  is the app's only removal path, so `minID` always tracks the oldest live row.
- **Live window semantics** — `LiveWindow` is a closed `[now-W, now]` interval
  in unix seconds on a second-precision synthetic clock; `YDomain` is the
  `[lo, hi]` extent of the visible values with `hi` clamped to `YAxisMax`
  (`<= 0` disables the clamp), with degenerate/empty fallbacks so a scale
  never collapses.

## Slice P0 API re-verification (FR-1) — drift logged

Every §1 signature was re-read from HEAD on 2026-08-07 and pinned by
`api_verify_test.go`. Drift found, logged as Findings, and NOT silently
worked around:

- **F-drift-area-signature** — §1.8 sketched `NewArea` as taking a
  `baseline marks.Binding[float64]` argument; HEAD's `NewArea` has the same
  signature as `NewLine` (its baseline is a fixed internal
  `marks.Const(0.0)`). Corrected by the verify test.
- **F-drift-fontsource** — §1.1/P0 sketched
  `text.FontSource{Data, Family, Weight}`; HEAD's `text.FontSource` is
  `{Path, Data, Name}`. `main.go` loads fonts with `Name` to match the
  testkit harness convention.
- **F-capindex-internal** (P9 notice) — the Capability Index exhibit wants to
  display the `capindex` catalog, but `cmd/lurpiclint/internal/capindex` is
  internal to the `cmd/lurpiclint` tree and cannot be imported from
  `demos/` (Go internal-package rule). P9 must lift the generator out of
  `internal/` or re-implement a thin scan; this is a Finding for that slice.

## Findings tracking

Inline `F-*` findings are consolidated in `devdocs/plans/lurpic-studio-redesign.md`
§9 (Findings register). This demo is the first integrator of the never-consumed
layout/viz stack; defects found while building it are Findings, never
in-demo edits (NG-2).

## QA checklist (stub — filled per-slice)

- [ ] P0: `go run ./demos/lurpic_studio` opens a themed 1280×800 window.
- [ ] P0: `go test ./demos/lurpic_studio/... -race` green.
- [ ] P0: `cmake --build build --target lint` green.
- [ ] P0: `lurpic_studio` root binary untracked; `lurpic validate demos`
      references real mark families and real demos.
- [x] P9: E6 shows all six family tabs reachable and interactive (write-back
      loop asserted per family in `e6_playground_test.go`).
- [x] P9: Capability Index renders the `capindex`-generated catalog
      (`capability_index_test.go`); `e6_action`/`e6_selection` goldens are
      byte-distinct.
- [x] P10: responsive collapse below 960dp — index → nav_drawer + bottom
      action bar, inspector → bottom sheet, stage full-width; crossing
      preserves store versions + values (`responsive_test.go`).
- [x] P10: command palette — Ctrl+K and the chrome ⌘K button open it; a
      registered command switches the active exhibit (`shell_wiring_test.go`).
- [x] P10: status bar wired — connection light, progress bar/ring in lock-step,
      row-count badge, active-exhibit caption (`shell_wiring_test.go`).
- [x] P10: coverage audit — the live-tree walk reaches 48/48 standard marks
      (`coverage_test.go`); every placed mark carries a distinctive behavior
      (`coverage_distinct_test.go`).
- [x] P10: `go test ./demos/lurpic_studio/... -race`, `cmake --build build
      --target lint`, `--target test-unit`, `--target lint-lurpiclint-ci` all
      green (AC-7 gate).
