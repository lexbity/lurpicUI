#!/usr/bin/env bash
set -euo pipefail

root=${1:-.}
phase=${2:-P1}
cd "$root"

expect_absent() {
	if rg -n "$@"; then
		printf '%s\n' 'forbidden source pattern found' >&2
		return 1
	else
		runtime_gate_status=$?
		if [ "$runtime_gate_status" -ne 1 ]; then
			return "$runtime_gate_status"
		fi
	fi
}

for package in internal/testevidence render/canonical runtime drive/server internal/testkit drive; do
	files=$(rg --files "$package" -g '*.go' -g '!**/*_test.go')
	if [ -z "$files" ]; then
		printf 'source inventory is empty for %s\n' "$package" >&2
		exit 1
	fi
done

expect_absent 'var[[:space:]]+driveClients|driveClients[[:space:]]*=' internal/testkit/drive.go
expect_absent 'if[[:space:]]+testkit\.RaceEnabled' drive/determinism_test.go drive/determinism_thousand_test.go drive/determinism_race_test.go
expect_absent 'layoutStart := rt\.clock\.Now|projStart := rt\.clock\.Now|renderStart := rt\.clock\.Now' runtime/frame.go
expect_absent 'recipeStart := rt\.clock\.Now|measureStart := rt\.clock\.Now|arrangeStart := rt\.clock\.Now' runtime/layers_runtime.go
expect_absent 'FramePacketHash' runtime
expect_absent 'reflect\.DeepEqual' internal/renderutil

grep -q '^//go:build !race$' drive/determinism_thousand_test.go
grep -q '^//go:build race$' drive/determinism_race_test.go
grep -q 'func TestDeterminismThousandConsecutiveFrames' drive/determinism_thousand_test.go
grep -q 'func TestDeterminismRaceScript120Frames' drive/determinism_race_test.go
grep -q 'func TestCanonicalEveryFieldMutation' render/canonical/encode_test.go
grep -q 'func TestCanonicalCommandInventory' render/canonical/encode_test.go

printf '%s\n' 'runtime-gate: P1 source contracts passed'

if [[ "$phase" == *P2* || "$phase" == *P3* || "$phase" == *P4* ]]; then
	for package in store app scale/reactive marks demos/lurpic_studio; do
		files=$(rg --files "$package" -g '*.go' -g '!**/*_test.go')
		if [ -z "$files" ]; then
			printf 'source inventory is empty for %s\n' "$package" >&2
			exit 1
		fi
	done
	expect_absent 'SetSignalQueueHook|SnapshotFrameDerivedCounters|derivedMu|deriveds[[:space:]]+\[\]' store runtime app scale/reactive marks demos/lurpic_studio
	expect_absent 'defaultScope|currentRuntime|currentScope' store runtime app scale/reactive marks demos/lurpic_studio
	grep -q 'func (s \*Scope) registerDerived' store/scope.go
	grep -q '^func NewValueStore\[T any\](scope \*Scope,' store/value.go
	grep -q '^func NewCollectionStore\[T any\](scope \*Scope,' store/collection.go
	grep -q '^func NewDerived\[T any\](scope \*Scope,' store/derived.go
	grep -q '^func Begin(scope \*Scope)' store/transaction.go
	grep -q 'func (s \*Scope) Close()' store/scope.go
	grep -q 'func (s \*Scope) FlushSignals()' store/scope.go
	grep -q 'func (s \*Scope) FlushDerived()' store/scope.go
	printf '%s\n' 'runtime-gate: P2 source contracts passed'
fi

