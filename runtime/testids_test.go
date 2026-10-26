package runtime

import (
	"fmt"
	"strings"

	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
)

func TestTestID_SetOnlyInCreated(t *testing.T) {
	root := buildBenchmarkTree()
	root.SetTestID("root")
	root.Base().SetTestID("root-2") // still Created
	if got := root.Base().TestID(); got != "root-2" {
		t.Fatalf("TestID = %q", got)
	}
	rt := mustRuntimeTree(t, root)
	defer rt.Shutdown()
	rt.RunOneFrame()
	func() {
		defer func() {
			r := recover()
			if r == nil || !strings.Contains(sprintfAny(r), "after attach") {
				t.Fatalf("SetTestID after attach must panic, got %v", r)
			}
		}()
		root.Base().SetTestID("late")
	}()
}

func sprintfAny(v any) string { return fmt.Sprint(v) }

func TestTestID_DuplicatePanicNamesBothFacets(t *testing.T) {
	// One runtime, one registry: two facets of the same tree claim the same
	// TestID — the attach transition must panic naming both facets (QP-7).
	root := buildBenchmarkTree()
	root.SetTestID("dup-parent")
	dup := buildBenchmarkTree()
	dup.SetTestID("dup-parent")
	rt := mustRuntimeTree(t, root)
	defer rt.Shutdown()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("duplicate live TestID must panic at attach")
		}
		msg := fmt.Sprint(r)
		if !strings.Contains(msg, "dup-parent") {
			t.Fatalf("panic %v must name the TestID", r)
		}
		if !strings.Contains(msg, "facet") {
			t.Fatalf("panic %v must name both facets", r)
		}
	}()
	rt.AddFacet(root, dup, facet.Attachment{})
	rt.RunOneFrame()
}

func TestTestID_ReleaseOnDispose(t *testing.T) {
	root := buildBenchmarkTree()
	root.SetTestID("reusable")
	rt := mustRuntimeTree(t, root)
	defer rt.Shutdown()
	rt.RunOneFrame()
	if got := rt.FacetIDForTestID("reusable"); got != root.Base().ID() {
		t.Fatalf("FacetIDForTestID = %d, want %d", got, root.Base().ID())
	}
	rt.Shutdown()
	if got := rt.FacetIDForTestID("reusable"); got != 0 {
		t.Fatalf("TestID still live after dispose: %d", got)
	}
}

func TestTestIDRegistriesAreRuntimeScoped(t *testing.T) {
	makeRuntime := func(t *testing.T) (*Runtime, *runtimeRenderFacet) {
		t.Helper()
		root := buildBenchmarkTree()
		root.SetTestID("shared.author.id")
		cfg := testConfig(t)
		cfg.LayerRegistry = testLayerRegistry(t)
		rt, err := New(cfg, nil, &testWindow{width: 100, height: 100}, &backendFixture{}, root)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(rt.Shutdown)
		rt.RunOneFrame()
		return rt, root
	}
	first, firstRoot := makeRuntime(t)
	second, secondRoot := makeRuntime(t)
	if first.FacetIDForTestID("shared.author.id") != firstRoot.Base().ID() {
		t.Fatalf("first runtime identity leaked: got %d want %d", first.FacetIDForTestID("shared.author.id"), firstRoot.Base().ID())
	}
	if second.FacetIDForTestID("shared.author.id") != secondRoot.Base().ID() {
		t.Fatalf("second runtime identity = %d want %d", second.FacetIDForTestID("shared.author.id"), secondRoot.Base().ID())
	}
	if firstRoot.Base().ID() == secondRoot.Base().ID() {
		t.Fatal("fixture facet identities must be independent")
	}
}
