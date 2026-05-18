package facet

import "sort"

// FocusRestoreMode declares how focus should behave when a trap closes.
type FocusRestoreMode uint8

const (
	FocusRestorePrevious FocusRestoreMode = iota
	FocusRestoreFirstFocusable
	FocusRestoreNone
)

// FocusTrapState describes one active focus trap scope.
type FocusTrapState struct {
	Scope   FacetID
	Restore FocusRestoreMode
}

// FocusManager tracks which facet currently holds keyboard focus.
// One instance, owned by the runtime, consulted by the input system.
type FocusManager struct {
	focused     FacetID
	focusedImpl FacetImpl
	tabOrder    []FacetID
	byID        map[FacetID]FacetImpl
	traps       []focusTrapFrame
}

type focusTrapFrame struct {
	scope     FacetID
	restore   FocusRestoreMode
	restoreID FacetID
}

// NewFocusManager creates an empty focus manager.
func NewFocusManager() *FocusManager {
	return &FocusManager{
		byID: make(map[FacetID]FacetImpl),
	}
}

// SetFocus grants focus to the given facet, revoking it from the current holder.
func (m *FocusManager) SetFocus(target FacetImpl) bool {
	if m == nil {
		return false
	}
	if target == nil {
		m.ClearFocus()
		return true
	}
	base := target.Base()
	if base == nil {
		return false
	}
	if !m.canFocus(target) {
		return false
	}
	if m.focused == base.ID() {
		m.focusedImpl = target
		m.byID[base.ID()] = target
		return true
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
	return true
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

// ActiveTrap returns the topmost focus-trap scope, if any.
func (m *FocusManager) ActiveTrap() FacetID {
	if m == nil || len(m.traps) == 0 {
		return 0
	}
	return m.traps[len(m.traps)-1].scope
}

// SyncFocusTraps updates the active trap stack to match the provided scopes.
func (m *FocusManager) SyncFocusTraps(traps []FocusTrapState) {
	if m == nil {
		return
	}
	normalized := make([]focusTrapFrame, 0, len(traps))
	seen := make(map[FacetID]struct{}, len(traps))
	for _, trap := range traps {
		if trap.Scope == 0 {
			continue
		}
		if _, ok := seen[trap.Scope]; ok {
			continue
		}
		seen[trap.Scope] = struct{}{}
		normalized = append(normalized, focusTrapFrame{scope: trap.Scope, restore: trap.Restore})
	}
	common := 0
	for common < len(m.traps) && common < len(normalized) {
		if m.traps[common].scope != normalized[common].scope {
			break
		}
		m.traps[common].restore = normalized[common].restore
		common++
	}
	for len(m.traps) > common {
		m.popTrap()
	}
	for i := common; i < len(normalized); i++ {
		m.pushTrap(normalized[i])
	}
}

// TabNext moves focus to the next focusable facet in tab order.
func (m *FocusManager) TabNext() {
	if m == nil || len(m.tabOrder) == 0 {
		return
	}
	order := m.filteredTabOrder()
	if len(order) == 0 {
		return
	}
	idx := m.indexOfFocused(order)
	if idx < 0 {
		m.focusByID(order[0])
		return
	}
	m.focusByID(order[(idx+1)%len(order)])
}

// TabPrev moves focus to the previous focusable facet in tab order.
func (m *FocusManager) TabPrev() {
	if m == nil || len(m.tabOrder) == 0 {
		return
	}
	order := m.filteredTabOrder()
	if len(order) == 0 {
		return
	}
	idx := m.indexOfFocused(order)
	if idx < 0 {
		m.focusByID(order[len(order)-1])
		return
	}
	m.focusByID(order[(idx-1+len(order))%len(order)])
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
		m.focused = 0
		m.focusedImpl = nil
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
		if impl, ok := m.byID[m.focused]; ok {
			if !m.isFocusable(impl) || !m.isAllowedInActiveTraps(impl.Base()) {
				m.ClearFocus()
			} else {
				m.focusedImpl = impl
			}
		} else {
			m.focused = 0
			m.focusedImpl = nil
		}
	}
}

func (m *FocusManager) filteredTabOrder() []FacetID {
	if m == nil {
		return nil
	}
	if len(m.traps) == 0 {
		return m.tabOrder
	}
	out := make([]FacetID, 0, len(m.tabOrder))
	for _, id := range m.tabOrder {
		if m.isAllowedInActiveTrapsByID(id) {
			out = append(out, id)
		}
	}
	return out
}

func (m *FocusManager) indexOfFocused(order []FacetID) int {
	if m == nil || m.focused == 0 {
		return -1
	}
	for i, id := range order {
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
		_ = m.SetFocus(impl)
	}
}

func (m *FocusManager) pushTrap(frame focusTrapFrame) {
	if m == nil || frame.scope == 0 {
		return
	}
	frame.restoreID = m.focused
	m.traps = append(m.traps, frame)
	if m.focused == 0 || !m.canFocus(m.focusedImpl) {
		if target, ok := m.firstFocusableInActiveTraps(); ok {
			_ = m.SetFocus(target)
			return
		}
		m.ClearFocus()
	}
}

func (m *FocusManager) popTrap() {
	if m == nil || len(m.traps) == 0 {
		return
	}
	frame := m.traps[len(m.traps)-1]
	m.traps = m.traps[:len(m.traps)-1]
	switch frame.restore {
	case FocusRestoreNone:
		if len(m.traps) > 0 {
			if target, ok := m.firstFocusableInActiveTraps(); ok {
				_ = m.SetFocus(target)
				return
			}
		}
		m.ClearFocus()
	case FocusRestoreFirstFocusable:
		if target, ok := m.firstFocusableInActiveTraps(); ok {
			_ = m.SetFocus(target)
			return
		}
		fallthrough
	default:
		if frame.restoreID != 0 {
			if impl, ok := m.byID[frame.restoreID]; ok && m.canFocus(impl) {
				_ = m.SetFocus(impl)
				return
			}
		}
		if target, ok := m.firstFocusableInActiveTraps(); ok {
			_ = m.SetFocus(target)
			return
		}
		m.ClearFocus()
	}
}

func (m *FocusManager) firstFocusableInActiveTraps() (FacetImpl, bool) {
	if m == nil {
		return nil, false
	}
	for _, id := range m.tabOrder {
		if impl, ok := m.byID[id]; ok && m.canFocus(impl) {
			return impl, true
		}
	}
	return nil, false
}

func (m *FocusManager) isAllowedInActiveTraps(base *Facet) bool {
	if m == nil {
		return false
	}
	if len(m.traps) == 0 {
		return true
	}
	for _, trap := range m.traps {
		if !m.isWithinScope(base, trap.scope) {
			return false
		}
	}
	return true
}

func (m *FocusManager) isAllowedInActiveTrapsByID(id FacetID) bool {
	if m == nil || id == 0 {
		return false
	}
	impl, ok := m.byID[id]
	if !ok || impl == nil {
		return false
	}
	return m.isAllowedInActiveTraps(impl.Base())
}

func (m *FocusManager) isWithinScope(base *Facet, scope FacetID) bool {
	if base == nil {
		return false
	}
	if scope == 0 {
		return true
	}
	for current := base; current != nil; current = current.parent {
		if current.ID() == scope {
			return true
		}
	}
	return false
}

func (m *FocusManager) isFocusable(target FacetImpl) bool {
	if m == nil || target == nil || target.Base() == nil {
		return false
	}
	role := target.Base().FocusRole()
	if role == nil {
		return false
	}
	focusable := true
	if role.Focusable != nil {
		focusable = role.Focusable()
	}
	return focusable
}

func (m *FocusManager) canFocus(target FacetImpl) bool {
	if m == nil || !m.isFocusable(target) {
		return false
	}
	return m.isAllowedInActiveTraps(target.Base())
}
