package runtime

import (
	"fmt"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
)

func liveIndexTree(size int) (*facet.Facet, []*facet.Facet) {
	root := facet.NewFacet()
	nodes := make([]*facet.Facet, size)
	for i := range nodes {
		node := facet.NewFacet()
		nodes[i] = &node
		root.AddChild(nodes[i])
	}
	return &root, nodes
}

func TestIdentityLookupNoTraversalOrAllocation(t *testing.T) {
	root, nodes := liveIndexTree(2048)
	index := newLiveIndex()
	index.registerTree(root)
	visits := index.treeVisits

	lookup := func() {
		if _, _, ok := index.lookup(nodes[1024].ID()); !ok {
			t.Fatal("existing facet was not found")
		}
		if _, _, ok := index.lookup(facet.FacetID(^uint64(0))); ok {
			t.Fatal("missing facet resolved")
		}
	}
	if allocations := testing.AllocsPerRun(1000, lookup); allocations != 0 {
		t.Fatalf("identity lookup allocated %.2f objects per run", allocations)
	}
	if index.treeVisits != visits {
		t.Fatalf("identity lookup traversed tree: visits %d -> %d", visits, index.treeVisits)
	}
}

func TestLiveIndexReplacementGeneration(t *testing.T) {
	root, nodes := liveIndexTree(1)
	index := newLiveIndex()
	index.registerTree(root)
	oldImpl, oldHandle, ok := index.lookup(nodes[0].ID())
	if !ok || oldImpl == nil {
		t.Fatal("initial facet missing")
	}
	index.removeSubtree(nodes[0].ID())
	if _, ok := index.resolve(oldHandle); ok {
		t.Fatal("removed lifetime still resolves")
	}
	replacementValue := facet.NewFacet()
	replacement := &replacementValue
	index.register(replacement, root.ID(), true)
	index.link(root.ID(), replacement.ID())
	_, newHandle, ok := index.lookup(replacement.ID())
	if !ok || newHandle.generation == oldHandle.generation {
		t.Fatalf("replacement lifetime reused generation: old=%d new=%d", oldHandle.generation, newHandle.generation)
	}
	if _, ok := index.resolve(oldHandle); ok {
		t.Fatal("stale handle resolved replacement")
	}
}

func TestLiveIndexHostedParity(t *testing.T) {
	root, nodes := liveIndexTree(1)
	index := newLiveIndex()
	index.registerTree(root)
	child := facet.NewFacet()
	index.registerHosted(root.ID(), []facet.FacetImpl{&child})
	if got, _, ok := index.lookup(child.ID()); !ok || got.Base() != &child {
		t.Fatal("hosted child was not registered")
	}
	index.registerHosted(root.ID(), nil)
	if _, _, ok := index.lookup(child.ID()); ok {
		t.Fatal("removed hosted child remained live")
	}
	// Canonical tree children are already owned by the tree and must not be
	// removed when an overlapping hosted declaration changes.
	index.registerHosted(root.ID(), []facet.FacetImpl{nodes[0]})
	index.registerHosted(root.ID(), nil)
	if _, _, ok := index.lookup(nodes[0].ID()); !ok {
		t.Fatal("hosted refresh removed a tree-owned child")
	}
}

func TestLiveIndexCapacityFollowsLiveNodes(t *testing.T) {
	root := facet.NewFacet()
	index := newLiveIndex()
	index.registerTree(&root)
	children := make([]facet.Facet, 2048)
	for i := range children {
		children[i] = facet.NewFacet()
		root.AddChild(&children[i])
	}
	index.registerTree(&root)
	for i := range children {
		index.removeSubtree(children[i].ID())
		root.RemoveChild(&children[i])
	}
	for id := range index.byID {
		if id != root.ID() {
			t.Fatalf("removed facet %d remained in live directory", id)
		}
	}
	if cap(index.slots[0].children) > 8 {
		t.Fatalf("high-water child storage was retained by directory: %d", cap(index.slots[0].children))
	}
	for cycle := 0; cycle < 1000; cycle++ {
		child := facet.NewFacet()
		root.AddChild(&child)
		index.registerTree(&child)
		index.removeSubtree(child.ID())
		root.RemoveChild(&child)
	}
	if len(index.slots) != 1 || index.slots[0].id != root.ID() {
		t.Fatalf("live slots did not return to root: %+v", index.slots)
	}
	if cap(index.slots) > 8 || index.directoryCapacity > 8 || len(index.byID) != 1 {
		t.Fatalf("directory retained historical capacity: slots=%d directory=%d entries=%d", cap(index.slots), index.directoryCapacity, len(index.byID))
	}
}

