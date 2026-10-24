package equivalence

import (
	"encoding/json"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// TestEveryMutatingVerbHasATwin is the FR-23 gate (G6.4): enumerate the LDP
// method registry; every mutating method MUST name a corpus twin. "n/a" is
// reserved for the time-control trio (time.mode, time.advance, frame.run) —
// step infrastructure, not GUI-addressable mutations. Adding a mutating verb
// without a corpus pair fails this test by construction.
func TestEveryMutatingVerbHasATwin(t *testing.T) {
	pairs := map[string]bool{}
	for _, p := range corpusTest() {
		pairs[p.Name] = true
	}
	for _, spec := range proto.Registry {
		if !spec.Mutating {
			continue
		}
		switch {
		case spec.Twin == "":
			t.Errorf("mutating method %s has no Twin (FR-23: mutating verbs are proven equivalent, not assumed)", spec.Name)
		case spec.Twin == "n/a":
			if spec.Name != "time.mode" && spec.Name != "time.advance" && spec.Name != "frame.run" {
				t.Errorf("method %s claims the n/a twin — only time.mode/time.advance/frame.run are time control", spec.Name)
			}
		case !pairs[spec.Twin]:
			t.Errorf("mutating method %s names twin %q which is not in the corpus", spec.Name, spec.Twin)
		}
	}
	// Corpus pairs must all be referenced twins (no orphan pairs — a pair
	// the registry doesn't know about proves nothing).
	registryTwins := map[string]bool{}
	for _, spec := range proto.Registry {
		if spec.Twin != "" && spec.Twin != "n/a" {
			registryTwins[spec.Twin] = true
		}
	}
	for _, p := range corpusTest() {
		if !registryTwins[p.Name] {
			t.Errorf("corpus pair %q is not referenced by any registry Twin (orphan pair)", p.Name)
		}
	}
}

// TestRegistrySyncWithRuntimeTable keeps proto.Registry in sync with the
// runtime's dispatch table: every runtime method is in the registry and vice
// versa (the P8 generator consumes this registry as the source of truth).
func TestRegistrySyncWithRuntimeTable(t *testing.T) {
	fix := newButtonFixture(t)
	rt := fix.Harness.Runtime()
	driveSink := rt.DriveSink()
	if driveSink == nil {
		t.Fatal("drive sink not installed")
	}

	// The runtime's method list arrives via sys.info.
	payload := fix.Harness.CallOK("sys.info", nil)
	var info struct {
		Methods []string `json:"methods"`
	}
	if err := json.Unmarshal(payload, &info); err != nil {
		t.Fatalf("sys.info payload: %v", err)
	}

	runtimeMethods := map[string]bool{}
	for _, m := range info.Methods {
		runtimeMethods[m] = true
	}
	registryMethods := map[string]bool{}
	for _, spec := range proto.Registry {
		registryMethods[spec.Name] = true
	}
	// sys.info derives its list from proto.Registry, so the reported methods
	// must equal the registry exactly.
	for _, m := range info.Methods {
		if !registryMethods[m] {
			t.Errorf("runtime method %q is not in proto.Registry", m)
		}
		registryMethods[m] = false
	}
	for m := range registryMethods {
		if registryMethods[m] {
			t.Errorf("registry method %q is not reported by sys.info", m)
		}
	}
}
