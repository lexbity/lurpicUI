# UI Replay

A deterministic scenario runner for the lurpicUI engine.

## Purpose

`ui_replay` answers:
- Did the scenario run the same way?
- Did the scene reach the expected state?
- Did the artifact bundle match the baseline?
- Did portability differences appear?

This is the machine-usable execution product for regression testing.

## Building

```bash
cd /home/lex/Public/lurpicUI/demos/ui_replay_app
go build -o ui_replay .
```

## Running

```bash
./ui_replay
# or with custom window size
./ui_replay -width=1600 -height=900
# or with scenario directory
./ui_replay -scenario-dir=./testdata/scenarios
```

## Testing

```bash
go test ./...
```

## Architecture

The application follows the repo's shared engineering constraints:

- Domain truth lives in stores (`store/`)
- Facets are projection boundaries (`ui/`)
- Runtime owns mutable state
- Layout, projection, input, signals, and rendering remain phase-separated

## Phase 1 - Shell, Loader, And Empty Run

**Status**: Complete

Deliverables:
- [x] Executable bootstrap
- [x] Scenario selection
- [x] Empty run/history panes
- [x] Environment summary
- [x] Current status display

Exit criteria:
- [x] The app can start and select scenarios

## Phase 2 - Scenario Schema And Validation

**Status**: Complete

Deliverables:
- [x] Scenario metadata (capabilities, description, expected state)
- [x] Environment presets
- [x] Action schema with validation
- [x] Assertion schema with validation
- [x] Artifact schema with validation
- [x] Schema versioning (1.0 with support list)
- [x] Stable target model (logical IDs + coordinate fallback)
- [x] Scenario capability declarations
- [x] Expected-state metadata
- [x] Structured validation errors
- [x] Load result tracking (loaded/invalid/error)

Exit criteria:
- [x] Scenarios can be validated before execution
- [x] Every scenario can declare what it needs from the scene and runtime

## Phase 3 - Deterministic Execution Core

**Status**: Complete

Deliverables:
- [x] Scene reset with canonical state restoration
- [x] Environment application (theme, density, window size)
- [x] Action dispatch for all action types
- [x] Frame stepping (`wait_frames`)
- [x] Idle/stability waits (`wait_idle`)
- [x] Controlled time advancement (frame counter)
- [x] Deterministic event scheduler
- [x] Stable checkpoint gating
- [x] Commit validation for background work
- [x] Scene-state normalization before execution

Tests:
- [x] Repeated runs preserve step order
- [x] Frame-count waits behave predictably
- [x] Scene reset returns to canonical state
- [x] Background job results only commit when versions match
- [x] Idle/stability waits do not advance too early
- [x] Resets do not carry stale theme, focus, or selection state

Exit criteria:
- [x] Scenarios can run end-to-end with deterministic control
- [x] Every action advances through the same observable run states

## Phase 4 - Assertions And Semantic Logging

**Status**: Complete

Deliverables:
- [x] Assertion engine with 10 assertion types
- [x] Semantic event capture (action, event, state, summary)
- [x] Store summary capture
- [x] Signal summary capture
- [x] Focus summary capture
- [x] Diagnostics summary capture
- [x] Render batch summary capture (placeholder)
- [x] Scene capability summary capture (placeholder)
- [x] Explicit assertion ordering (step-based)
- [x] Structured failure reasons (AssertionResult with Message, Expected, Actual)

Tests:
- [x] Assertions pass/fail deterministically
- [x] Logs preserve semantic ordering (timestamp-based)
- [x] Event summaries match the executed actions
- [x] Assertion failures report the correct step and reason
- [x] Semantic summaries match the final scene state
- [x] Diagnostic summaries are stable enough to diff across runs

Exit criteria:
- [x] The replay result can be validated without raw UI inspection
- [x] A failed run is readable without opening the live app

## Phase 5 - Artifact Capture And Bundle Packaging

**Status**: Complete

Deliverables:
- [x] Screenshot capture artifact type
- [x] Scene state export (JSON)
- [x] Diagnostics snapshot export
- [x] Log export
- [x] Assertion report export
- [x] Final bundle packaging (directory and ZIP)
- [x] Stable bundle manifest (version 1.0)
- [x] Artifact path normalization (lowercase, underscores)
- [x] Scenario/environment provenance data
- [x] SHA-256 checksums for integrity
- [x] Bundle validation (version, checksums)

Tests:
- [x] Bundle contents are stable across runs
- [x] Artifact names are deterministic (normalized)
- [x] Bundle metadata includes correct environment identity
- [x] Screenshots, logs, and snapshots match scenario identity
- [x] Bundle output is self-describing for offline inspection
- [x] ZIP and directory formats are loadable

Exit criteria:
- [x] Replay output can be shared and diffed
- [x] Another engineer can open the bundle without the live app

## Phase 6 - Multi-Scene, Theme, And Density Coverage

**Status**: Complete

