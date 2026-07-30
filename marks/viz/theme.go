package viz

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
)

// resolveVizColor resolves a color from a marks.Binding[gfx.Color] value with
// a theme-derived fallback.  If the binding's value is non-zero, it is used
// directly.  Otherwise the theme default replaces the old hardcoded literal.
func resolveVizColor(v gfx.Color, themeColor gfx.Color) gfx.Color {
	if v != (gfx.Color{}) {
		return v
	}
	return themeColor
}

// syncThemeColor resolves a default color for the given token from the theme
// context and stores it in the provided pointer.
//
// t MUST be the Theme value from a facet.MeasureContext or facet.ArrangeContext
// (which the runtime populates per layout pass); it MUST NOT be
// facet.AttachContext.Theme, which is nil at attach time.  Calling this from
// OnAttach is the bug it exists to prevent — OnAttach's ctx.Theme is nil, the
// nil-guard below returns early, and the mark renders with no default color.
//
// If the theme cannot be resolved (nil or unexpected type) the pointer is left
// unchanged, so the caller's zero value stands as the last-resort default.
func syncThemeColor(t any, out *gfx.Color, token theme.ColorToken) {
	if t == nil {
		return
	}
	rc, ok := t.(theme.ResolvedContext)
	if !ok {
		var rcp *theme.ResolvedContext
		if rcp, ok = t.(*theme.ResolvedContext); ok {
			rc = *rcp
		}
	}
	if ok && *out == (gfx.Color{}) {
		*out = rc.Color(token)
	}
}
