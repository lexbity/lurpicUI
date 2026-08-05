no stubs
no shims
no aliases
no compat

# Verification
For Go code changes, run `cmake --build build --target lint` before handing work back.
This runs both golangci-lint (repo quality) and lurpiclint (framework-contract checking).

To run tests: `cmake --build build --target test-unit`

# Building apps with lurpicUI
When building an application with lurpicUI, use the lurpiclint analyzer to get capability awareness
and check for framework-contract violations:

- `lurpiclint capabilities` — prints the uxauthoring index (available marks, layouts, and fingerprints)
- `lurpiclint check ./...` — runs the analyzer; use `--fail-on error` as your blocking gate