Deliverables:
- [x] Multi-scene scenario chain support
- [x] Scene transition tracking (SceneManager with history)
- [x] Theme switching support with history
- [x] Density switching support with history
- [x] Target re-resolution across scene changes
- [x] Backend comparison mode
- [x] Platform comparison mode
- [x] Theme comparison mode
- [x] Density comparison mode
- [x] Diff reporting between variants
- [x] Phase state capture (scene/theme/density at each step)

Tests:
- [x] Scenarios span multiple scenes correctly
- [x] Theme changes are recorded and queryable
- [x] Density changes are recorded and queryable
- [x] Targets resolve correctly after scene transitions
- [x] Backend variants produce comparable results
- [x] Platform variants produce comparable results
- [x] Cross-scene assertions reference correct state
- [x] Scene changes carry no stale state from previous scenes

Exit criteria:
- [x] A scenario can load scenes in sequence
- [x] Every scene change triggers target re-resolution
- [x] Comparison modes highlight cross-theme/density differences

## Phase 7 - Nondeterminism Detection And Hardening

**Status**: Complete

Deliverables:
- [x] Drift detection between runs (DriftDetector with fingerprints)
- [x] Drift classification (timing, state, environment, artifact)
- [x] Unstable wait handling with retry and backoff
- [x] Commit gating with version validation
- [x] Environment normalization (noise field removal, timestamp normalization)
- [x] Artifact diff summaries
- [x] Rerun comparison hooks (OnBeforeRerun, OnAfterRerun, OnDriftDetected)
- [x] Drift severity classification (warning, error, critical)
- [x] Stability report generation

Tests:
- [x] Drift is detected when frame counts differ
- [x] Drift is detected when step counts differ
- [x] Drift is detected when timing exceeds tolerance
- [x] Tolerance prevents false positives for acceptable variance
- [x] Unstable waits are retried and detected
- [x] Commit gates block stale data
- [x] Environment noise is removed from comparisons
- [x] Rerun hooks fire correctly

Exit criteria:
- [x] Nondeterminism is detected during nightly runs
- [x] The same scenario produces the same output on every run

## Phase 8 - Portable Regression Suites

**Status**: Complete

Deliverables:
- [x] Backend matrix execution across multiple backends (software, vulkan)
- [x] Platform matrix execution across multiple platforms (linux, windows, macos)
- [x] Scenario subsets with filtering (by tag, capability, family)
- [x] Portability summary reporting
- [x] Stable baseline management per matrix cell
- [x] Artifact comparison policy per matrix cell
- [x] Parallel matrix execution with concurrency control
- [x] Matrix cell callbacks (OnCellStart, OnCellComplete, OnCellError)

Tests:
- [x] Backend variants execute correctly
- [x] Platform variants execute correctly
- [x] Scenario subsets filter correctly
- [x] Portability report detects cross-platform differences
- [x] Baselines are recorded and retrievable per cell
- [x] Baseline comparison detects drift
- [x] Parallel execution completes without deadlock

Exit criteria:
- [x] Scenarios run across all backend/platform combinations
- [x] Cross-platform differences are flagged in reports
- [x] Nightly regression suite is fully automated

## File Structure

```
ui_replay_app/
├── main.go                 # Application entry point
├── deps.go                 # Package imports for facet registration
├── go.mod, go.sum          # Module definition
├── README.md               # This file
├── model/
│   ├── scenario.go         # Scenario types and validation
│   ├── scenario_test.go    # Validation tests
│   └── metadata.go         # Build metadata
├── store/
│   ├── registry.go         # Scenario registry with load results
│   ├── registry_test.go    # Registry tests
│   ├── execution.go        # Execution state and history
│   └── environment.go      # Environment configuration
├── engine/
│   ├── runner.go           # Scenario execution engine
│   ├── runner_test.go      # Execution tests
│   ├── assertion.go        # Assertion engine
│   ├── assertion_test.go   # Assertion tests
│   ├── logger.go           # Semantic logging
│   ├── logger_test.go      # Logging tests
│   ├── scene_manager.go    # Multi-scene transition tracking
│   ├── scene_manager_test.go # Scene manager tests
│   ├── comparison.go       # Comparison modes (theme/density/backend/platform)
│   ├── comparison_test.go  # Comparison tests
│   ├── drift_detector.go   # Nondeterminism detection and classification
│   ├── drift_detector_test.go # Drift detection tests
│   ├── commit_gate.go      # Commit gating and unstable wait handling
│   ├── commit_gate_test.go # Commit gate tests
│   ├── regression_matrix.go # Portable regression matrix execution
│   └── regression_matrix_test.go # Regression matrix tests
├── artifact/
│   ├── bundle.go           # Bundle builder and manifest
│   └── bundle_test.go      # Bundle tests
├── ui/
│   ├── shell.go            # Layout constants and bounds
│   ├── root.go             # Root facet managing shell layout
│   ├── header.go           # Header with scenario/env/status
│   ├── sidebar.go          # Scenario list with validation stats
│   ├── content.go          # Main scenario viewport
│   ├── inspector.go        # Diagnostics panel
│   └── footer.go           # Status bar
└── testdata/
    └── scenarios/          # Sample scenario files
```
