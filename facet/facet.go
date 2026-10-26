package facet

import (
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
)

// Facet is the base type embedded by all concrete facet implementations.
// It owns identity, lifecycle state, child structure, and local dirty state.
type Facet struct {
	id             FacetID
	state          LifecycleState
	testID         string // author identity for LDP locators (QP-7); set in Created only
	testIDRegistry TestIDRegistry

	parent                *Facet
	children              []*Facet
	impl                  FacetImpl
	roles                 []Role
	subs                  signal.Subscriptions
	storeScope            *store.Scope
	onDisposed            func(FacetID)
	onTickActivityChanged func(FacetID, *TickRole, bool)
	onFocusPolicyChanged  func(FacetID)
	textLeases            []*text.LayoutLease

	dirtyLayout       bool
	dirtyProjection   bool
	dirtyHit          bool
	lastInvalidatedBy string

	subscribedVersions []store.Version

	// layer holds the layer contract set during construction by AttachLayer.
	// The runtime picks this up during layer resolution.
	layer    LayerAttachment
	layerSet bool
}

// ReplaceTextLeases releases the previous shaped layouts for this facet.
// The facet calls this at a shaping boundary before adopting the next set.
func (f *Facet) ReplaceTextLeases() {
	if f == nil {
		return
	}
	f.releaseTextLeases(true)
}

// ReleaseTextLeases ends all shaped-layout ownership and drops the backing
// lease table. Disposal uses this to release per-facet high-water storage.
func (f *Facet) ReleaseTextLeases() {
	if f == nil {
		return
	}
	f.releaseTextLeases(false)
}

func (f *Facet) releaseTextLeases(retainCapacity bool) {
	for _, lease := range f.textLeases {
		lease.Release()
	}
	clear(f.textLeases)
	if retainCapacity {
		f.textLeases = f.textLeases[:0]
		return
	}
	f.textLeases = nil
}

// KeepTextLease transfers a layout lease to the facet lifecycle owner.
func (f *Facet) KeepTextLease(lease *text.LayoutLease) {
	if f == nil || lease == nil {
		return
	}
	f.textLeases = append(f.textLeases, lease)
}

// NewFacet constructs a facet in the Created state with a unique ID.
func NewFacet() Facet {
	return Facet{
		id:    nextID(),
		state: StateCreated,
	}
}

// ID returns the facet identity.
func (f *Facet) ID() FacetID {
	return f.id
}

// State returns the current lifecycle state.
func (f *Facet) State() LifecycleState {
	return f.state
}

// IsDisposed reports whether the facet has been disposed.
func (f *Facet) IsDisposed() bool {
	return f.state == StateDisposed
}

// Parent returns the parent facet, if any.
func (f *Facet) Parent() *Facet {
	return f.parent
}

// Children returns a copy of the current child list.
func (f *Facet) Children() []*Facet {
	if len(f.children) == 0 {
		return []*Facet{}
	}
	out := make([]*Facet, len(f.children))
	copy(out, f.children)
	return out
}

// HasChildren reports whether the facet has any immediate children without
// copying the public child snapshot.
func (f *Facet) HasChildren() bool { return f != nil && len(f.children) != 0 }

// VisitChildren visits the current immediate children without allocating a
// public snapshot. The callback must not mutate the tree or retain the child
// implementations; return false to stop the visit early.
func (f *Facet) VisitChildren(visit func(FacetImpl) bool) bool {
	if f == nil || visit == nil {
		return true
	}
	for _, child := range f.children {
		if child == nil {
			continue
		}
		impl := child.impl
		if impl == nil {
			impl = child
		}
		if !visit(impl) {
			return false
		}
	}
	return true
}

// VisitChildrenReverse visits immediate children in reverse construction order
// without allocating a public snapshot. It has the same callback contract as
// VisitChildren and is intended for owner-managed depth-first stacks.
func (f *Facet) VisitChildrenReverse(visit func(FacetImpl) bool) bool {
	if f == nil || visit == nil {
		return true
	}
	for i := len(f.children) - 1; i >= 0; i-- {
		child := f.children[i]
		if child == nil {
			continue
		}
		impl := child.impl
		if impl == nil {
			impl = child
		}
		if !visit(impl) {
			return false
		}
	}
	return true
}

// BindImpl records the concrete implementation that owns this base facet.
// Concrete facets call this from their Base method so lifecycle traversal can
// dispatch to the real implementation when they are nested as children.
//
// Binding is write-once: the implementation is bound by the first call and
// every later call is a read-only no-op. This keeps the canonical lifecycle
// owner stable while the runtime walks facets and resolves mark capabilities.
func (f *Facet) BindImpl(impl FacetImpl) {
	if impl == nil {
		return
	}
	if f.impl != nil {
		if f.impl != impl {
			panic("facet: BindImpl called with a different implementation")
		}
		return
	}
	f.impl = impl
}

// Impl returns the bound concrete implementation, if one has been recorded.
func (f *Facet) Impl() FacetImpl {
	return f.impl
}

