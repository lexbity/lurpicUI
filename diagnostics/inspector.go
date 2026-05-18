package diagnostics

import (
	"fmt"
	"reflect"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// FacetInfo is a read-only snapshot of one facet.
type FacetInfo struct {
	ID                facet.FacetID
	TypeName          string
	State             facet.LifecycleState
	Roles             []string
	Layers            []LayerSnapshot
	ArrangedBounds    gfx.Rect
	DirtyFlags        facet.DirtyFlags
	ChildCount        int
	LastInvalidatedBy string
}

// Inspector provides a read-only tree view over a facet tree.
type Inspector struct {
	root         facet.FacetImpl
	layerSource  LayerSource
	anchorSource AnchorSource
	hitTraceSrc  HitTraceSource
}

// NewInspector constructs an inspector for root.
func NewInspector(root facet.FacetImpl) *Inspector {
	return &Inspector{root: root}
}

// SetLayerSource binds an external layer snapshot source.
func (i *Inspector) SetLayerSource(source LayerSource) {
	if i == nil {
		return
	}
	i.layerSource = source
}

// SetAnchorSource binds an external anchor snapshot source.
func (i *Inspector) SetAnchorSource(source AnchorSource) {
	if i == nil {
		return
	}
	i.anchorSource = source
}

// SetHitTraceSource binds an external hit-trace source.
func (i *Inspector) SetHitTraceSource(source HitTraceSource) {
	if i == nil {
		return
	}
	i.hitTraceSrc = source
}

// Walk calls fn for every facet in tree order.
func (i *Inspector) Walk(fn func(depth int, info FacetInfo)) {
	if i == nil || fn == nil {
		return
	}
	i.walkFacet(i.root, 0, fn)
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

// LayerSnapshots returns the resolved layers for the given parent facet.
func (i *Inspector) LayerSnapshots(parent facet.FacetID) []LayerSnapshot {
	if i == nil || i.layerSource == nil {
		return nil
	}
	return i.layerSource.LayerSnapshots(parent)
}

// AnchorSnapshot returns the resolved anchors for the given parent facet.
func (i *Inspector) AnchorSnapshot(parent facet.FacetID) (AnchorSnapshot, bool) {
	if i == nil || i.anchorSource == nil {
		return AnchorSnapshot{}, false
	}
	return i.anchorSource.AnchorSnapshot(parent)
}

// HitTrace returns the most recent hit traversal trace.
func (i *Inspector) HitTrace() HitTestTrace {
	if i == nil || i.hitTraceSrc == nil {
		return HitTestTrace{}
	}
	return i.hitTraceSrc.HitTrace()
}

// Describe renders the current tree, including layer and anchor snapshots.
func (i *Inspector) Describe() string {
	if i == nil || i.root == nil {
		return ""
	}
	var b strings.Builder
	i.Walk(func(depth int, info FacetInfo) {
		indent := strings.Repeat("  ", depth)
		fmt.Fprintf(&b, "%sFacetID: %d (%s)\n", indent, info.ID, info.TypeName)
		if len(info.Roles) > 0 {
			fmt.Fprintf(&b, "%s  Roles: %s\n", indent, strings.Join(info.Roles, ", "))
		}
		if info.LastInvalidatedBy != "" {
			fmt.Fprintf(&b, "%s  Dirty: %s\n", indent, info.LastInvalidatedBy)
		}
		if len(info.Layers) > 0 {
			fmt.Fprintf(&b, "%s  Layers:\n", indent)
			for _, layer := range info.Layers {
				fmt.Fprintf(&b, "%s    %s\n", indent, layer.String())
				if len(layer.ArrangedChildren) > 0 {
					fmt.Fprintf(&b, "%s      ArrangedChildren:\n", indent)
					for _, child := range layer.ArrangedChildren {
						fmt.Fprintf(&b, "%s        %s\n", indent, child.String())
					}
				}
			}
		}
		if snap, ok := i.AnchorSnapshot(info.ID); ok {
			fmt.Fprintf(&b, "%s  AnchorCache: %s\n", indent, snap.String())
		}
	})
	return strings.TrimRight(b.String(), "\n")
}

func (i *Inspector) walkFacet(node facet.FacetImpl, depth int, fn func(depth int, info FacetInfo)) {
	if node == nil || fn == nil {
		return
	}
	base := node.Base()
	if base == nil {
		return
	}
	fn(depth, i.facetInfoFor(node))
	for _, child := range base.Children() {
		i.walkFacet(child, depth+1, fn)
	}
}

func (i *Inspector) facetInfoFor(node facet.FacetImpl) FacetInfo {
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
	if i != nil && i.layerSource != nil {
		info.Layers = i.layerSource.LayerSnapshots(base.ID())
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