func TestLiveIndexRejectsDuplicateHostedIdentity(t *testing.T) {
	root, _ := liveIndexTree(0)
	child := facet.NewFacet()
	index := newLiveIndex()
	index.registerTree(root)
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("duplicate hosted identity was accepted")
		}
	}()
	index.registerHosted(root.ID(), []facet.FacetImpl{&child, &child})
}

func TestRuntimeAddRemoveFacetUpdatesLiveIdentity(t *testing.T) {
	rootValue := facet.NewFacet()
	root := &rootValue
	rt := mustRuntimeTree(t, root)
	rt.attachTree(root)
	childValue := facet.NewFacet()
	child := &childValue
	if rt.FacetByID(child.ID()) != nil {
		t.Fatal("created child unexpectedly resolved before attach")
	}
	rt.AddFacet(root, child, facet.Attachment{})
	if got := rt.FacetByID(child.ID()); got == nil || got.Base() != child {
		t.Fatal("attached child was not published to live identity directory")
	}
	rt.RemoveFacet(child)
	if got := rt.FacetByID(child.ID()); got != nil {
		t.Fatal("disposed child remained in live identity directory")
	}
}

type rejectedAttachFacet struct{ facet.Facet }

func (f *rejectedAttachFacet) Base() *facet.Facet {
	f.BindImpl(f)
	return &f.Facet
}
func (*rejectedAttachFacet) OnAttach(facet.AttachContext) { panic("intentional attach rejection") }
func (*rejectedAttachFacet) OnDetach()                    {}
func (*rejectedAttachFacet) OnActivate()                  {}
func (*rejectedAttachFacet) OnDeactivate()                {}

func TestRuntimeFailedAttachLeavesNoLiveIdentity(t *testing.T) {
	rootValue := facet.NewFacet()
	root := &rootValue
	rt := mustRuntimeTree(t, root)
	rt.attachTree(root)
	child := &rejectedAttachFacet{Facet: facet.NewFacet()}
	childID := child.Base().ID()
	func() {
		defer func() {
			if recovered := recover(); recovered == nil {
				t.Fatal("attach rejection did not propagate")
			}
		}()
		rt.AddFacet(root, child, facet.Attachment{})
	}()
	if rt.FacetByID(childID) != nil {
		t.Fatal("failed attach published a live identity")
	}
	if child.Base().Parent() != nil || child.Base().State() != facet.StateDisposed {
		t.Fatalf("failed attach was not unwound: parent=%v state=%v", child.Base().Parent(), child.Base().State())
	}
}

type runtimeHostedIndexFixture struct {
	facet.Facet
	children []facet.FacetImpl
	publish  func(facet.FacetID, []facet.FacetImpl)
}

func (f *runtimeHostedIndexFixture) Base() *facet.Facet {
	f.BindImpl(f)
	return &f.Facet
}
func (f *runtimeHostedIndexFixture) OnAttach(ctx facet.AttachContext) {
	f.publish = ctx.OnHostedChildrenChanged
}
func (*runtimeHostedIndexFixture) OnDetach()                           {}
func (*runtimeHostedIndexFixture) OnActivate()                         {}
func (*runtimeHostedIndexFixture) OnDeactivate()                       {}
func (f *runtimeHostedIndexFixture) HostedChildren() []facet.FacetImpl { return f.children }

func TestRuntimeCanonicalHostedChildLifecycle(t *testing.T) {
	childValue := facet.NewFacet()
	host := &runtimeHostedIndexFixture{Facet: facet.NewFacet(), children: []facet.FacetImpl{&childValue}}
	rt := mustRuntimeTree(t, host)
	rt.attachTree(host)
	if rt.FacetByID(childValue.ID()) == nil {
		t.Fatal("canonical hosted-child probe did not publish initial child")
	}
	if host.publish == nil {
		t.Fatal("host was not given the lifecycle publisher")
	}
	host.children = nil
	host.publish(host.Base().ID(), nil)
	if rt.FacetByID(childValue.ID()) != nil {
		t.Fatal("removed hosted child remained live")
	}
}

func BenchmarkIdentityLookupSweep(b *testing.B) {
	for _, size := range []int{512, 2048} {
		b.Run(fmt.Sprintf("nodes-%d", size), func(b *testing.B) {
			root, nodes := liveIndexTree(size)
			index := newLiveIndex()
			index.registerTree(root)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, node := range nodes {
					if _, _, ok := index.lookup(node.ID()); !ok {
						b.Fatal("registered node missing")
					}
				}
				if _, _, ok := index.lookup(facet.FacetID(^uint64(0))); ok {
					b.Fatal("missing identity resolved")
				}
			}
		})
	}
}
