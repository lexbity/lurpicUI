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

## Notes

- LL004 depends on the capability index produced from the registered framework
  packages.
- The catalogue above is intentionally scoped to what is actually registered in
  the current source tree.
- The rule code and tests live under `cmd/lurpiclint/internal/rules/`, not the
  nonexistent `internal/rules/` path called out in older notes.
