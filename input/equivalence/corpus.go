// Package equivalence is the differential honesty harness (QP-14 / FR-23):
// every mutating LDP verb is paired with a testkit-driven GUI gesture on the
// same fixture, and the assertion is that both produce identical store-version
// trajectories, identical dirty-facet sets, and identical frame packet hashes
// over the settle window. LDP and testkit share NO input code — equivalence
// is enforced by measurement, not construction.
package equivalence

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/store"
)

// versionedStore is the trajectory-capture view of a store (all three store
// shapes expose Version() store.Version).
type versionedStore interface {
	Version() store.Version
}

// settleFrames is the post-interaction observation window: the comparable
// region after both paths completed their interaction (the drive verb
// settles internally; the GUI gesture runs its own frames).
const settleFrames = 8

// Pair is one verb's differential corpus entry.
type Pair struct {
	// Name is the corpus pair name (the registry's Twin reference,
	// e.g. "input.click/button").
	Name string
	// Fixture boots a fresh mark tree (with named stores registered) per run.
	Fixture func(t testing.TB) *Fixture
	// Verb runs the LDP path through the harness's drive sink. A non-nil
	// *proto.Error fails the assertion.
	Verb func(h *testkit.DriveHarness) *proto.Error
	// GUI runs the testkit gesture path (warmup + injected events + frames).
	GUI func(h *testkit.Harness)
	// GUIHelper names the testkit gesture (corpus documentation).
	GUIHelper string
	// Compare scopes the assertion. CompareAll (default) asserts packet
	// hashes, dirty digests, and store versions — the input.* verbs' claim.
	// CompareStores asserts only store-version trajectories: the sanctioned
	// scope for store.write's INDIRECT twin (registry: "yes (indirect)") —
	// a GUI gesture carries pointer artifacts (hover, press visuals) a
	// direct store write legitimately lacks.
	Compare CompareScope
}

// CompareScope scopes AssertEquivalent's assertion.
type CompareScope uint8

const (
	// CompareAll asserts packets, dirty sets, and store versions.
	CompareAll CompareScope = iota
	// CompareStores asserts only store versions.
	CompareStores
)

// Fixture is a booted mark tree with named stores registered for version
// capture.
type Fixture struct {
	Harness *testkit.DriveHarness
	// names/stores are the version-captured named stores, sorted by name.
	names  []string
	stores []versionedStore
}

// RegisterNames binds the fixture's stores for trajectory capture (the same
// names both paths observe).
func (f *Fixture) RegisterNames(t testing.TB, pairs map[string]versionedStore) {
	t.Helper()
	for name, s := range pairs {
		if err := f.Harness.Runtime().RegisterNamedStore(name, s); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
		f.names = append(f.names, name)
	}
	// insertion sort for deterministic capture order
	for i := 1; i < len(f.names); i++ {
		for j := i; j > 0 && f.names[j] < f.names[j-1]; j-- {
			f.names[j], f.names[j-1] = f.names[j-1], f.names[j]
		}
	}
	for _, name := range f.names {
		f.stores = append(f.stores, pairs[name])
	}
}

// Trajectory is the per-cut observation stream: packet hash, dirty-facet
// digest, and named-store versions.
type Trajectory struct {
	Hashes   []uint64
	Dirty    []uint64
	Versions [][]uint64
}

// capture runs settleFrames frames and observes each cut.
func (f *Fixture) capture(settleFrames int) Trajectory {
	rt := f.Harness.Runtime()
	tr := Trajectory{}
	for i := 0; i < settleFrames; i++ {
		f.Harness.RunFrame()
		tr.Hashes = append(tr.Hashes, rt.FramePacketHash())
		tr.Dirty = append(tr.Dirty, dirtyDigest(rt))
		versions := make([]uint64, 0, len(f.stores))
		for _, s := range f.stores {
			versions = append(versions, uint64(s.Version()))
		}
		tr.Versions = append(tr.Versions, versions)
	}
	return tr
}

// dirtyDigest folds the frame's dirty-facet set into one digest (the same
// set the runtime re-lays-out).
func dirtyDigest(rt *runtime.Runtime) uint64 {
	snap := rt.LastDirtySnapshot()
	h := uint64(14695981039346656037)
	h = (h ^ uint64(len(snap))) * 1099511628211
	for id, flags := range snap {
		h = (h ^ uint64(id)) * 1099511628211
		h = (h ^ uint64(flags)) * 1099511628211
	}
	return h
}

// AssertEquivalent runs the pair twice — LDP verb path and testkit GUI path
// — on fresh fixtures and asserts identical trajectories (FR-23). The TB
// parameter is the full testing.TB so meta-tests (the negative control) can
// pass a recording implementation that captures Fatalf instead of failing.
func AssertEquivalent(t testing.TB, p Pair) {
	t.Helper()

	runPath := func(viaDrive bool) Trajectory {
		fix := p.Fixture(t)
		if viaDrive {
			if perr := p.Verb(fix.Harness); perr != nil {
				t.Fatalf("verb %s failed: %v", p.Name, perr)
			}
		} else {
			p.GUI(fix.Harness.Harness)
		}
		return fix.capture(settleFrames)
	}

	drive := runPath(true)
	gui := runPath(false)

	if len(drive.Hashes) != len(gui.Hashes) {
		t.Fatalf("trajectory lengths differ: %d vs %d", len(drive.Hashes), len(gui.Hashes))
	}
	for i := range drive.Hashes {
		if p.Compare == CompareStores {
			for j := range drive.Versions[i] {
				if drive.Versions[i][j] != gui.Versions[i][j] {
					t.Fatalf("%s: store version diverged at settle frame %d (store %d): %d vs %d",
						p.Name, i, j, drive.Versions[i][j], gui.Versions[i][j])
				}
			}
			continue
		}
		if drive.Hashes[i] != gui.Hashes[i] {
			t.Fatalf("%s: packet hash diverged at settle frame %d: %#x vs %#x", p.Name, i, drive.Hashes[i], gui.Hashes[i])
		}
		if drive.Dirty[i] != gui.Dirty[i] {
			t.Fatalf("%s: dirty-facet digest diverged at settle frame %d: %#x vs %#x", p.Name, i, drive.Dirty[i], gui.Dirty[i])
		}
		for j := range drive.Versions[i] {
			if drive.Versions[i][j] != gui.Versions[i][j] {
				t.Fatalf("%s: store version diverged at settle frame %d (store %d): %d vs %d",
					p.Name, i, j, drive.Versions[i][j], gui.Versions[i][j])
			}
		}
	}
}
