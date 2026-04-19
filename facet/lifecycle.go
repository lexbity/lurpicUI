package facet

// LifecycleState tracks where a facet sits in the runtime lifecycle.
type LifecycleState uint8

const (
	StateCreated LifecycleState = iota
	StateAttached
	StateActive
	StateInactive
	StateDisposed
)

func (s LifecycleState) String() string {
	switch s {
	case StateCreated:
		return "StateCreated"
	case StateAttached:
		return "StateAttached"
	case StateActive:
		return "StateActive"
	case StateInactive:
		return "StateInactive"
	case StateDisposed:
		return "StateDisposed"
	default:
		return "LifecycleState(unknown)"
	}
}

// DirtyFlags mark which parts of the projection pipeline need recomputation.
type DirtyFlags uint8

const (
	DirtyLayout DirtyFlags = 1 << iota
	DirtyProjection
	DirtyHit
	DirtyAll = DirtyLayout | DirtyProjection | DirtyHit
)

func (d DirtyFlags) String() string {
	if d == 0 {
		return "0"
	}
	parts := make([]string, 0, 3)
	if d&DirtyLayout != 0 {
		parts = append(parts, "DirtyLayout")
	}
	if d&DirtyProjection != 0 {
		parts = append(parts, "DirtyProjection")
	}
	if d&DirtyHit != 0 {
		parts = append(parts, "DirtyHit")
	}
	if len(parts) == 1 {
		return parts[0]
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += "|" + parts[i]
	}
	return out
}

func invalidTransition(current, next LifecycleState) string {
	return "facet: invalid lifecycle transition " + current.String() + " -> " + next.String()
}
