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
	hooked := make([]*Facet, 0, 16)
	defer func() {
		if recovered := recover(); recovered != nil {
			for i := len(hooked) - 1; i >= 0; i-- {
				hooked[i].onDisposed = nil
				hooked[i].onInvalidated = nil
			}
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
		base.onDisposed = ctx.OnFacetDisposed
		base.onInvalidated = ctx.OnFacetInvalidated
		base.onTickActivityChanged = ctx.OnTickActivityChanged
		base.onFocusPolicyChanged = ctx.OnFocusPolicyChanged
		hooked = append(hooked, base)
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
		stack = appendChildrenReverse(stack, base, func(child FacetImpl) attachFrame {
			return attachFrame{impl: child, context: attachContext.IdentityContext}
		})
	}
	if ctx.OnFacetTreeAttached != nil {
		ctx.OnFacetTreeAttached(f)
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
		stack = appendChildrenReverse(stack, base, func(child FacetImpl) activateFrame {
			return activateFrame{impl: child}
		})
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
		stack = appendChildrenReverse(stack, base, func(child FacetImpl) deactivateFrame {
			return deactivateFrame{impl: child}
		})
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
			stack = appendChildrenReverse(stack, base, func(child FacetImpl) disposeFrame {
				return disposeFrame{impl: child}
			})
			continue
		}
		roles := base.rolesSnapshot()
		for _, role := range roles {
			role.onDispose(base)
		}
		frame.impl.OnDetach()
		base.onInvalidated = nil
		base.ReleaseTextLeases()
		base.releaseSubscriptions()
		base.releaseStoreScope()
		base.releaseTestID()
		base.setState(StateDisposed)
		onDisposed := base.onDisposed
		base.onDisposed = nil
		if onDisposed != nil {
			onDisposed(base.ID())
		}
	}
}

func appendChildrenReverse[T any](stack []T, base *Facet, wrap func(FacetImpl) T) []T {
	base.VisitChildrenReverse(func(child FacetImpl) bool {
		stack = append(stack, wrap(child))
		return true
	})
	return stack
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
