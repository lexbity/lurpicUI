//go:build !race

package testkit

// RaceEnabled reports whether the binary was built with the race detector.
// The alive-coverage suite (demos/lurpic_studio) uses it to skip its heavy
// deterministic pixel/interaction walks under -race: those tests exercise no
// concurrency (the framework's forked-projection and signal-queue paths are
// covered by the other studio tests), and the race-instrumented shell render
// inflates their runtime ~18x, pushing the studio race suite past the default
// 600s go test timeout (RX-1 FR-20 / NFR-4).
const RaceEnabled = false
