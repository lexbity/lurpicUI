package runtime

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
)

type liveSlot struct {
	id         facet.FacetID
	generation uint64
	impl       facet.FacetImpl
	parent     facet.FacetID
	children   []facet.FacetID
	treeOwned  bool
}

type liveHandle struct {
	id         facet.FacetID
	generation uint64
}

type pendingLiveFacet struct {
	impl   facet.FacetImpl
	parent facet.FacetID
}

// liveIndex is the runtime's sole FacetID directory. Slots are dense in live
// topology; FacetID high-water does not determine its resident capacity.
type liveIndex struct {
	byID              map[facet.FacetID]int
	slots             []liveSlot
	nextGeneration    uint64
	directoryCapacity int
	hosted            map[facet.FacetID]map[facet.FacetID]struct{}
	treeVisits        uint64
}

func newLiveIndex() *liveIndex {
	return &liveIndex{
		byID:              make(map[facet.FacetID]int),
		directoryCapacity: 8,
		hosted:            make(map[facet.FacetID]map[facet.FacetID]struct{}),
	}
}

func (index *liveIndex) lookup(id facet.FacetID) (facet.FacetImpl, liveHandle, bool) {
	if index == nil || id == 0 {
		return nil, liveHandle{}, false
	}
	slotIndex, ok := index.byID[id]
	if !ok {
		return nil, liveHandle{}, false
	}
	slot := &index.slots[slotIndex]
	return slot.impl, liveHandle{id: slot.id, generation: slot.generation}, true
}

func (index *liveIndex) contains(id facet.FacetID) bool {
	if index == nil || id == 0 {
		return false
	}
	_, ok := index.byID[id]
	return ok
}

func (index *liveIndex) resolve(handle liveHandle) (facet.FacetImpl, bool) {
	if index == nil || handle.id == 0 || handle.generation == 0 {
		return nil, false
	}
	slotIndex, ok := index.byID[handle.id]
	if !ok {
		return nil, false
	}
	slot := &index.slots[slotIndex]
	if slot.generation != handle.generation {
		return nil, false
	}
	return slot.impl, true
}

func (index *liveIndex) parent(id facet.FacetID) facet.FacetID {
	if index == nil {
		return 0
	}
	slotIndex, ok := index.byID[id]
	if !ok {
		return 0
	}
	return index.slots[slotIndex].parent
}

func (index *liveIndex) registerTree(root facet.FacetImpl) {
	if root == nil || root.Base() == nil {
		return
	}
	rootParent := facet.FacetID(0)
	if parent := root.Base().Parent(); parent != nil {
		rootParent = parent.ID()
	}
	ordered := collectLiveSubtree(root, rootParent)
	index.treeVisits += uint64(len(ordered))
	index.validateRegistrations(ordered)
	for _, current := range ordered {
		index.register(current.impl, current.parent, true)
		index.link(current.parent, current.impl.Base().ID())
	}
}

func collectLiveSubtree(root facet.FacetImpl, parent facet.FacetID) []pendingLiveFacet {
	stack := []pendingLiveFacet{{impl: root, parent: parent}}
	seen := make(map[facet.FacetID]struct{})
	ordered := make([]pendingLiveFacet, 0, 8)
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack[last] = pendingLiveFacet{}
		stack = stack[:last]
		if current.impl == nil || current.impl.Base() == nil {
			continue
		}
		base := current.impl.Base()
		if base.State() == facet.StateDisposed {
			continue
		}
		id := base.ID()
		if id == 0 {
			panic("runtime: live facet identity cannot be zero")
		}
		if _, exists := seen[id]; exists {
			panic(fmt.Sprintf("runtime: duplicate live facet identity %d in subtree", id))
		}
		seen[id] = struct{}{}
		ordered = append(ordered, current)
		base.VisitChildren(func(child facet.FacetImpl) bool {
			stack = append(stack, pendingLiveFacet{impl: child, parent: id})
			return true
		})
	}
	return ordered
}

