package runtime

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestTwoRuntimeScopesCloseIndependently(t *testing.T) {
	firstScope := store.NewScope()
	secondScope := store.NewScope()
	firstValue := store.NewValueStore(firstScope, 1)
	secondValue := store.NewValueStore(secondScope, 10)
	firstDerived := store.NewDerived(firstScope, func() int { return firstValue.Get() + 1 }, firstValue)
	secondDerived := store.NewDerived(secondScope, func() int { return secondValue.Get() + 1 }, secondValue)

	firstRoot := facet.NewFacet()
	secondRoot := facet.NewFacet()
	firstRuntime := mustRuntimeTreeWithScope(t, firstScope, &firstRoot)
	secondRuntime := mustRuntimeTreeWithScope(t, secondScope, &secondRoot)
	firstRuntime.RunOneFrame()
	secondRuntime.RunOneFrame()
	if firstDerived.Get() != 2 || secondDerived.Get() != 11 {
		t.Fatal("runtime scopes did not retain their own initial derived values")
	}
	firstScope.FlushSignals()
	secondScope.FlushSignals()

	firstSignal, secondSignal := 0, 0
	firstScope.QueueSignal(func() { firstSignal++ })
	secondScope.QueueSignal(func() { secondSignal++ })
	firstRuntime.Shutdown()
	if firstSignal != 0 {
		t.Fatalf("closed runtime delivered %d queued signals", firstSignal)
	}
	if secondScope.FlushSignals() != 1 || secondSignal != 1 {
		t.Fatalf("surviving runtime signal state = %d, want one delivery", secondSignal)
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("closed runtime store remained readable")
			}
		}()
		firstValue.Get()
	}()
	secondValue.Set(20)
	secondScope.FlushSignals()
	secondScope.FlushDerived()
	secondRuntime.RunOneFrame()
	if got := secondDerived.Get(); got != 21 {
		t.Fatalf("surviving runtime derived = %d, want 21", got)
	}
}

func TestRuntimeRejectsFacetScopeOutsideApplication(t *testing.T) {
	applicationScope := store.NewScope()
	t.Cleanup(applicationScope.Close)
	foreignParent := store.NewScope()
	t.Cleanup(foreignParent.Close)
	root := facet.NewFacet()
	root.OwnStoreScope(foreignParent.Child())
	config := testConfigForScope(t, applicationScope)
	config.LayerRegistry = testLayerRegistry(t)

	_, err := New(config, nil, nil, &backendFixture{}, &root)
	if err == nil || !strings.Contains(err.Error(), "outside the configured application scope") {
		t.Fatalf("New error = %v, want foreign reactive scope rejection", err)
	}
	// Rebinding to a fresh identity would panic if failed construction had
	// already claimed the application's scope.
	applicationScope.Bind(store.NewRuntimeIdentity())
}
