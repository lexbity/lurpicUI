package facet

import "fmt"

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
	attached := make([]*Facet, 0, 16)
	defer func() {
		if recovered := recover(); recovered != nil {
			for i := len(attached) - 1; i >= 0; i-- {
				base := attached[i]
				base.releaseTestID()
				base.bindTestIDRegistry(nil)
			}
			panic(recovered)
		}
	}()
	type attachFrame struct {
		impl    FacetImpl
		context string
	}
	rootContext := ctx.IdentityContext
	if rootContext == "" {
		rootContext = "runtime"
	}
	stack := []attachFrame{{impl: f, context: rootContext}}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		base := baseOf(frame.impl)
		frame.impl = concreteImpl(base, frame.impl)
		requireState(base, StateCreated, StateAttached)
		base.bindTestIDRegistry(ctx.TestIDs)
		roles := base.rolesSnapshot()
		for _, role := range roles {
			role.onAttach(base)
		}
		attachContext := ctx
		segment := base.TestID()
		if segment == "" {
			segment = fmt.Sprintf("facet#%d", base.ID())
		}
		attachContext.IdentityContext = frame.context + "/" + segment
		frame.impl.OnAttach(attachContext)
		base.registerTestID(attachContext.IdentityContext)
		base.setState(StateAttached)
		attached = append(attached, base)
		children := base.childrenSnapshot()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, attachFrame{impl: concreteImpl(children[i], children[i]), context: attachContext.IdentityContext})
		}
	}
}

// Activate transitions a facet from Attached or Inactive to Active.
func Activate(f FacetImpl) {
	type activateFrame struct {
		impl FacetImpl
	}
	stack := []activateFrame{{impl: f}}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		base := baseOf(frame.impl)
		frame.impl = concreteImpl(base, frame.impl)
		switch base.State() {
		case StateAttached, StateInactive:
		default:
			panic(invalidTransition(base.State(), StateActive))
		}
		roles := base.rolesSnapshot()
		for _, role := range roles {
			role.onActivate(base)
		}
		frame.impl.OnActivate()
		base.setState(StateActive)
		children := base.childrenSnapshot()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, activateFrame{impl: concreteImpl(children[i], children[i])})
		}
	}
}

// Deactivate transitions a facet from Active to Inactive.
func Deactivate(f FacetImpl) {
	type deactivateFrame struct {
		impl FacetImpl
	}
	stack := []deactivateFrame{{impl: f}}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		base := baseOf(frame.impl)
		frame.impl = concreteImpl(base, frame.impl)
		requireState(base, StateActive, StateInactive)
		roles := base.rolesSnapshot()
		for _, role := range roles {
			role.onDeactivate(base)
		}
		frame.impl.OnDeactivate()
		base.setState(StateInactive)
		children := base.childrenSnapshot()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, deactivateFrame{impl: concreteImpl(children[i], children[i])})
		}
	}
}

// Dispose transitions a facet into the terminal Disposed state.
func Dispose(f FacetImpl) {
	type disposeFrame struct {
		impl    FacetImpl
		entered bool
	}
	stack := []disposeFrame{{impl: f}}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		base := baseOf(frame.impl)
		frame.impl = concreteImpl(base, frame.impl)
		if !frame.entered {
			switch base.State() {
			case StateCreated, StateAttached, StateActive, StateInactive:
			default:
				panic(invalidTransition(base.State(), StateDisposed))
			}
			stack = append(stack, disposeFrame{impl: frame.impl, entered: true})
			children := base.childrenSnapshot()
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, disposeFrame{impl: concreteImpl(children[i], children[i])})
			}
			continue
		}
		roles := base.rolesSnapshot()
		for _, role := range roles {
			role.onDispose(base)
		}
		frame.impl.OnDetach()
		base.releaseSubscriptions()
		base.releaseStoreScope()
		base.releaseTestID()
		base.setState(StateDisposed)
	}
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