// DirtyFlags reports the current dirty bits.
func (f *Facet) DirtyFlags() DirtyFlags {
	var flags DirtyFlags
	if f.dirtyLayout {
		flags |= DirtyLayout
	}
	if f.dirtyProjection {
		flags |= DirtyProjection
	}
	if f.dirtyHit {
		flags |= DirtyHit
	}
	return flags
}

// Subs returns the facet-owned subscription bag.
func (f *Facet) Subs() *signal.Subscriptions {
	return &f.subs
}

// OwnStoreScope assigns the facet's explicitly created reactive child scope.
// Disposal closes this scope after detach hooks and subscriptions have run.
func (f *Facet) OwnStoreScope(scope *store.Scope) {
	if f == nil || scope == nil {
		panic("facet: owned store scope is required")
	}
	if f.storeScope != nil && f.storeScope != scope {
		panic("facet: store scope already assigned")
	}
	f.storeScope = scope
}

// StoreScope returns the facet-owned reactive scope, if it has one.
func (f *Facet) StoreScope() *store.Scope {
	if f == nil {
		return nil
	}
	return f.storeScope
}

func (f *Facet) releaseStoreScope() {
	if f.storeScope != nil {
		f.storeScope.Close()
		f.storeScope = nil
	}
}

// SubscribedVersions returns a copy of the versions registered for cache keys.
func (f *Facet) SubscribedVersions() []store.Version {
	if len(f.subscribedVersions) == 0 {
		return []store.Version{}
	}
	out := make([]store.Version, len(f.subscribedVersions))
	copy(out, f.subscribedVersions)
	return out
}

// TrackVersion registers the current version from versionFn and returns its slot.
// External packages use the returned slot to keep the version source updated.
func (f *Facet) TrackVersion(versionFn func() store.Version) int {
	if versionFn == nil {
		return -1
	}
	f.subscribedVersions = append(f.subscribedVersions, versionFn())
	return len(f.subscribedVersions) - 1
}

// UpdateTrackedVersion refreshes a previously registered version slot.
func (f *Facet) UpdateTrackedVersion(index int, versionFn func() store.Version) {
	if index < 0 || index >= len(f.subscribedVersions) || versionFn == nil {
		return
	}
	f.subscribedVersions[index] = versionFn()
}

// AddChild attaches a child facet.
func (f *Facet) AddChild(child *Facet) {
	if f == nil {
		panic("facet: nil parent in AddChild")
	}
	if f.state != StateCreated {
		panic("facet: AddChild after attach")
	}
	if child == nil {
		panic("facet: nil child in AddChild")
	}
	if child == f {
		panic("facet: AddChild cannot add facet as its own child")
	}
	for parent := f; parent != nil; parent = parent.parent {
		if parent == child {
			panic("facet: AddChild would create a cycle")
		}
	}
	if child.parent != nil {
		panic("facet: child already has a parent")
	}
	child.parent = f
	f.children = append(f.children, child)
}

// AddChildRuntime attaches a child facet after the parent has been attached.
// This is used by the runtime for dynamic tree updates.
func (f *Facet) AddChildRuntime(child *Facet) {
	if f == nil {
		panic("facet: nil parent in AddChildRuntime")
	}
	if f.state == StateDisposed {
		panic("facet: AddChildRuntime on disposed parent")
	}
	if child == nil {
		panic("facet: nil child in AddChildRuntime")
	}
	if child == f {
		panic("facet: AddChildRuntime cannot add facet as its own child")
	}
	for parent := f; parent != nil; parent = parent.parent {
		if parent == child {
			panic("facet: AddChildRuntime would create a cycle")
		}
	}
	if child.parent != nil {
		panic("facet: child already has a parent")
	}
	if child.state != StateCreated {
		panic("facet: AddChildRuntime requires a created child")
	}
	child.parent = f
	f.children = append(f.children, child)
}

// RemoveChild detaches and disposes a child facet.
func (f *Facet) RemoveChild(child *Facet) {
	if f == nil {
		panic("facet: nil parent in RemoveChild")
	}
	if child == nil {
		return
	}
	index := -1
	for i, candidate := range f.children {
		if candidate == child {
			index = i
			break
		}
	}
	if index < 0 {
		panic("facet: RemoveChild called for non-child")
	}
	copy(f.children[index:], f.children[index+1:])
	last := len(f.children) - 1
	f.children[last] = nil
	f.children = f.children[:last]
	if cap(f.children) > 8 && len(f.children)*4 <= cap(f.children) {
		capacity := len(f.children) * 2
		if capacity < 8 {
			capacity = 8
		}
		children := make([]*Facet, len(f.children), capacity)
		copy(children, f.children)
		f.children = children
	}
	child.parent = nil
	if child.state != StateDisposed {
		Dispose(child)
	}
}

// Invalidate marks the facet dirty.
func (f *Facet) Invalidate(flags DirtyFlags) {
	f.InvalidateWithSource(flags, "")
}

