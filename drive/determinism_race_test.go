//go:build race

package drive_test

import "testing"

func TestDeterminismRaceScript120Frames(t *testing.T) {
	first := replayStudioSession(t, 120)
	second := replayStudioSession(t, 120)
	assertReplayEqual(t, first, second, 120)
}
