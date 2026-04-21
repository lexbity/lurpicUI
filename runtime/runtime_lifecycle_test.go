package runtime

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestRuntime_start_registers_thread(t *testing.T) {
	rt := mustRuntime(t)
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !syncutil.OnRuntimeThread() {
		t.Fatal("expected runtime thread")
	}
	rt.Shutdown()
}

func TestRuntime_shutdown_clean(t *testing.T) {
	rt := mustRuntime(t)
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.Shutdown()
}

func TestRuntime_phase1TickHooks_areScoped_and_cleared_on_shutdown(t *testing.T) {
	rt := mustRuntime(t)
	var count int
	unregister := rt.RegisterPhase1TickHook(func(time.Duration) {
		count++
	})
	rt.runPhase1TickHooks(time.Second)
	if count != 1 {
		t.Fatalf("count before shutdown = %d", count)
	}
	rt.shutdown()
	rt.runPhase1TickHooks(time.Second)
	if count != 1 {
		t.Fatalf("count after shutdown = %d", count)
	}
	unregister()
}

func TestRuntime_shutdownHooks_are_invoked_and_cleared(t *testing.T) {
	rt := mustRuntime(t)
	var count int
	unregister := rt.RegisterShutdownHook(func() {
		count++
	})
	rt.runShutdownHooks()
	if count != 1 {
		t.Fatalf("count before clear = %d", count)
	}
	unregister()
	rt.runShutdownHooks()
	if count != 1 {
		t.Fatalf("count after clear = %d", count)
	}
}

func TestRuntime_shutdown_disposes_tree_bottomup(t *testing.T) {
	order := []string{}
	root := &runtimeTestFacet{Facet: facet.NewFacet(), name: "root", detachOrder: &order}
	rt := mustRuntimeTree(t, root)
	rt.disposeTree(root)
	if len(order) != 1 || order[0] != "root" {
		t.Fatalf("order = %#v", order)
	}
}

func TestRuntime_attachtree_calls_onattach(t *testing.T) {
	root := &runtimeTestFacet{Facet: facet.NewFacet(), name: "root"}
	rt := mustRuntimeTree(t, root)
	rt.attachTree(root)
	if root.attachCount != 1 {
		t.Fatalf("attach count = %d", root.attachCount)
	}
}

func TestRuntime_subscribe_builder_integrates_with_lifecycle(t *testing.T) {
	s := store.NewValueStore(1)
	root := newRuntimeSubscriptionFacet(s)
	rt := mustRuntimeTree(t, root)
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	if got := root.Subs().Len(); got != 1 {
		t.Fatalf("subscriptions after attach = %d", got)
	}
	if got := root.SubscribedVersions(); len(got) != 1 || got[0] != 0 {
		t.Fatalf("subscribed versions after attach = %#v", got)
	}

	s.Set(2)
	rt.RunOneFrame()

	if root.changeCount != 1 {
		t.Fatalf("changeCount = %d", root.changeCount)
	}
	if got := root.SubscribedVersions(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("subscribed versions after update = %#v", got)
	}

	rt.Shutdown()
	if s.OnChange.HasSubscribers() {
		t.Fatal("expected subscriptions to be released on shutdown")
	}
}

func TestRuntime_activatetree_calls_onactivate(t *testing.T) {
	root := &runtimeTestFacet{Facet: facet.NewFacet(), name: "root"}
	rt := mustRuntimeTree(t, root)
	rt.attachTree(root)
	rt.activateTree(root)
	if root.activateCount != 1 {
		t.Fatalf("activate count = %d", root.activateCount)
	}
}

func TestRuntime_marktreedirty_sets_flags(t *testing.T) {
	root, child, leaf := newRuntimeTestTree()
	rt := mustRuntimeTree(t, root)
	rt.markTreeDirty(root, facet.DirtyAll)
	if root.DirtyFlags() != facet.DirtyAll || child.DirtyFlags() != facet.DirtyAll || leaf.DirtyFlags() != facet.DirtyAll {
		t.Fatalf("dirty flags = %#v %#v %#v", root.DirtyFlags(), child.DirtyFlags(), leaf.DirtyFlags())
	}
}
