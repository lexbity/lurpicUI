package runtime

import (
	"testing"
	"time"
)

// TestReads_DoNotPerturbFrames is the QP-9 proof (G4.2): interleaving the
// full read surface — store.read, store.list, derived.read, facet.query,
// frame.packet — between frames leaves the frame packet trajectory
// byte-identical to the same frames without reads. An observation that
// perturbs the observed frame (a derived flushed early, a cache key shifted)
// diverges the hashes.
func TestReads_DoNotPerturbFrames(t *testing.T) {
	hashesWithout := readPerturbationRun(t, false)
	hashesWith := readPerturbationRun(t, true)
	if len(hashesWithout) != len(hashesWith) {
		t.Fatalf("trajectory lengths differ: %d vs %d", len(hashesWithout), len(hashesWith))
	}
	for i := range hashesWithout {
		if hashesWithout[i] != hashesWith[i] {
			t.Fatalf("read perturbation diverged the packet trajectory at frame %d: %#x vs %#x",
				i, hashesWithout[i], hashesWith[i])
		}
	}
}

func readPerturbationRun(t *testing.T, withReads bool) []uint64 {
	t.Helper()
	rt, probe := newReadRuntime(t)
	probe.Rows.Insert("gamma") // give the derived real sources
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rt.RunOneFrameAt(epoch) // settle

	hashes := make([]uint64, 0, 8)
	for i := 0; i < 6; i++ {
		if withReads {
			// Interleave every read method, including derived.read (the
			// dangerous one — it touches the derived's cache state) and
			// facet.query with details.
			driveCall(t, rt, "store.read", map[string]any{
				"target": map[string]string{"testid": "probe"}, "field": "Opacity",
			}, allGrants())
			driveCall(t, rt, "derived.read", map[string]any{
				"target": map[string]string{"testid": "probe"}, "field": "Computed",
			}, allGrants())
			driveCall(t, rt, "facet.query", map[string]any{
				"target": map[string]string{"testid": "probe"}, "details": true,
			}, allGrants())
			driveCall(t, rt, "store.list", nil, allGrants())
			driveCall(t, rt, "frame.packet", nil, allGrants())
		}
		rt.RunOneFrameAt(epoch.Add(time.Duration(i+1) * time.Second))
		hashes = append(hashes, rt.FramePacketHash())
	}
	return hashes
}