func (index *liveIndex) validateRegistrations(entries []pendingLiveFacet) {
	known := make(map[facet.FacetID]struct{}, len(entries))
	for _, current := range entries {
		id := current.impl.Base().ID()
		if _, duplicate := known[id]; duplicate {
			panic(fmt.Sprintf("runtime: duplicate live facet identity %d in registration", id))
		}
		known[id] = struct{}{}
	}
	for _, current := range entries {
		id := current.impl.Base().ID()
		if slotIndex, exists := index.byID[id]; exists {
			slot := &index.slots[slotIndex]
			if slot.impl.Base() != current.impl.Base() || slot.parent != current.parent {
				panic(fmt.Sprintf("runtime: live facet identity %d has conflicting owners", id))
			}
		}
		if _, parentIncluded := known[current.parent]; current.parent != 0 && !index.contains(current.parent) && !parentIncluded {
			panic(fmt.Sprintf("runtime: live facet parent %d is unavailable", current.parent))
		}
	}
}

func (index *liveIndex) validateNewTree(root facet.FacetImpl) error {
	if root == nil || root.Base() == nil {
		return fmt.Errorf("runtime: cannot attach a facet without an implementation and identity")
	}
	stack := []facet.FacetImpl{root}
	seen := make(map[facet.FacetID]struct{})
	for len(stack) > 0 {
		last := len(stack) - 1
		impl := stack[last]
		stack[last] = nil
		stack = stack[:last]
		if impl == nil || impl.Base() == nil {
			continue
		}
		base := impl.Base()
		id := base.ID()
		if id == 0 || base.State() != facet.StateCreated {
			return fmt.Errorf("runtime: facet %d is not a created identity", id)
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("runtime: duplicate facet identity %d in attached subtree", id)
		}
		if index.contains(id) {
			return fmt.Errorf("runtime: facet identity %d is already live", id)
		}
		seen[id] = struct{}{}
		base.VisitChildren(func(child facet.FacetImpl) bool {
			stack = append(stack, child)
			return true
		})
	}
	return nil
}

func (index *liveIndex) registerHosted(parent facet.FacetID, children []facet.FacetImpl) {
	if index == nil || parent == 0 {
		return
	}
	if _, ok := index.byID[parent]; !ok {
		panic(fmt.Sprintf("runtime: hosted children owner %d is not live", parent))
	}
	newChildren := make(map[facet.FacetID]struct{}, len(children))
	entries := make([]pendingLiveFacet, 0, len(children))
	for _, child := range children {
		if child == nil || child.Base() == nil || child.Base().State() == facet.StateDisposed {
			continue
		}
		id := child.Base().ID()
		if actualParent := child.Base().Parent(); actualParent != nil && actualParent.ID() != parent {
			panic(fmt.Sprintf("runtime: hosted child %d belongs to facet %d, not host %d", id, actualParent.ID(), parent))
		}
		if _, duplicate := newChildren[id]; duplicate {
			panic(fmt.Sprintf("runtime: hosted child %d is declared more than once by %d", id, parent))
		}
		newChildren[id] = struct{}{}
		entries = append(entries, collectLiveSubtree(child, parent)...)
	}
	index.treeVisits += uint64(len(entries))
	index.validateRegistrations(entries)
	oldChildren := index.hosted[parent]
	for id := range oldChildren {
		if _, remains := newChildren[id]; remains {
			continue
		}
		if slotIndex, live := index.byID[id]; live && !index.slots[slotIndex].treeOwned {
			index.removeSubtree(id)
		}
	}
	for _, current := range entries {
		index.register(current.impl, current.parent, false)
		index.link(current.parent, current.impl.Base().ID())
	}
	if len(newChildren) == 0 {
		delete(index.hosted, parent)
		return
	}
	index.hosted[parent] = newChildren
}

