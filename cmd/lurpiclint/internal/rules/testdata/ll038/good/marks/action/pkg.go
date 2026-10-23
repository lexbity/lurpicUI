package action

import (
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/runtime"
)

// The shared marks-root helpers are the sanctioned access path.
func goodShaper(rt any) *struct{ x int } {
	_ = marks.ShaperFor(rt)
	_ = marks.FontRegistryFrom(rt)
	return nil
}

func goodIcon(rt any, ref string) (runtime.IconAsset, bool) {
	asset, ok := marks.ResolveIconFrom(rt, ref)
	return asset, ok
}
