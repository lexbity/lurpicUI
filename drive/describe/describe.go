// Package describe adapts the marks canonical capability interfaces
// (Descriptor, Accessible, KeyedItem) to the drive locator's metadata view.
// It exists because the runtime cannot import marks (marks reads runtime
// services), so the adapter is injected via runtime.Config.DescribeMark —
// the same injection pattern as runtime.Config.IconResolver. Apps and
// harnesses that link marks wire it in; without it, kind/role/name/key
// locators resolve empty and only TestID/coords match.
package describe

import (
	"codeburg.org/lexbit/lurpicui/drive/locate"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
)

// Describe adapts a facet implementation to the locator metadata view.
func Describe(impl facet.FacetImpl) locate.Meta {
	if impl == nil {
		return locate.Meta{}
	}
	var m locate.Meta
	if d, ok := impl.(interface{ Descriptor() marks.Descriptor }); ok {
		desc := d.Descriptor()
		m.Kind = desc.Family + "/" + desc.TypeName
	}
	if a, ok := impl.(marks.Accessible); ok {
		m.Role = a.AccessibilityRole()
		m.Name = a.AccessibleName()
	}
	if k, ok := impl.(marks.KeyedItem); ok {
		m.Key = k.ItemKey()
	}
	return m
}
