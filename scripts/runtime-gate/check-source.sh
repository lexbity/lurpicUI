#!/usr/bin/env bash
set -euo pipefail

root=${1:-.}
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
