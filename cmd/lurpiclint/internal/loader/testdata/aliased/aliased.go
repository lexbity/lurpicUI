package aliased

import (
	f "codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	_ "codeburg.org/lexbit/lurpicui/layout"
	. "codeburg.org/lexbit/lurpicui/projection"
)

func UseAlias() f.LayoutRole {
	return f.LayoutRole{}
}

func UseNoAlias() gfx.Rect {
	return gfx.Rect{}
}

func UseDotImport() { _ = ProjectionMethod }