func (index *liveIndex) register(impl facet.FacetImpl, parent facet.FacetID, treeOwned bool) {
	if impl == nil || impl.Base() == nil || impl.Base().ID() == 0 {
		panic("runtime: cannot register a live facet without an implementation and identity")
	}
	base := impl.Base()
	if slotIndex, exists := index.byID[base.ID()]; exists {
		slot := &index.slots[slotIndex]
		if slot.impl.Base() != base || slot.parent != parent {
			panic(fmt.Sprintf("runtime: live facet identity %d has conflicting owners", base.ID()))
		}
		slot.impl = impl
		slot.treeOwned = slot.treeOwned || treeOwned
		return
	}
	index.nextGeneration++
	if index.nextGeneration == 0 {
		panic("runtime: live facet generation exhausted")
	}
	slot := liveSlot{id: base.ID(), generation: index.nextGeneration, impl: impl, parent: parent, treeOwned: treeOwned}
	slotIndex := len(index.slots)
	index.slots = append(index.slots, slot)
	index.byID[slot.id] = slotIndex
	if len(index.slots) > index.directoryCapacity {
		index.directoryCapacity = len(index.slots) * 2
	}
}

func (index *liveIndex) link(parent, child facet.FacetID) {
	if parent == 0 || child == 0 {
		return
	}
	parentIndex, parentLive := index.byID[parent]
	_, childLive := index.byID[child]
	if !parentLive || !childLive {
		panic("runtime: cannot link a non-live facet")
	}
	children := index.slots[parentIndex].children
	for _, existing := range children {
		if existing == child {
			return
		}
	}
	index.slots[parentIndex].children = append(children, child)
}

func (index *liveIndex) removeSubtree(id facet.FacetID) {
	if index == nil {
		return
	}
	slotIndex, exists := index.byID[id]
	if !exists {
		return
	}
	children := append([]facet.FacetID(nil), index.slots[slotIndex].children...)
	for _, child := range children {
		index.removeSubtree(child)
	}
	slotIndex, exists = index.byID[id]
	if !exists {
		return
	}
	slot := index.slots[slotIndex]
	if slot.parent != 0 {
		if parentIndex, ok := index.byID[slot.parent]; ok {
			parent := &index.slots[parentIndex]
			for i, child := range parent.children {
				if child == id {
					copy(parent.children[i:], parent.children[i+1:])
					parent.children[len(parent.children)-1] = 0
					parent.children = parent.children[:len(parent.children)-1]
					if cap(parent.children) > 8 && len(parent.children)*4 <= cap(parent.children) {
						capacity := len(parent.children) * 2
						if capacity < 8 {
							capacity = 8
						}
						children := make([]facet.FacetID, len(parent.children), capacity)
						copy(children, parent.children)
						parent.children = children
					}
					break
				}
			}
		}
		if hostedChildren := index.hosted[slot.parent]; hostedChildren != nil {
			delete(hostedChildren, id)
			if len(hostedChildren) == 0 {
				delete(index.hosted, slot.parent)
			}
		}
	}
	delete(index.byID, id)
	delete(index.hosted, id)
	var zero liveSlot
	index.slots[slotIndex] = zero
	last := len(index.slots) - 1
	if slotIndex != last {
		index.slots[slotIndex] = index.slots[last]
		index.byID[index.slots[slotIndex].id] = slotIndex
	}
	index.slots[last] = liveSlot{}
	index.slots = index.slots[:last]
	if len(index.slots)*4 <= index.directoryCapacity {
		capacity := len(index.slots) * 2
		if capacity < 8 {
			capacity = 8
		}
		slots := make([]liveSlot, len(index.slots), capacity)
		copy(slots, index.slots)
		index.slots = slots
		byID := make(map[facet.FacetID]int, capacity)
		for index, slot := range index.slots {
			byID[slot.id] = index
		}
		index.byID = byID
		index.directoryCapacity = capacity
	}
}
