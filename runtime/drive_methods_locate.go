package runtime

import (
	"encoding/json"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/locate"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// driveLocateError converts a typed locator error to a protocol error with
// the candidate list appended to the detail payload.
func driveLocateError(lerr *locate.Error) *proto.Error {
	code := proto.ErrorCode(lerr.Code)
	detail := lerr.Detail
	if len(lerr.Candidates) > 0 {
		if b, err := json.Marshal(lerr.Candidates); err == nil {
			detail = detail + " " + string(b)
		}
	}
	return &proto.Error{Code: code, Detail: detail}
}

// locateResolveParams is the locate.resolve payload.
type locateResolveParams struct {
	Locator locate.Locator `json:"locator"`
}

// driveLocateResolve implements locate.resolve (grant read): resolves a
// locator against the live tree at this cut and reports the match.
func driveLocateResolve(rt *Runtime, cmd drive.Command) driveOutcome {
	var p locateResolveParams
	if len(cmd.Params) > 0 {
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: "locate.resolve params: " + err.Error()})
		}
	}
	m, _, lerr := locate.Resolve(rt.root, p.Locator, rt.driveDescribe, rt.driveHitTest)
	if lerr != nil {
		return errorOutcome(driveLocateError(lerr))
	}
	payload, err := json.Marshal(map[string]any{
		"path":     m.Path,
		"testid":   m.Base.TestID(),
		"facet_id": uint64(m.Base.ID()),
	})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}

// driveDescribe adapts a facet implementation to the locator's metadata
// view through the injected adapter (runtime.Config.DescribeMark — the
// marks canonical interfaces are adapted in drive/describe; the runtime
// cannot import marks).
func (rt *Runtime) driveDescribe(impl facet.FacetImpl) locate.Meta {
	if rt.config.DescribeMark == nil {
		return locate.Meta{}
	}
	return rt.config.DescribeMark(impl)
}

// driveHitTest resolves screen coordinates through the current frame hit
// map (the Coords fallback, §3.4).
func (rt *Runtime) driveHitTest(pt gfx.Point) (facet.FacetImpl, bool) {
	hitMap := rt.layeredHitMap(rt.projectionSystem.CurrentHitMap())
	if hitMap == nil {
		return nil, false
	}
	hit := hitMap.HitTest(pt)
	if hit == nil {
		return nil, false
	}
	impl := rt.findFacetByID(rt.root, hit.FacetID)
	if impl == nil {
		return nil, false
	}
	return impl, true
}

// treeDumpParams is the tree.dump payload.
type treeDumpParams struct {
	From  *locate.Locator `json:"from,omitempty"`
	Depth int             `json:"depth,omitempty"` // 0 = unlimited
}

// treeNodeDump is one reported node (FR-9).
type treeNodeDump struct {
	Path   string          `json:"path"`
	Kind   string          `json:"kind,omitempty"`
	TestID string          `json:"testid,omitempty"`
	Role   string          `json:"role,omitempty"`
	Name   string          `json:"name,omitempty"`
	Bounds *gfx.Rect       `json:"bounds,omitempty"`
	Layer  *treeLayerState `json:"layer,omitempty"`
}

type treeLayerState struct {
	Band  string `json:"band,omitempty"`
	Mount string `json:"mount,omitempty"`
}

// driveTreeDump implements tree.dump (grant read, FR-9): descriptors, TestIDs,
// roles/names, arranged bounds, and layer state for a subtree.
func driveTreeDump(rt *Runtime, cmd drive.Command) driveOutcome {
	var p treeDumpParams
	if len(cmd.Params) > 0 {
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: "tree.dump params: " + err.Error()})
		}
	}
	root := rt.root
	if p.From != nil {
		m, _, lerr := locate.Resolve(rt.root, *p.From, rt.driveDescribe, rt.driveHitTest)
		if lerr != nil {
			return errorOutcome(driveLocateError(lerr))
		}
		root = m.Impl
	}
	out := make([]treeNodeDump, 0, 64)
	type walkNode struct {
		base  *facet.Facet
		path  string
		depth int
	}
	stack := []walkNode{{base: root.Base(), path: locate.PathFor(root.Base())}}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n.base == nil || n.base.State() == facet.StateDisposed {
			continue
		}
		entry := treeNodeDump{Path: n.path, TestID: n.base.TestID()}
		meta := rt.driveDescribe(n.base.Impl())
		entry.Kind = meta.Kind
		entry.Role = meta.Role
		entry.Name = meta.Name
		if lr := n.base.LayoutRole(); lr != nil {
			bounds := lr.ArrangedBounds
			if !bounds.IsEmpty() {
				entry.Bounds = &bounds
			}
		}
		if n.base.IsLayer() {
			att := n.base.LayerAttachment()
			state := treeLayerState{Band: att.Band.String()}
			if att.Mount != nil {
				state.Mount = "mounted"
				if !att.Mount.Get() {
					state.Mount = "unmounted"
				}
			}
			entry.Layer = &state
		}
		out = append(out, entry)
		if p.Depth > 0 && n.depth+1 >= p.Depth {
			continue
		}
		children := n.base.Children()
		for i := len(children) - 1; i >= 0; i-- {
			child := children[i]
			if child == nil {
				continue
			}
			stack = append(stack, walkNode{
				base:  child,
				path:  n.path + "/" + locate.PathFor(child),
				depth: n.depth + 1,
			})
		}
	}
	payload, err := json.Marshal(map[string]any{"nodes": out})
	if err != nil {
		return errorOutcome(&proto.Error{Code: proto.ErrInternalPanic, Detail: err.Error()})
	}
	return payloadOutcome(payload)
}
