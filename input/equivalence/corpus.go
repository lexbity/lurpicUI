// Package equivalence is the differential honesty harness (QP-14 / FR-23):
// every mutating LDP verb is paired with a testkit-driven GUI gesture on the
// same fixture, and the assertion is that both produce identical store-version
// trajectories, identical dirty-facet sets, and identical frame packet hashes
// over the settle window. LDP and testkit share NO input code — equivalence
// is enforced by measurement, not construction.
package equivalence

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/canonical"
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
	// CompareCommandBoundary checks application state after the GUI command
	// dispatcher boundary. Pointer hover/press visuals are outside a command's
	// semantic twin.
	Compare CompareScope
}

// CompareScope scopes AssertEquivalent's assertion.
type CompareScope uint8

const (
	// CompareAll asserts packets, dirty sets, and store versions.
	CompareAll CompareScope = iota
	// CompareCommandBoundary asserts the command's application-state trajectory.
	CompareCommandBoundary
)

// Fixture is a booted mark tree with named stores registered for version
// capture.
type Fixture struct {
	Harness *testkit.DriveHarness
	// names/stores are the version-captured named stores, sorted by name.
	names        []string
	stores       []versionedStore
	facetTestIDs []string
}

// CaptureVersions declares the fixture's stores for trajectory comparison.
// These private test observations do not register protocol capabilities.
func (f *Fixture) CaptureVersions(t testing.TB, pairs map[string]versionedStore) {
	t.Helper()
	for name := range pairs {
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
	Frames []FrameObservation
}

// FrameObservation is one completed frame's independent semantic and pixel
// evidence, captured synchronously before later input can change the surface.
type FrameObservation struct {
	Number   uint64
	Packet   string
	Surface  string
	Dirty    string
	Versions []uint64
}

// observe installs complete frame capture before the first interaction atom.
func (f *Fixture) observe(t testing.TB, tr *Trajectory) func() error {
	t.Helper()
	var captureErr error
	f.Harness.SetFrameObserver(func(rt *runtime.Runtime, surface *testkit.MemorySurface) {
		if captureErr != nil {
			return
		}
		frame := rt.LastPrimaryFrame()
		if frame == nil {
			// No submission leaves the previous complete surface in place. The
			// empty packet distinguishes that state from a newly submitted one;
			// the independently captured surface remains authoritative pixels.
			frame = &render.Frame{}
		}
		packet, err := canonical.Digest(frame)
		if err != nil {
			captureErr = fmt.Errorf("canonical frame %d: %w", rt.LastFrameStats().FrameNumber, err)
			return
		}
		pixels := surface.Capture()
		versions := make([]uint64, 0, len(f.stores))
		for _, store := range f.stores {
			versions = append(versions, uint64(store.Version()))
		}
		dirty, err := dirtyDigest(rt, f.facetTestIDs)
		if err != nil {
			captureErr = err
			return
		}
		tr.Frames = append(tr.Frames, FrameObservation{
			Number:   rt.LastFrameStats().FrameNumber,
			Packet:   packet,
			Surface:  fmt.Sprintf("%x", sha256.Sum256(pixels.Pix)),
			Dirty:    dirty,
			Versions: versions,
		})
	})
	return func() error { return captureErr }
}

// dirtyDigest folds the frame's dirty-facet set into one digest (the same
// set the runtime re-lays-out).
func dirtyDigest(rt *runtime.Runtime, stableIDs []string) (string, error) {
	snap := rt.LastDirtySnapshot()
	type dirtyEntry struct {
		id    string
		flags uint64
	}
	entries := make([]dirtyEntry, 0, len(snap))
	for _, testID := range stableIDs {
		id := rt.FacetIDForTestID(testID)
		if id == 0 {
			return "", fmt.Errorf("stable fixture identity %q is not live", testID)
		}
		if flags := snap[id]; flags != 0 {
			entries = append(entries, dirtyEntry{id: testID, flags: uint64(flags)})
		}
	}
	if len(entries) != len(snap) {
		return "", fmt.Errorf("dirty snapshot has %d facets but only %d stable fixture identities", len(snap), len(entries))
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	h := sha256.New()
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(len(entries)))
	_, _ = h.Write(buf[:])
	for _, entry := range entries {
		binary.BigEndian.PutUint64(buf[:], uint64(len(entry.id)))
		_, _ = h.Write(buf[:])
		_, _ = h.Write([]byte(entry.id))
		binary.BigEndian.PutUint64(buf[:], entry.flags)
		_, _ = h.Write(buf[:])
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// AssertEquivalent runs the pair twice — LDP verb path and testkit GUI path
// — on fresh fixtures and asserts identical trajectories (FR-23). The TB
// parameter is the full testing.TB so meta-tests (the negative control) can
// pass a recording implementation that captures Fatalf instead of failing.
func AssertEquivalent(t testing.TB, p Pair) {
	t.Helper()

	runPath := func(viaDrive bool) Trajectory {
		fix := p.Fixture(t)
		trajectory := Trajectory{}
		checkCapture := fix.observe(t, &trajectory)
		if viaDrive {
			if perr := p.Verb(fix.Harness); perr != nil {
				t.Fatalf("verb %s failed: %v", p.Name, perr)
			}
		} else {
			p.GUI(fix.Harness.Harness)
		}
		for i := 0; i < settleFrames; i++ {
			fix.Harness.RunFrame()
		}
		if err := checkCapture(); err != nil {
			t.Fatalf("%s trajectory capture: %v", p.Name, err)
		}
		return trajectory
	}

	drive := runPath(true)
	gui := runPath(false)

	if p.Compare != CompareCommandBoundary && len(drive.Frames) != len(gui.Frames) {
		t.Fatalf("trajectory frame counts differ: %d vs %d", len(drive.Frames), len(gui.Frames))
		return
	}
	if p.Compare == CompareCommandBoundary {
		if len(drive.Frames) == 0 || len(gui.Frames) == 0 {
			t.Fatalf("command boundary produced no observed frames: %d/%d", len(drive.Frames), len(gui.Frames))
			return
		}
		driveFinal := drive.Frames[len(drive.Frames)-1].Versions
		guiFinal := gui.Frames[len(gui.Frames)-1].Versions
		if len(driveFinal) != len(guiFinal) {
			t.Fatalf("command state shapes differ: %d vs %d", len(driveFinal), len(guiFinal))
			return
		}
		for i := range driveFinal {
			if driveFinal[i] != guiFinal[i] {
				t.Fatalf("%s: command application state diverged (store %d): %d vs %d", p.Name, i, driveFinal[i], guiFinal[i])
			}
		}
		return
	}
	for i := range drive.Frames {
		if drive.Frames[i].Number != gui.Frames[i].Number {
			t.Fatalf("%s: frame number diverged at interaction frame %d: %d vs %d", p.Name, i, drive.Frames[i].Number, gui.Frames[i].Number)
		}
		if drive.Frames[i].Packet != gui.Frames[i].Packet {
			t.Fatalf("%s: canonical packet diverged at interaction frame %d: %s vs %s", p.Name, i, drive.Frames[i].Packet, gui.Frames[i].Packet)
		}
		if drive.Frames[i].Surface != gui.Frames[i].Surface {
			t.Fatalf("%s: ready software pixels diverged at interaction frame %d: %s vs %s", p.Name, i, drive.Frames[i].Surface, gui.Frames[i].Surface)
		}
		if drive.Frames[i].Dirty != gui.Frames[i].Dirty {
			t.Fatalf("%s: sorted dirty-facet digest diverged at interaction frame %d: %s vs %s", p.Name, i, drive.Frames[i].Dirty, gui.Frames[i].Dirty)
		}
		for j := range drive.Frames[i].Versions {
			if drive.Frames[i].Versions[j] != gui.Frames[i].Versions[j] {
				t.Fatalf("%s: store version diverged at interaction frame %d (store %d): %d vs %d",
					p.Name, i, j, drive.Frames[i].Versions[j], gui.Frames[i].Versions[j])
			}
		}
	}
}
