//go:build !race

package drive_test

import "testing"

func TestDeterminismThousandConsecutiveFrames(t *testing.T) {
	first := replayStudioSession(t, 1000)
	second := replayStudioSession(t, 1000)
	assertReplayEqual(t, first, second, 1000)
}
