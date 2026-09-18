# lurpiclint Rules

This catalog covers the rules registered in
[`cmd/lurpiclint/internal/rules/`](../cmd/lurpiclint/internal/rules/).

The current tree registers the following rule IDs:

- LL001
- LL002
- LL003
- LL004
- LL010
- LL011
- LL012
- LL013
- LL014
- LL015

No rule files or registrations were found for LL005 through LL009 in the current
repository state.

## Rule Summary

| ID | Default Severity | Intent | Evidence |
|---|---|---|---|
| LL001 | warn | `facet.LayoutRole` `OnMeasure`/`OnArrange` populated (composite literal *or* field assignment) outside `layout/` or `marks/`; prefer composition. | [`reinvent_layoutrole.go`](../cmd/lurpiclint/internal/rules/reinvent_layoutrole.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL002 | warn | Absolute-coordinate placement via `gfx.RectFromXYWH` in a layout path; prefer relative layout. | [`reinvent_coords.go`](../cmd/lurpiclint/internal/rules/reinvent_coords.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL003 | error | Child-arranging `LayoutRole`; use an existing container or mark. | [`reinvent_container.go`](../cmd/lurpiclint/internal/rules/reinvent_container.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL004 | info | Child-arranging facet matches a known built-in capability; consider using it directly. | [`suggest_shapematch.go`](../cmd/lurpiclint/internal/rules/suggest_shapematch.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL010 | error | `facet` or `projection` package imports `render`. | [`contract_render_import.go`](../cmd/lurpiclint/internal/rules/contract_render_import.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL011 | error | Goroutine or channel operation in facet code; use `job.Schedule` instead. | [`contract_facet_goroutine.go`](../cmd/lurpiclint/internal/rules/contract_facet_goroutine.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL012 | warn | Facet holds domain state in a field; keep facets stateless. | [`contract_domain_state.go`](../cmd/lurpiclint/internal/rules/contract_domain_state.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL013 | warn | Theme token captured in `OnAttach` or constructor; resolve at projection time instead. | [`contract_token_attach.go`](../cmd/lurpiclint/internal/rules/contract_token_attach.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL014 | error | Overlay mark missing layer registration, hit policy, or dismissal trigger. | [`contract_overlay.go`](../cmd/lurpiclint/internal/rules/contract_overlay.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL015 | error | Mark declares stability without verified evidence. | [`contract_stability.go`](../cmd/lurpiclint/internal/rules/contract_stability.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL016 | error | Store mutation in an `OnMeasure`/`OnArrange` callback; layout callbacks must be read-only. | [`contract_layout_mutation.go`](../cmd/lurpiclint/internal/rules/contract_layout_mutation.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL017 | warn | `app.Asset` used with a media file; use `Manager.Load*` for images, fonts, and large assets. | [`contract_asset_bootstrap.go`](../cmd/lurpiclint/internal/rules/contract_asset_bootstrap.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL018 | warn | Overlay value not mounted via `AddChild`/`AddChildRuntime`; may render incorrectly. | [`contract_overlay_mounted.go`](../cmd/lurpiclint/internal/rules/contract_overlay_mounted.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL019 | error | Facet embeds `facet.Facet` and adds children but registers no `LayoutRole`; children never arrange. | [`reinvent_no_layout_role.go`](../cmd/lurpiclint/internal/rules/reinvent_no_layout_role.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL020 | error | Direct `LayoutRole` callback invocation or field write; use the public `Arrange`/`Measure` methods. | [`contract_layout_callback_access.go`](../cmd/lurpiclint/internal/rules/contract_layout_callback_access.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL021 | error | Overlay mounted as a plain child without a layer attachment; use `facet.AttachLayer` with a `ZBand` instead. | [`contract_sibling_overlay.go`](../cmd/lurpiclint/internal/rules/contract_sibling_overlay.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL022 | error | Column/Row stack of only `Fixed` children with no `ScrollRegion`; overflow has nowhere to go. | [`contract_unbounded_fixed_stack.go`](../cmd/lurpiclint/internal/rules/contract_unbounded_fixed_stack.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL023 | error | Reactive binding overwritten by `marks.Const` or caller-supplied store reassigned; mutate the store instead. | [`contract_reactive_overwrite.go`](../cmd/lurpiclint/internal/rules/contract_reactive_overwrite.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL024 | error | Constructor accepts a caller-supplied store and also manufactures one; use the caller's store. | [`contract_value_store_manufacture.go`](../cmd/lurpiclint/internal/rules/contract_value_store_manufacture.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL025 | error | Viz mark declares a `*reactive.ReactiveScale` field without a `signal.Track` subscription in `OnAttach`. | [`contract_viz_scale_subscribe.go`](../cmd/lurpiclint/internal/rules/contract_viz_scale_subscribe.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL026 | error | Named cache struct echoes domain data without a version field; add `version uint64` from `store.Version()`. | [`contract_cache_version.go`](../cmd/lurpiclint/internal/rules/contract_cache_version.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL027 | error | String formatting in `signal.Emit`; use a typed signal payload instead. | [`contract_signal_format.go`](../cmd/lurpiclint/internal/rules/contract_signal_format.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL028 | warn | Hardcoded `gfx.Color`, font, or size literal in a viz mark; use theme tokens instead. | [`contract_viz_theme.go`](../cmd/lurpiclint/internal/rules/contract_viz_theme.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL029 | error | Mark declares the DataBound capability without a contract proof (`contracttest.AssertDataBound`). | [`contract_databound.go`](../cmd/lurpiclint/internal/rules/contract_databound.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL030 | error | Mark implements AnchorExporting without a contract proof (`contracttest.AssertAnchorExport`). | [`contract_anchor.go`](../cmd/lurpiclint/internal/rules/contract_anchor.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL031 | error | Container mark exposes `Children()` without a contract proof (`contracttest.AssertGroupChildren`). | [`contract_group_children.go`](../cmd/lurpiclint/internal/rules/contract_group_children.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL032 | error | Mark implements Accessible without a contract proof (`contracttest.AssertAccessible`). | [`contract_accessible.go`](../cmd/lurpiclint/internal/rules/contract_accessible.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL033 | error | Mark implements Focusable without a contract proof (`contracttest.AssertFocusable`). | [`contract_focusable.go`](../cmd/lurpiclint/internal/rules/contract_focusable.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL034 | error | Layer-attached facet must not also be a group-arranged child (RX-1 Q4 layer exclusivity). | [`contract_layer_exclusive.go`](../cmd/lurpiclint/internal/rules/contract_layer_exclusive.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL035 | warn | `OnProject` closures must not write stores; projection is read-only. | [`contract_layer_exclusive.go`](../cmd/lurpiclint/internal/rules/contract_layer_exclusive.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |
| LL036 | warn | `GridRows`/`GridColumns` count above 16 is suspicious; counts do not size tracks (RX-1 Q6 / FR-6). | [`contract_grid_count.go`](../cmd/lurpiclint/internal/rules/contract_grid_count.go) and [`rules_test.go`](../cmd/lurpiclint/internal/rules/rules_test.go) |

## Notes

- LL004 depends on the capability index produced from the registered framework
  packages.
- The catalogue above is intentionally scoped to what is actually registered in
  the current source tree.
- The rule code and tests live under `cmd/lurpiclint/internal/rules/`, not the
  nonexistent `internal/rules/` path called out in older notes.
