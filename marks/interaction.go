package marks

import "codeburg.org/lexbit/lurpicui/theme"

// InteractionStateFor is the canonical interaction-state decision table every
// mark applies over its hover/press/focus flags: disabled wins over pressed,
// pressed over hover, hover over focus-visible. Marks with additional states
// (selected, checked) branch before or after this call.
func InteractionStateFor(disabled, pressed, hovered, focusedVisible bool) theme.InteractionState {
	switch {
	case disabled:
		return theme.StateDisabled
	case pressed:
		return theme.StatePressed
	case hovered:
		return theme.StateHover
	case focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}