if [[ "$phase" == *P3* || "$phase" == *P4* ]]; then
	for package in runtime facet marks/navigation marks/structure app; do
		files=$(rg --files "$package" -g '*.go' -g '!**/*_test.go')
		if [ -z "$files" ]; then
			printf 'source inventory is empty for %s\n' "$package" >&2
			exit 1
		fi
	done
	expect_absent 'findFacetByID|findFacetByID\(' runtime
	expect_absent '\.Children\(\)' runtime -g '*.go' -g '!**/*_test.go'
	grep -q 'func (rt \*Runtime) liveFacet(id facet.FacetID)' runtime/layout.go
	grep -q 'rt.liveIndex.lookup(id)' runtime/layout.go
	grep -q 'func (f \*Facet) VisitChildren' facet/facet.go
	grep -q 'func (f \*Facet) VisitChildrenReverse' facet/facet.go
	grep -q 'OnFacetTreeAttached:.*rt.onFacetTreeAttached' runtime/layout.go
	grep -q 'OnFacetDisposed:.*rt.onFacetDisposed' runtime/layout.go
	grep -q 'OnHostedChildrenChanged:.*rt.onHostedChildrenChanged' runtime/layout.go
	grep -q 'rtConfig.HostedChildren = marks.HostedChildrenFor' app/run.go
	grep -q 'func (index \*liveIndex) registerHosted' runtime/live_index.go
	grep -q 'func (index \*liveIndex) removeSubtree' runtime/live_index.go
	grep -q 'func TestIdentityLookupNoTraversalOrAllocation' runtime/live_index_test.go
	grep -q 'func BenchmarkIdentityLookupSweep' runtime/live_index_test.go
	printf '%s\n' 'runtime-gate: P3 source contracts passed'
fi

if [[ "$phase" == *P4* ]]; then
	for package in text marks runtime app internal/membudget demos/lurpic_studio; do
		files=$(rg --files "$package" -g '*.go' -g '!**/*_test.go')
		if [ -z "$files" ]; then
			printf 'source inventory is empty for %s\n' "$package" >&2
			exit 1
		fi
	done
	expect_absent 'text\.NewShaper|SetContentScale' marks demos/lurpic_studio runtime -g '*.go' -g '!**/*_test.go'
	expect_absent 'NewShaper' text marks runtime app demos -g '*.go' -g '!**/*_test.go'
	expect_absent 'sync\.Pool' internal/membudget text -g '*.go' -g '!**/*_test.go'
	grep -q '^func ShaperFor(runtime any) \*text.Service' marks/runtime_access.go
	grep -q '^func NewService(registry \*FontRegistry, limits Limits)' text/service.go
	grep -q '^func NewFontRegistry(wallet \*membudget.Wallet, maxFaces int)' text/registry.go
	grep -q 'func (s \*Service) Shape(r ShapeRequest) (\*LayoutLease, error)' text/service.go
	grep -q 'func (s \*Service) Trim(targetBytes uint64)' text/service.go
	grep -q 'func (p ResourcePolicy) Validate() error' runtime/resource_policy.go
	grep -q 'func (w \*Wallet) Reserve(bytes uint64) (\*Reservation, error)' internal/membudget/plan.go
	grep -q 'func (r \*Reservation) Cancel() error' internal/membudget/plan.go
	printf '%s\n' 'runtime-gate: P4 source contracts passed'
fi

