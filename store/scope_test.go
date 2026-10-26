package store

import "testing"

func TestScopeCloseUnsubscribesAncestorSources(t *testing.T) {
	parent := NewScope()
	defer parent.Close()
	child := parent.Child()
	source := NewValueStore(parent, 1)
	derived := NewDerived(child, func() int { return source.Get() }, source)
	if got := derived.Get(); got != 1 {
		t.Fatalf("initial value = %d, want 1", got)
	}
	var invalidated int
	derived.OnInvalidated.Subscribe(func(struct{}) { invalidated++ })

	source.Set(2)
	parent.FlushSignals()
	child.FlushSignals()
	if got := invalidated; got != 1 {
		t.Fatalf("invalidations before close = %d, want 1", got)
	}
	child.Close()
	source.Set(3)
	parent.FlushSignals()
	child.FlushSignals()
	if got := invalidated; got != 1 {
		t.Fatalf("closed scope received another invalidation: %d", got)
	}
	if got := len(parent.children); got != 0 {
		t.Fatalf("closed child remains in parent directory: %d entries", got)
	}
}

func TestScopeThousandReactiveChildCyclesReturnToBaseline(t *testing.T) {
	root := NewScope()
	defer root.Close()
	baselineChildren := len(root.children)
	for cycle := 0; cycle < 1000; cycle++ {
		child := root.Child()
		source := NewValueStore(child, cycle)
		derived := NewDerived(child, func() int { return source.Get() + 1 }, source)
		derived.Get()
		root.FlushSignals()
		source.Set(cycle + 1)
		root.FlushSignals()
		root.FlushDerived()
		child.Close()
		if len(root.children) != baselineChildren {
			t.Fatalf("cycle %d left %d child scopes at root, baseline %d", cycle, len(root.children), baselineChildren)
		}
		if len(child.resources) != 0 || len(child.dirty) != 0 || len(child.signals) != 0 {
			t.Fatalf("cycle %d retained child resources: resources=%d dirty=%d signals=%d", cycle, len(child.resources), len(child.dirty), len(child.signals))
		}
	}
}

func TestScopeCloseFencesQueuedSignals(t *testing.T) {
	scope := NewScope()
	called := false
	scope.QueueSignal(func() { called = true })
	scope.Close()
	if delivered := scope.FlushSignals(); delivered != 0 {
		t.Fatalf("delivered %d signals after close", delivered)
	}
	if called {
		t.Fatal("queued signal ran after close")
	}
}

func TestRootScopeFlushesDescendantSignalsAndDerivedWork(t *testing.T) {
	root := NewScope()
	defer root.Close()
	child := root.Child()
	value := NewValueStore(child, 1)
	computed := NewDerived(child, func() int { return value.Get() * 2 }, value)
	if got := computed.Get(); got != 2 {
		t.Fatalf("initial derived = %d, want 2", got)
	}
	root.FlushSignals()
	value.Set(3)
	if got := root.FlushSignals(); got == 0 {
		t.Fatal("root signal boundary did not deliver child notifications")
	}
	flush := root.FlushDerived()
	if flush.Evaluated != 1 || flush.Recomputed != 1 {
		t.Fatalf("root derived flush = %+v, want one child computation", flush)
	}
	if got := computed.Get(); got != 6 {
		t.Fatalf("child derived = %d, want 6", got)
	}
}

func TestClosedScopeRejectsReactiveValueUse(t *testing.T) {
	scope := NewScope()
	value := NewValueStore(scope, 1)
	scope.Close()
	assertPanics(t, "closed value read", func() { value.Get() })
	assertPanics(t, "closed value write", func() { value.Set(2) })
}

func TestScopeFlushVisitsOnlyDirtyDerivedGraph(t *testing.T) {
	scope := NewScope()
	defer scope.Close()
	source := NewValueStore(scope, 1)
	firstCalls, secondCalls, cleanCalls := 0, 0, 0
	first := NewDerived(scope, func() int { firstCalls++; return source.Get() + 1 }, source)
	second := NewDerived(scope, func() int { secondCalls++; return first.Get() + 1 }, first)
	clean := NewDerived(scope, func() int { cleanCalls++; return 99 })
	first.Get()
	second.Get()
	clean.Get()
	scope.FlushSignals()
	scope.ResetFrameDerivedCounters()

	source.Set(2)
	scope.FlushSignals()
	result := scope.FlushDerived()
	if result.Evaluated != 2 || result.Recomputed != 2 {
		t.Fatalf("flush result = %+v, want exactly the dirty two-node graph", result)
	}
	if firstCalls != 2 || secondCalls != 2 || cleanCalls != 1 {
		t.Fatalf("compute counts = %d/%d/%d, want 2/2/1", firstCalls, secondCalls, cleanCalls)
	}
	if got := second.Get(); got != 4 {
		t.Fatalf("dependent result = %d, want 4", got)
	}
}

func TestScopeRejectsForeignAndSiblingSources(t *testing.T) {
	root := NewScope()
	defer root.Close()
	left := root.Child()
	right := root.Child()
	foreign := NewValueStore(NewScope(), 1)
	defer foreign.OwnerScope().Close()
	sibling := NewValueStore(right, 2)

	assertPanics(t, "foreign source", func() {
		NewDerived(left, func() int { return foreign.Get() }, foreign)
	})
	assertPanics(t, "sibling source", func() {
		NewDerived(left, func() int { return sibling.Get() }, sibling)
	})
}

func TestScopeRejectsUndeclaredDerivedReads(t *testing.T) {
	root := NewScope()
	defer root.Close()
	owner := root.Child()
	sibling := root.Child()
	declared := NewValueStore(root, 1)
	var undeclared *Derived[int]
	computed := NewDerived(owner, func() int { return undeclared.Get() }, declared)
	undeclared = NewDerived(sibling, func() int { return declared.Get() }, declared)
	assertPanics(t, "undeclared cross-scope dependency", func() { computed.Get() })
}

func TestScopeSubscriptionSlotGeneration(t *testing.T) {
	var set invalidationSet
	oldCalls, newCalls := 0, 0
	old := set.add(func() { oldCalls++ })
	oldCallbacks := set.snapshot()
	old.Release()
	old.Release()
	current := set.add(func() { newCalls++ })
	if len(set.slots) != 1 {
		t.Fatalf("slot count = %d, want reused single slot", len(set.slots))
	}
	oldCallbacks[0]()
	for _, callback := range set.snapshot() {
		callback()
	}
	if oldCalls != 0 || newCalls != 1 {
		t.Fatalf("callback counts = %d/%d, want 0/1", oldCalls, newCalls)
	}
	current.Release()
}

func TestScopeBindsToOneRuntimeIdentity(t *testing.T) {
	scope := NewScope()
	defer scope.Close()
	scope.Bind(NewRuntimeIdentity())
	assertPanics(t, "second runtime binding", func() { scope.Bind(NewRuntimeIdentity()) })
}

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s did not panic", name)
		}
	}()
	fn()
}
