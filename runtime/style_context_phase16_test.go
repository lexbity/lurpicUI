package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestRuntime_rootStyleContext_installed_and_dirty_on_change(t *testing.T) {
	root := facet.NewFacet()
	rt := mustRuntimeWithBackend(t, &root, &stubBackend{})

	store, ok := rt.RootStyleContext().(*theme.StyleContextStore)
	if !ok || store == nil {
		t.Fatalf("root style context = %#v", rt.RootStyleContext())
	}

	next := store.Get()
	next.Depth++
	store.Set(next)

	if got := rt.dirtyFacets[root.ID()] & facet.DirtyLayout; got == 0 {
		t.Fatalf("expected dirty layout after style change, dirty=%v", rt.dirtyFacets[root.ID()])
	}
	if got := rt.dirtyFacets[root.ID()] & facet.DirtyProjection; got == 0 {
		t.Fatalf("expected dirty projection after style change, dirty=%v", rt.dirtyFacets[root.ID()])
	}
}
