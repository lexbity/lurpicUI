# Lurpic Studio

Realtime interactable documentation for the lurpicUI framework: a gallery of
live exhibits, each demonstrating one framework capability interactively.
See `devdocs/plans/lurpic-studio-redesign.md` for the full specification.

## Status

Slice P2 of the multi-slice plan is in place (P0/P1 done previously):

- **Gallery shell (`studio/`)**: Root (linear.Vertical group-parent via
  `layout/linear.New` — the policy's first production consumer), ChromeStack
  (title + ⌘K + theme triggers, right-aligned), GallerySplit (the bespoke
  `layout/split` host — first production consumer; 3 panes: index | stage |
  inspector with static gutters), StatusBar, and placeholder Card panes.
  Rendered 1280×800 golden (`studio/testdata/golden/linux/shell.png`).
- **Framework feedback (Slice P2 surfaced these):**
  - **F-fork-race (fixed in `facet/facet.go`)**: the runtime's forked
    projection (trees > 8 nodes) + theme style resolution
    (`NearestStyleContext` → `FacetByID` → walks the tree calling `Base()`)
    raced on `facet.impl` because `BindImpl` re-wrote it on every `Base()`
    call. `BindImpl` is now write-once (bound during single-threaded attach;
    later calls are read-only no-ops). This was required for the demo's
    NFR-race gate and is a genuine immutability fix.
  - **F-linear-marks / F-badge-contract**: the standard marks mostly don't
    declare `SupportsLinear`, so `layout/linear` cannot host them directly;
    ChromeStack/StatusBar hand-arrange their marks (toolbar idiom) and pick a
    placement each mark's contract supports.
  - **F-lint-hosts**: the bespoke hosts carry documented
    `//lurpiclint:ignore * -- <reason>` directives (LL001/LL003 on the raw
    LayoutRole, LL021 on the chrome's AddChild lines).
- Seed data + parser (`dataset/`), store topology (`state/`), app entry
  (`main.go`), API verification, smoke tests, and debt clearance from P0/P1.

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
