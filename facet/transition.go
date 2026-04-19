package facet

// FacetImpl is implemented by every concrete facet type.
type FacetImpl interface {
	Base() *Facet
	OnAttach(ctx AttachContext)
	OnDetach()
	OnActivate()
	OnDeactivate()
}

// Attach transitions a facet from Created to Attached.
func Attach(f FacetImpl, ctx AttachContext) {
	base := baseOf(f)
	f = concreteImpl(base, f)
	requireState(base, StateCreated, StateAttached)
	roles := base.rolesSnapshot()
	for _, role := range roles {
		role.onAttach(base)
	}
	f.OnAttach(ctx)
	base.setState(StateAttached)
	for _, child := range base.childrenSnapshot() {
		Attach(concreteImpl(child, child), ctx)
	}
}

// Activate transitions a facet from Attached or Inactive to Active.
func Activate(f FacetImpl) {
	base := baseOf(f)
	f = concreteImpl(base, f)
	switch base.State() {
	case StateAttached, StateInactive:
	default:
		panic(invalidTransition(base.State(), StateActive))
	}
	roles := base.rolesSnapshot()
	for _, role := range roles {
		role.onActivate(base)
	}
	f.OnActivate()
	base.setState(StateActive)
	for _, child := range base.childrenSnapshot() {
		Activate(concreteImpl(child, child))
	}
}

// Deactivate transitions a facet from Active to Inactive.
func Deactivate(f FacetImpl) {
	base := baseOf(f)
	f = concreteImpl(base, f)
	requireState(base, StateActive, StateInactive)
	roles := base.rolesSnapshot()
	for _, role := range roles {
		role.onDeactivate(base)
	}
	f.OnDeactivate()
	base.setState(StateInactive)
	for _, child := range base.childrenSnapshot() {
		Deactivate(concreteImpl(child, child))
	}
}

// Dispose transitions a facet into the terminal Disposed state.
func Dispose(f FacetImpl) {
	base := baseOf(f)
	f = concreteImpl(base, f)
	switch base.State() {
	case StateCreated, StateAttached, StateActive, StateInactive:
	default:
		panic(invalidTransition(base.State(), StateDisposed))
	}
	for _, child := range base.childrenSnapshot() {
		Dispose(concreteImpl(child, child))
	}
	roles := base.rolesSnapshot()
	for _, role := range roles {
		role.onDispose(base)
	}
	f.OnDetach()
	base.releaseSubscriptions()
	base.setState(StateDisposed)
}

func concreteImpl(base *Facet, fallback FacetImpl) FacetImpl {
	if base != nil && base.impl != nil {
		return base.impl
	}
	return fallback
}

func baseOf(f FacetImpl) *Facet {
	if f == nil {
		panic("facet: nil FacetImpl")
	}
	base := f.Base()
	if base == nil {
		panic("facet: FacetImpl returned nil Base")
	}
	return base
}

func requireState(base *Facet, want, next LifecycleState) {
	switch base.State() {
	case want:
		return
	default:
		panic(invalidTransition(base.State(), next))
	}
}
