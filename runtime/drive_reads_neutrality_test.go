package runtime

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/render/canonical"
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
			t.Fatalf("read perturbation diverged the canonical packet trajectory at frame %d: %s vs %s",
				i, hashesWithout[i], hashesWith[i])
		}
	}
}

func readPerturbationRun(t *testing.T, withReads bool) []string {
	t.Helper()
	rt, probe := newReadRuntime(t)
	probe.Rows.Insert("gamma") // give the derived real sources
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rt.RunOneFrameAt(epoch) // settle

	hashes := make([]string, 0, 8)
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
		frame, err := rt.RetainLastPrimaryFrame()
		if err != nil {
			t.Fatalf("retain ready frame: %v", err)
		}
		digest, err := canonical.Digest(frame)
		if frame != nil {
			_ = frame.Release()
		}
		if err != nil {
			t.Fatalf("canonical frame digest: %v", err)
		}
		hashes = append(hashes, digest)
	}
	return hashes
}