if [[ "$phase" == *P5* ]]; then
	for package in projection runtime facet platform platform/android platform/linux/internal/display; do
		files=$(rg --files "$package" -g '*.go' -g '!**/*_test.go')
		if [ -z "$files" ]; then
			printf 'source inventory is empty for %s\n' "$package" >&2
			exit 1
		fi
	done
	expect_absent 'go[[:space:]]+func|walkChildSubtrees|ProjectionOutputPartitions' projection/projection.go
	expect_absent 'append\(\[\]func\(time\.Duration\)\{\}, rt\.phase1Hooks\.\.\.' runtime/phase_hooks_runtime.go
	expect_absent 'rt\.dirtyFacets = make|rt\.dirtySources = make' runtime/frame.go
	expect_absent '^[[:space:]]*(Focusable|TabIndex)[[:space:]]' facet/roles.go
	grep -q 'func (rt \*Runtime) recordInvalidation' runtime/dirty_work.go
	grep -q 'nextDirtyFacets' runtime/dirty_work.go
	grep -q 'phase1HookActive' runtime/phase_hooks_runtime.go
	grep -q 'func (rt \*Runtime) compactPhase1HooksLocked' runtime/phase_hooks_runtime.go
	grep -q 'func (rt \*Runtime) hasWallFrameWork' runtime/lifecycle.go
	grep -q 'func (rt \*Runtime) waitForWallWork' runtime/lifecycle.go
	grep -q 'OnFacetInvalidated func' facet/context.go
	grep -q 'MarkDirtyFlags(id, flags)' runtime/dirty_work.go
	expect_absent 'collectDirtyFlags|propagateDirty|clearTreeDirty|managedDirty|SetManagedInvalidations' projection/projection.go runtime
	grep -q 'func TestRuntimeOwnedProjectionQuietFrameSkipsTreeWalk' projection/projection_thread_test.go
	grep -q 'func TestRuntimeOwnedLayoutInvalidationFollowsChangedGeometry' projection/projection_thread_test.go
	grep -q 'func TestSwitchedGolden_stageReproducesFreshAfterExhibitCycle' demos/lurpic_studio/studio/switched_golden_test.go
	grep -q 'func TestPendingHoverDeadlineWakesIdle' runtime/idle_test.go
	grep -q 'func TestFrameTimerWakeWithoutRemainingWorkDoesNotAuthorizeFrame' runtime/smoke_test.go
	grep -q 'frameEligible func() bool' runtime/config.go
	grep -q 'func TestReadyWallJobRequestsFrame' runtime/idle_test.go
	grep -q 'func TestIdleVirtualJobDoesNotAdvance' runtime/idle_test.go
	grep -q 'SetFrameDemand(bool)' platform/platform.go
	grep -q 'syncVsyncDemand' platform/android/android.go
	grep -q 'runOnUiThread(this::nativeApplyVsyncDemand)' cmd/lurpic/androidtmpl/LurpicNativeActivity.java
	grep -q 'nativeApplyVsyncDemand(' platform/android/internal/bridge/lurpic_android.c
	grep -q 'g_vsync_update_posted' platform/android/internal/bridge/lurpic_android.c
	grep -q 'g_vsync_callback_pending' platform/android/internal/bridge/lurpic_android.c
	grep -q 'func (a \*app) runEventPump' platform/linux/internal/display/display_cgo.go
	grep -q 'func TestWallRuntimeParksAfterBootstrapUntilFrameRequest' runtime/idle_test.go
	grep -q 'func TestProjectionCallbacksStaySerialAndPreOrder' projection/projection_thread_test.go
	grep -q 'func TestPhase1HookReleaseCompactsActiveEntries' runtime/phase_hooks_runtime_test.go
	printf '%s\n' 'runtime-gate: P5 source contracts passed'
fi

if [[ "$phase" == *P6* ]]; then
	for package in gfx render runtime text; do
		files=$(rg --files "$package" -g '*.go' -g '!**/*_test.go')
		if [ -z "$files" ]; then
			printf 'source inventory is empty for %s\n' "$package" >&2
			exit 1
		fi
	done
	expect_absent 'type[[:space:]]+(RenderBatch|LayeredBatch|FramePacket)[[:space:]]+struct|RenderBatchs[[:space:]]+\[\]RenderBatch' render
	expect_absent 'NewCommandPage\(' gfx render
	expect_absent 'reflect\.DeepEqual' internal/renderutil render
	grep -q 'func SealCommandPageFromSource' gfx/command_pages.go
	grep -q 'func (f \*Frame) Retain()' render/frame_pages.go
	grep -q 'func (f \*Frame) Generation()' render/frame_pages.go
	grep -q 'ResourceGeneration uint64' runtime/pipeline.go
	grep -q 'TryReserveCleanupFrame' runtime/pipeline.go
	grep -q 'SubmitCleanupReserved' runtime/pipeline.go
	grep -q 'RetainLastPrimaryFrame' runtime/core.go
	grep -q 'PinForRender' text/registry.go
	grep -q 'func TestFramePagesWaitForAllConsumers' render/frame_test.go
	grep -q 'func TestCleanupCreditSurvivesBlockedRenderer' runtime/pipeline_test.go
	grep -q 'func TestFailedAndLateFrameAcknowledgmentsPreserveLastReady' runtime/frame_lifetime_test.go
	grep -q 'func TestSealedGlyphPagePinsFontChargeUntilRelease' text/render_page_lifetime_test.go
	printf '%s\n' 'runtime-gate: P6 source contracts passed'
fi
