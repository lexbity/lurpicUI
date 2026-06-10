package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/gfx"
)

const breakpointWide float32 = 960

func ModeFor(size gfx.Size) state.LayoutMode {
	if size.W >= breakpointWide {
		return state.LayoutWide
	}
	return state.LayoutNarrow
}
