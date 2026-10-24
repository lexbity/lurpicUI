package contracttest

import (
	"codeburg.org/lexbit/lurpicui/facet"
)

// AssertKeyedItem is the KeyedItem contract helper (LDP Phase 3): the mark
// implements marks.KeyedItem and its key is the host-assigned stable
// identity the locator walk addresses. wantKey empty means the mark legitimately
// carries no key until a host assigns one — the assertion is then only that
// the capability exists and never panics.
func AssertKeyedItem(t TB, build func() facet.FacetImpl, wantKey string) {
	t.Helper()
	m := build()
	ctx := facet.AttachContext{Runtime: NoopRuntime{}}
	facet.Attach(m, ctx)
	defer facet.Dispose(m)

	keyed, ok := m.(interface{ ItemKey() string })
	if !ok {
		t.Fatalf("AssertKeyedItem: mark does not implement marks.KeyedItem")
	}
	if wantKey != "" && keyed.ItemKey() != wantKey {
		t.Fatalf("AssertKeyedItem: ItemKey=%q, want %q", keyed.ItemKey(), wantKey)
	}
}

// AssertHostsChildren is the HostsChildren contract helper (LDP Phase 3):
// the group mark exposes its hosted (virtually-projected) children so the
// LDP locator walk reaches keyed rows the facet tree never attaches.
func AssertHostsChildren(t TB, build func() facet.FacetImpl, wantMin int) {
	t.Helper()
	m := build()
	ctx := facet.AttachContext{Runtime: NoopRuntime{}}
	facet.Attach(m, ctx)
	defer facet.Dispose(m)

	host, ok := m.(interface{ HostedChildren() []facet.FacetImpl })
	if !ok {
		t.Fatalf("AssertHostsChildren: mark does not implement marks.HostsChildren")
	}
	if got := len(host.HostedChildren()); got < wantMin {
		t.Fatalf("AssertHostsChildren: HostedChildren=%d, want >= %d", got, wantMin)
	}
}