// InvalidateWithSource marks the facet dirty and records the source tag.
func (f *Facet) InvalidateWithSource(flags DirtyFlags, source string) {
	if f == nil {
		panic("facet: nil facet in Invalidate")
	}
	if f.state == StateDisposed {
		panic("facet: Invalidate after dispose")
	}
	if flags&DirtyLayout != 0 {
		f.dirtyLayout = true
		if role := f.LayoutRole(); role != nil {
			role.InvalidateCache()
		}
	}
	if flags&DirtyProjection != 0 {
		f.dirtyProjection = true
	}
	if flags&DirtyHit != 0 {
		f.dirtyHit = true
	}
	if source != "" {
		f.lastInvalidatedBy = source
	}
}

// ClearDirty clears the supplied dirty flags.
func (f *Facet) ClearDirty(flags DirtyFlags) {
	if f == nil {
		panic("facet: nil facet in ClearDirty")
	}
	if f.state == StateDisposed {
		panic("facet: ClearDirty after dispose")
	}
	if flags&DirtyLayout != 0 {
		f.dirtyLayout = false
	}
	if flags&DirtyProjection != 0 {
		f.dirtyProjection = false
	}
	if flags&DirtyHit != 0 {
		f.dirtyHit = false
	}
	if !f.dirtyLayout && !f.dirtyProjection && !f.dirtyHit {
		f.lastInvalidatedBy = ""
	}
}

// AddRole registers a role on the facet.
func (f *Facet) AddRole(r Role) {
	if f == nil {
		panic("facet: nil facet in AddRole")
	}
	if f.state != StateCreated {
		panic("facet: AddRole after attach")
	}
	if r == nil {
		panic("facet: nil role in AddRole")
	}
	for _, existing := range f.roles {
		if existing == r {
			panic("facet: duplicate role registration")
		}
	}
	f.roles = append(f.roles, r)
}

// LastInvalidatedBy reports the source tag of the last invalidation.
func (f *Facet) LastInvalidatedBy() string {
	return f.lastInvalidatedBy
}

// LayoutRole returns the registered layout role, if any.
func (f *Facet) LayoutRole() *LayoutRole {
	for _, role := range f.roles {
		if typed, ok := role.(*LayoutRole); ok {
			return typed
		}
	}
	return nil
}

// RenderRole returns the registered render role, if any.
func (f *Facet) RenderRole() *RenderRole {
	for _, role := range f.roles {
		if typed, ok := role.(*RenderRole); ok {
			return typed
		}
	}
	return nil
}

// HitRole returns the registered hit role, if any.
func (f *Facet) HitRole() *HitRole {
	for _, role := range f.roles {
		if typed, ok := role.(*HitRole); ok {
			return typed
		}
	}
	return nil
}

// InputRole returns the registered input role, if any.
func (f *Facet) InputRole() *InputRole {
	for _, role := range f.roles {
		if typed, ok := role.(*InputRole); ok {
			return typed
		}
	}
	return nil
}

// FocusRole returns the registered focus role, if any.
func (f *Facet) FocusRole() *FocusRole {
	for _, role := range f.roles {
		if typed, ok := role.(*FocusRole); ok {
			return typed
		}
	}
	return nil
}

// ViewportRole returns the registered viewport role, if any.
func (f *Facet) ViewportRole() *ViewportRole {
	for _, role := range f.roles {
		if typed, ok := role.(*ViewportRole); ok {
			return typed
		}
	}
	return nil
}

// ProjectionRole returns the registered projection role, if any.
func (f *Facet) ProjectionRole() *ProjectionRole {
	for _, role := range f.roles {
		if typed, ok := role.(*ProjectionRole); ok {
			return typed
		}
	}
	return nil
}

// TextRole returns the registered text role, if any.
func (f *Facet) TextRole() *TextRole {
	for _, role := range f.roles {
		if typed, ok := role.(*TextRole); ok {
			return typed
		}
	}
	return nil
}

// TickRole returns the registered tick role, if any.
func (f *Facet) TickRole() *TickRole {
	for _, role := range f.roles {
		if typed, ok := role.(*TickRole); ok {
			return typed
		}
	}
	return nil
}

// Base satisfies the FacetImpl interface for the base type in tests.
func (f *Facet) Base() *Facet {
	return f
}

// OnAttach is a no-op default for structural facets.
func (f *Facet) OnAttach(ctx AttachContext) {}

// OnDetach is a no-op default for structural facets.
func (f *Facet) OnDetach() {}

// OnActivate is a no-op default for structural facets.
func (f *Facet) OnActivate() {}

// OnDeactivate is a no-op default for structural facets.
func (f *Facet) OnDeactivate() {}

func (f *Facet) setState(state LifecycleState) {
	f.state = state
}

func (f *Facet) rolesSnapshot() []Role {
	if len(f.roles) == 0 {
		return []Role{}
	}
	out := make([]Role, len(f.roles))
	copy(out, f.roles)
	return out
}

// LayerAttachment returns the layer contract set by AttachLayer during
// construction, or the zero value for a plain child.
func (f *Facet) LayerAttachment() LayerAttachment {
	if f == nil {
		return LayerAttachment{}
	}
	return f.layer
}

// IsLayer reports whether the facet was mounted via AttachLayer.
func (f *Facet) IsLayer() bool {
	return f != nil && f.layerSet
}

func (f *Facet) releaseSubscriptions() {
	f.subs.Release()
}
