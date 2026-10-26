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
	focused      FacetID
	focusedImpl  FacetImpl
	tabOrder     []FacetID
	byID         map[FacetID]FacetImpl
	traps        []focusTrapFrame
	entries      []focusEntry
	walkStack    []FacetImpl
	RebuildCount uint64
}

type focusEntry struct {
	id    FacetID
	index int
	order int
	impl  FacetImpl
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

// Reset drops retained facet references when the owning runtime closes.
func (m *FocusManager) Reset() {
	if m == nil {
		return
	}
	m.focused = 0
	m.focusedImpl = nil
	clear(m.tabOrder)
	m.tabOrder = m.tabOrder[:0]
	clear(m.byID)
	clear(m.traps)
	m.traps = m.traps[:0]
	clear(m.entries)
	m.entries = m.entries[:0]
	clear(m.walkStack)
	m.walkStack = m.walkStack[:0]
}

// SetFocus grants focus to the given facet, revoking it from the current holder.
func (m *FocusManager) SetFocus(target FacetImpl) bool {
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
			// Synchronous focus loss (F4). The same callback also fires via
			// the input-routed FocusLostEvent, recovered separately in
			// input/routing_deliver.go (F3); both paths are quarantined.
			RunRecovered("focus_lost", m.focused, role.OnFocusLost)
		}
	}
	m.focused = base.ID()
	m.focusedImpl = target
	m.byID[base.ID()] = target
	if role := base.FocusRole(); role != nil && role.OnFocusGained != nil {
		// Synchronous focus gain (F4). The input-routed FocusGainedEvent path
		// is recovered separately in input/routing_deliver.go (F3).
		RunRecovered("focus_gained", base.ID(), role.OnFocusGained)
	}
	return true
}

// ClearFocus removes focus from any currently focused facet.
func (m *FocusManager) ClearFocus() {
	if m.focusedImpl != nil {
		if role := m.focusedImpl.Base().FocusRole(); role != nil && role.OnFocusLost != nil {
			// Synchronous focus loss (F4); the input-routed FocusLostEvent
			// path is recovered separately in input/routing_deliver.go (F3).
			RunRecovered("focus_lost", m.focused, role.OnFocusLost)
		}
	}
	m.focused = 0
	m.focusedImpl = nil
}

// Focused returns the currently focused FacetID, or 0 if none.
func (m *FocusManager) Focused() FacetID {
	return m.focused
}

// FocusedImpl returns the currently focused facet implementation, if any.
func (m *FocusManager) FocusedImpl() FacetImpl {
	return m.focusedImpl
}

// ActiveTrap returns the topmost focus-trap scope, if any.
func (m *FocusManager) ActiveTrap() FacetID {
	if len(m.traps) == 0 {
		return 0
	}
	return m.traps[len(m.traps)-1].scope
}

// SyncFocusTraps updates the active trap stack to match the provided scopes.
func (m *FocusManager) SyncFocusTraps(traps []FocusTrapState) {
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
	if len(m.tabOrder) == 0 {
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
	if len(m.tabOrder) == 0 {
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

// RebuildTabOrder walks the tree and re-collects tab order. The runtime calls
// it after focus policy or topology invalidation, not on unchanged frames.
func (m *FocusManager) RebuildTabOrder(root FacetImpl) {
	m.tabOrder = m.tabOrder[:0]
	clear(m.byID)
	clear(m.entries)
	m.entries = m.entries[:0]
	m.walkStack = m.walkStack[:0]
	m.RebuildCount++
	if root == nil {
		m.focused = 0
		m.focusedImpl = nil
		return
	}
	seq := 0
	m.walkStack = append(m.walkStack, root)
	for len(m.walkStack) > 0 {
		last := len(m.walkStack) - 1
		impl := m.walkStack[last]
		m.walkStack[last] = nil
		m.walkStack = m.walkStack[:last]
		if impl == nil {
			continue
		}
		base := impl.Base()
		if base == nil {
			continue
		}
		m.byID[base.ID()] = impl
		if role := base.FocusRole(); role != nil {
			focusable := role.IsFocusable()
			if role.TabOrder() >= 0 && focusable {
				m.entries = append(m.entries, focusEntry{id: base.ID(), index: role.TabOrder(), order: seq, impl: impl})
			}
		}
		seq++
		base.VisitChildrenReverse(func(child FacetImpl) bool {
			m.walkStack = append(m.walkStack, child)
			return true
		})
	}

	sort.SliceStable(m.entries, func(i, j int) bool {
		if m.entries[i].index != m.entries[j].index {
			return m.entries[i].index < m.entries[j].index
		}
		return m.entries[i].order < m.entries[j].order
	})
	if cap(m.tabOrder) < len(m.entries) {
		m.tabOrder = make([]FacetID, len(m.entries))
	} else {
		m.tabOrder = m.tabOrder[:len(m.entries)]
	}
	for i, e := range m.entries {
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
	if m.focused == 0 {
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
	if id == 0 {
		return
	}
	if impl, ok := m.byID[id]; ok {
		_ = m.SetFocus(impl)
	}
}

func (m *FocusManager) pushTrap(frame focusTrapFrame) {
	if frame.scope == 0 {
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
	if len(m.traps) == 0 {
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
	for _, id := range m.tabOrder {
		if impl, ok := m.byID[id]; ok && m.canFocus(impl) {
			return impl, true
		}
	}
	return nil, false
}

func (m *FocusManager) isAllowedInActiveTraps(base *Facet) bool {
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
	if id == 0 {
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
	if target == nil || target.Base() == nil {
		return false
	}
	role := target.Base().FocusRole()
	if role == nil {
		return false
	}
	return role.IsFocusable()
}

func (m *FocusManager) canFocus(target FacetImpl) bool {
	if !m.isFocusable(target) {
		return false
	}
	return m.isAllowedInActiveTraps(target.Base())
}
