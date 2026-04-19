package diagnostics

import (
	"reflect"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// FacetInfo is a read-only snapshot of one facet.
type FacetInfo struct {
	ID                facet.FacetID
	TypeName          string
	State             facet.LifecycleState
	Roles             []string
	ArrangedBounds    gfx.Rect
	DirtyFlags        facet.DirtyFlags
	ChildCount        int
	LastInvalidatedBy string
}

// Inspector provides a read-only tree view over a facet tree.
type Inspector struct {
	root facet.FacetImpl
}

// NewInspector constructs an inspector for root.
func NewInspector(root facet.FacetImpl) *Inspector {
	return &Inspector{root: root}
}

// Walk calls fn for every facet in tree order.
func (i *Inspector) Walk(fn func(depth int, info FacetInfo)) {
	if i == nil || fn == nil {
		return
	}
	walkFacet(i.root, 0, fn)
}

// Find returns the FacetInfo for the given ID.
func (i *Inspector) Find(id facet.FacetID) (FacetInfo, bool) {
	if i == nil {
		return FacetInfo{}, false
	}
	var (
		out FacetInfo
		ok  bool
	)
	i.Walk(func(depth int, info FacetInfo) {
		if ok || info.ID != id {
			return
		}
		out = info
		ok = true
	})
	return out, ok
}

// DirtySet returns all currently dirty facets and their flags.
func (i *Inspector) DirtySet() map[facet.FacetID]facet.DirtyFlags {
	if i == nil {
		return nil
	}
	out := make(map[facet.FacetID]facet.DirtyFlags)
	i.Walk(func(depth int, info FacetInfo) {
		if info.DirtyFlags != 0 {
			out[info.ID] = info.DirtyFlags
		}
	})
	if len(out) == 0 {
		return nil
	}
	return out
}

func walkFacet(node facet.FacetImpl, depth int, fn func(depth int, info FacetInfo)) {
	if node == nil || fn == nil {
		return
	}
	base := node.Base()
	if base == nil {
		return
	}
	fn(depth, facetInfoFor(node))
	for _, child := range base.Children() {
		walkFacet(child, depth+1, fn)
	}
}

func facetInfoFor(node facet.FacetImpl) FacetInfo {
	base := node.Base()
	info := FacetInfo{
		ID:                base.ID(),
		TypeName:          typeName(node),
		State:             base.State(),
		DirtyFlags:        base.DirtyFlags(),
		ChildCount:        len(base.Children()),
		LastInvalidatedBy: base.LastInvalidatedBy(),
		ArrangedBounds:    arrangedBounds(base),
		Roles:             roleNames(base),
	}
	return info
}

func typeName(v any) string {
	if v == nil {
		return ""
	}
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

func arrangedBounds(base *facet.Facet) gfx.Rect {
	if base == nil {
		return gfx.Rect{}
	}
	if lr := base.LayoutRole(); lr != nil {
		return lr.ArrangedBounds
	}
	return gfx.Rect{}
}

func roleNames(base *facet.Facet) []string {
	if base == nil {
		return nil
	}
	out := make([]string, 0, 8)
	if base.LayoutRole() != nil {
		out = append(out, "LayoutRole")
	}
	if base.RenderRole() != nil {
		out = append(out, "RenderRole")
	}
	if base.HitRole() != nil {
		out = append(out, "HitRole")
	}
	if base.InputRole() != nil {
		out = append(out, "InputRole")
	}
	if base.FocusRole() != nil {
		out = append(out, "FocusRole")
	}
	if base.ViewportRole() != nil {
		out = append(out, "ViewportRole")
	}
	if base.ProjectionRole() != nil {
		out = append(out, "ProjectionRole")
	}
	if base.TickRole() != nil {
		out = append(out, "TickRole")
	}
	return out
}
