no stubs
no shims
no aliases
no compat
no copy-paste plumbing

# De-duplication
Shared mark plumbing lives in the marks root package exactly once — use it, do
not re-declare it per mark:

- font registry / shaper access: `marks.ShaperFor`, `marks.FontRegistryFrom`
- icon resolution and rendering: `marks.IconResolverFrom`, `marks.ResolveIconFrom`, `marks.IconCommands`
- interaction-state decision table: `marks.InteractionStateFor`

Do not re-declare runtime-provider interfaces (`FontRegistry` / `IconResolver`
/ `ResolveIcon` duck types), hover/press/focus decision tables, or other
shared plumbing inside a mark. lurpiclint LL038 enforces the provider rule;
new shared capability probes are added to the marks root package once.

# Verification
For Go code changes, run `cmake --build build --target lint` before handing work back.
This runs both golangci-lint (repo quality) and lurpiclint (framework-contract checking).

To run tests: `cmake --build build --target test-unit`
Drive protocol suite: `cmake --build build --target test-drive`
Differential verb-equivalence suite: `cmake --build build --target test-equivalence`

To reclaim disk space after long test/check sessions: `cmake --build build --target clean-cache`
(deletes build/go-cache, build/go-tmp, and the Rust target/ directory; preview with
`scripts/clean-cache.sh --dry-run`).

# Building apps with lurpicUI
When building an application with lurpicUI, use the lurpiclint analyzer to get capability awareness
and check for framework-contract violations:

- `lurpiclint capabilities` — prints the uxauthoring index (available marks, layouts, and fingerprints)
- `lurpiclint check ./...` — runs the analyzer; use `--fail-on error` as your blocking gate
