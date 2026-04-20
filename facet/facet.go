package facet

import (
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// Facet is the base type embedded by all concrete facet implementations.
// It owns identity, lifecycle state, child structure, and local dirty state.
type Facet struct {
	id    FacetID
	state LifecycleState

	parent   *Facet
	children []*Facet
	impl     FacetImpl
	roles    []Role
	subs     signal.Subscriptions

	dirtyLayout       bool
	dirtyProjection   bool
	dirtyHit          bool
	lastInvalidatedBy string

	subscribedVersions []store.Version
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
	if f == nil {
		return 0
	}
	return f.id
}

// State returns the current lifecycle state.
func (f *Facet) State() LifecycleState {
	if f == nil {
		return StateDisposed
	}
	return f.state
}

// Parent returns the parent facet, if any.
func (f *Facet) Parent() *Facet {
	if f == nil {
		return nil
	}
	return f.parent
}

// Children returns a copy of the current child list.
func (f *Facet) Children() []*Facet {
	if f == nil || len(f.children) == 0 {
		return nil
	}
	out := make([]*Facet, len(f.children))
	copy(out, f.children)
	return out
}

// BindImpl records the concrete implementation that owns this base facet.
// Concrete facets call this from their Base method so lifecycle traversal can
// dispatch to the real implementation when they are nested as children.
func (f *Facet) BindImpl(impl FacetImpl) {
	if f == nil || impl == nil {
		return
	}
	if f.impl != nil && f.impl != impl {
		panic("facet: BindImpl called with a different implementation")
	}
	f.impl = impl
}

// Impl returns the bound concrete implementation, if one has been recorded.
func (f *Facet) Impl() FacetImpl {
	if f == nil {
		return nil
	}
	return f.impl
}

// DirtyFlags reports the current dirty bits.
func (f *Facet) DirtyFlags() DirtyFlags {
	if f == nil {
		return 0
	}
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
	if f == nil {
		return nil
	}
	return &f.subs
}

// SubscribedVersions returns a copy of the versions registered for cache keys.
func (f *Facet) SubscribedVersions() []store.Version {
	if f == nil || len(f.subscribedVersions) == 0 {
		return nil
	}
	out := make([]store.Version, len(f.subscribedVersions))
	copy(out, f.subscribedVersions)
	return out
}

// TrackVersion registers the current version from versionFn and returns its slot.
// External packages use the returned slot to keep the version source updated.
func (f *Facet) TrackVersion(versionFn func() store.Version) int {
	if f == nil || versionFn == nil {
		return -1
	}
	f.subscribedVersions = append(f.subscribedVersions, versionFn())
	return len(f.subscribedVersions) - 1
}

// UpdateTrackedVersion refreshes a previously registered version slot.
func (f *Facet) UpdateTrackedVersion(index int, versionFn func() store.Version) {
	if f == nil || index < 0 || index >= len(f.subscribedVersions) || versionFn == nil {
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
	f.children = append(f.children[:index], f.children[index+1:]...)
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
	if f == nil {
		return ""
	}
	return f.lastInvalidatedBy
}

// LayoutRole returns the registered layout role, if any.
func (f *Facet) LayoutRole() *LayoutRole {
	if f == nil {
		return nil
	}
	for _, role := range f.roles {
		if typed, ok := role.(*LayoutRole); ok {
			return typed
		}
	}
	return nil
}

// RenderRole returns the registered render role, if any.
func (f *Facet) RenderRole() *RenderRole {
	if f == nil {
		return nil
	}
	for _, role := range f.roles {
		if typed, ok := role.(*RenderRole); ok {
			return typed
		}
	}
	return nil
}

// HitRole returns the registered hit role, if any.
func (f *Facet) HitRole() *HitRole {
	if f == nil {
		return nil
	}
	for _, role := range f.roles {
		if typed, ok := role.(*HitRole); ok {
			return typed
		}
	}
	return nil
}

// InputRole returns the registered input role, if any.
func (f *Facet) InputRole() *InputRole {
	if f == nil {
		return nil
	}
	for _, role := range f.roles {
		if typed, ok := role.(*InputRole); ok {
			return typed
		}
	}
	return nil
}

// FocusRole returns the registered focus role, if any.
func (f *Facet) FocusRole() *FocusRole {
	if f == nil {
		return nil
	}
	for _, role := range f.roles {
		if typed, ok := role.(*FocusRole); ok {
			return typed
		}
	}
	return nil
}

// ViewportRole returns the registered viewport role, if any.
func (f *Facet) ViewportRole() *ViewportRole {
	if f == nil {
		return nil
	}
	for _, role := range f.roles {
		if typed, ok := role.(*ViewportRole); ok {
			return typed
		}
	}
	return nil
}

// ProjectionRole returns the registered projection role, if any.
func (f *Facet) ProjectionRole() *ProjectionRole {
	if f == nil {
		return nil
	}
	for _, role := range f.roles {
		if typed, ok := role.(*ProjectionRole); ok {
			return typed
		}
	}
	return nil
}

// TextRole returns the registered text role, if any.
func (f *Facet) TextRole() *TextRole {
	if f == nil {
		return nil
	}
	for _, role := range f.roles {
		if typed, ok := role.(*TextRole); ok {
			return typed
		}
	}
	return nil
}

// TickRole returns the registered tick role, if any.
func (f *Facet) TickRole() *TickRole {
	if f == nil {
		return nil
	}
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
		return nil
	}
	out := make([]Role, len(f.roles))
	copy(out, f.roles)
	return out
}

func (f *Facet) childrenSnapshot() []*Facet {
	if len(f.children) == 0 {
		return nil
	}
	out := make([]*Facet, len(f.children))
	copy(out, f.children)
	return out
}

func (f *Facet) releaseSubscriptions() {
	f.subs.Release()
}
