package facet

import "sort"

// FocusManager tracks which facet currently holds keyboard focus.
// One instance, owned by the runtime, consulted by the input system.
type FocusManager struct {
	focused     FacetID
	focusedImpl FacetImpl
	tabOrder    []FacetID
	byID        map[FacetID]FacetImpl
}

// NewFocusManager creates an empty focus manager.
func NewFocusManager() *FocusManager {
	return &FocusManager{
		byID: make(map[FacetID]FacetImpl),
	}
}

// SetFocus grants focus to the given facet, revoking it from the current holder.
func (m *FocusManager) SetFocus(target FacetImpl) {
	if m == nil {
		return
	}
	if target == nil {
		m.ClearFocus()
		return
	}
	base := target.Base()
	if base == nil {
		return
	}
	if m.focused == base.ID() {
		m.focusedImpl = target
		return
	}
	if m.focusedImpl != nil {
		if role := m.focusedImpl.Base().FocusRole(); role != nil && role.OnFocusLost != nil {
			role.OnFocusLost()
		}
	}
	m.focused = base.ID()
	m.focusedImpl = target
	m.byID[base.ID()] = target
	if role := base.FocusRole(); role != nil && role.OnFocusGained != nil {
		role.OnFocusGained()
	}
}

// ClearFocus removes focus from any currently focused facet.
func (m *FocusManager) ClearFocus() {
	if m == nil {
		return
	}
	if m.focusedImpl != nil {
		if role := m.focusedImpl.Base().FocusRole(); role != nil && role.OnFocusLost != nil {
			role.OnFocusLost()
		}
	}
	m.focused = 0
	m.focusedImpl = nil
}

// Focused returns the currently focused FacetID, or 0 if none.
func (m *FocusManager) Focused() FacetID {
	if m == nil {
		return 0
	}
	return m.focused
}

// FocusedImpl returns the currently focused facet implementation, if any.
func (m *FocusManager) FocusedImpl() FacetImpl {
	if m == nil {
		return nil
	}
	return m.focusedImpl
}

// TabNext moves focus to the next focusable facet in tab order.
func (m *FocusManager) TabNext() {
	if m == nil || len(m.tabOrder) == 0 {
		return
	}
	idx := m.indexOfFocused()
	if idx < 0 {
		m.focusByID(m.tabOrder[0])
		return
	}
	m.focusByID(m.tabOrder[(idx+1)%len(m.tabOrder)])
}

// TabPrev moves focus to the previous focusable facet in tab order.
func (m *FocusManager) TabPrev() {
	if m == nil || len(m.tabOrder) == 0 {
		return
	}
	idx := m.indexOfFocused()
	if idx < 0 {
		m.focusByID(m.tabOrder[len(m.tabOrder)-1])
		return
	}
	m.focusByID(m.tabOrder[(idx-1+len(m.tabOrder))%len(m.tabOrder)])
}

// RebuildTabOrder walks the tree and re-collects tab order.
// Called by the runtime after layout and projection each frame.
func (m *FocusManager) RebuildTabOrder(root FacetImpl) {
	if m == nil {
		return
	}
	m.tabOrder = m.tabOrder[:0]
	m.byID = make(map[FacetID]FacetImpl)
	if root == nil {
		return
	}
	type entry struct {
		id    FacetID
		index int
		order int
		impl  FacetImpl
	}
	entries := make([]entry, 0, 8)
	seq := 0
	stack := []FacetImpl{root}
	for len(stack) > 0 {
		impl := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if impl == nil {
			continue
		}
		base := impl.Base()
		if base == nil {
			continue
		}
		m.byID[base.ID()] = impl
		if role := base.FocusRole(); role != nil {
			focusable := true
			if role.Focusable != nil {
				focusable = role.Focusable()
			}
			if role.TabIndex >= 0 && focusable {
				entries = append(entries, entry{id: base.ID(), index: role.TabIndex, order: seq, impl: impl})
			}
		}
		seq++
		children := base.Children()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, children[i])
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].index != entries[j].index {
			return entries[i].index < entries[j].index
		}
		return entries[i].order < entries[j].order
	})
	m.tabOrder = make([]FacetID, len(entries))
	for i, e := range entries {
		m.tabOrder[i] = e.id
	}
	if m.focused != 0 {
		if _, ok := m.byID[m.focused]; !ok {
			m.focused = 0
			m.focusedImpl = nil
		}
	}
}

func (m *FocusManager) indexOfFocused() int {
	if m == nil || m.focused == 0 {
		return -1
	}
	for i, id := range m.tabOrder {
		if id == m.focused {
			return i
		}
	}
	return -1
}

func (m *FocusManager) focusByID(id FacetID) {
	if m == nil || id == 0 {
		return
	}
	if impl, ok := m.byID[id]; ok {
		m.SetFocus(impl)
	}
}
