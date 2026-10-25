// Package locate implements the explicit LDP locator grammar and canonical
// visible-tree walk. Runtime supplies mark descriptions and hosted children;
// this package never probes mark implementations itself.
package locate

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"unicode/utf8"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

type Locator struct {
	TestID string      `json:"testid,omitempty"`
	Role   string      `json:"role,omitempty"`
	Name   string      `json:"name,omitempty"`
	Key    string      `json:"key,omitempty"`
	Text   string      `json:"text,omitempty"`
	Kind   string      `json:"kind,omitempty"`
	Coords *[2]float32 `json:"coords,omitempty"`
	Window string      `json:"window,omitempty"`
	Within *Locator    `json:"within,omitempty"`
}

type Candidate struct {
	FacetPath string `json:"path"`
	TestID    string `json:"testid,omitempty"`
	Role      string `json:"role,omitempty"`
	Name      string `json:"name,omitempty"`
	Kind      string `json:"kind,omitempty"`
}

type Match struct {
	Impl facet.FacetImpl
	Base *facet.Facet
	Path string
}

type Meta struct {
	Kind string
	Role string
	Name string
	Key  string
}

type DescribeFunc func(impl facet.FacetImpl) Meta
type HitTestFunc func(pt gfx.Point) (facet.FacetImpl, bool)
type VisibleChildrenFunc func(impl facet.FacetImpl) []facet.FacetImpl

type Node struct {
	Impl     facet.FacetImpl
	Base     *facet.Facet
	Path     string
	ParentID facet.FacetID
	Depth    int
	Meta     Meta
}

type Error struct {
	Code       string
	Detail     string
	Candidates []Candidate
	Truncated  bool
}

const maxCandidates = 10

func (e *Error) Error() string { return e.Code + ": " + e.Detail }

func ErrUnresolved(l Locator) *Error {
	return &Error{Code: "locator.unresolved", Detail: fmt.Sprintf("no facet matches %+v", l)}
}

func ErrAmbiguous(l Locator, candidates []Candidate, truncated bool) *Error {
	return &Error{Code: "locator.ambiguous", Detail: fmt.Sprintf("multiple facets match %+v", l), Candidates: candidates, Truncated: truncated}
}

// Validate checks the locator oneof and its bounded scope nesting.
func Validate(l Locator) error {
	return validate(l, 0)
}

// StableAddress reports whether this locator uses author identity or a
// scoped stable item key, the only locator forms allowed to address a mark
// export capability.
func (l Locator) StableAddress() bool { return l.TestID != "" || l.Key != "" }

func validate(l Locator, depth int) error {
	if depth > 8 {
		return fmt.Errorf("locator within nesting exceeds eight")
	}
	for _, value := range []string{l.TestID, l.Role, l.Name, l.Key, l.Text, l.Kind, l.Window} {
		if !utf8.ValidString(value) || len(value) > 256 {
			return fmt.Errorf("locator strings must be valid UTF-8 and at most 256 bytes")
		}
	}
	branches := 0
	if l.TestID != "" {
		branches++
	}
	if l.Role != "" || l.Name != "" {
		branches++
		if l.Role == "" || l.Name == "" {
			return fmt.Errorf("role locator requires both role and name")
		}
	}
	if l.Key != "" {
		branches++
	}
	if l.Text != "" {
		branches++
	}
	if l.Kind != "" {
		branches++
	}
	if l.Coords != nil {
		branches++
		if math.IsNaN(float64(l.Coords[0])) || math.IsInf(float64(l.Coords[0]), 0) || math.IsNaN(float64(l.Coords[1])) || math.IsInf(float64(l.Coords[1]), 0) {
			return fmt.Errorf("coordinate locator values must be finite float32 values")
		}
		if l.Window != "" && l.Window != "primary" {
			return fmt.Errorf("coordinate locator window is unavailable")
		}
	}
	if branches != 1 {
		return fmt.Errorf("locator requires exactly one branch")
	}
	if l.Within != nil {
		return validate(*l.Within, depth+1)
	}
	return nil
}

// VisibleTree returns one canonical deterministic pre-order view. The
// callback must return only currently visible/materialized hosted children.
func VisibleTree(root facet.FacetImpl, describe DescribeFunc, children VisibleChildrenFunc) []Node {
	nodes, _ := VisibleTreeBounded(root, describe, children, int(^uint(0)>>1))
	return nodes
}

// VisibleTreeBounded is the bounded form used by cut capture. The boolean is
// true only when the live tree exceeded limit; callers must discard the
// partial result and report capacity exhaustion rather than expose a prefix
// as a complete tree.
func VisibleTreeBounded(root facet.FacetImpl, describe DescribeFunc, children VisibleChildrenFunc, limit int) ([]Node, bool) {
	if root == nil || root.Base() == nil {
		return nil, false
	}
	if limit < 1 {
		return nil, true
	}
	type pending struct {
		impl   facet.FacetImpl
		parent facet.FacetID
		depth  int
		path   string
		meta   Meta
	}
	rootMeta := Meta{}
	if describe != nil {
		rootMeta = describe(root)
	}
	rootPath := segment(root.Base(), rootMeta, 0, true)
	stack := []pending{{impl: root, depth: 0, path: rootPath, meta: rootMeta}}
	out := make([]Node, 0, 64)
	seen := make(map[facet.FacetID]struct{})
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current.impl == nil || current.impl.Base() == nil {
			continue
		}
		base := current.impl.Base()
		if base.State() == facet.StateDisposed {
			continue
		}
		if _, exists := seen[base.ID()]; exists {
			continue
		}
		seen[base.ID()] = struct{}{}
		if len(out) == limit {
			return nil, true
		}
		out = append(out, Node{Impl: current.impl, Base: base, Path: current.path, ParentID: current.parent, Depth: current.depth, Meta: current.meta})
		visible := make([]facet.FacetImpl, 0, len(base.Children()))
		for _, child := range base.Children() {
			if child == nil || child.State() == facet.StateDisposed {
				continue
			}
			impl := child.Impl()
			if impl == nil {
				impl = child
			}
			visible = append(visible, impl)
		}
		if children != nil {
			visible = append(visible, children(current.impl)...)
		}
		childNodes := make([]pending, 0, len(visible))
		for index, child := range visible {
			if child == nil || child.Base() == nil || child.Base().State() == facet.StateDisposed {
				continue
			}
			childMeta := Meta{}
			if describe != nil {
				childMeta = describe(child)
			}
			childNodes = append(childNodes, pending{impl: child, parent: base.ID(), depth: current.depth + 1, path: current.path + "/" + segment(child.Base(), childMeta, index, false), meta: childMeta})
		}
		for i := len(childNodes) - 1; i >= 0; i-- {
			stack = append(stack, childNodes[i])
		}
	}
	return out, false
}

func segment(base *facet.Facet, meta Meta, index int, root bool) string {
	if base != nil && base.TestID() != "" {
		return base.TestID()
	}
	kind := meta.Kind
	if kind == "" {
		kind = "facet"
	}
	if meta.Key != "" {
		return kind + "[key=" + url.PathEscape(meta.Key) + "]"
	}
	if root {
		return kind
	}
	return kind + "[" + strconv.Itoa(index) + "]"
}

func Resolve(root facet.FacetImpl, locator Locator, describe DescribeFunc, hitTest HitTestFunc, children VisibleChildrenFunc) (Match, []Candidate, *Error) {
	if err := Validate(locator); err != nil {
		return Match{}, nil, &Error{Code: "protocol.invalid_params", Detail: err.Error()}
	}
	if locator.Coords != nil {
		if hitTest == nil {
			return Match{}, nil, &Error{Code: "locator.unresolved", Detail: "coords resolution requires a projected hit map"}
		}
		impl, ok := hitTest(gfx.Point{X: locator.Coords[0], Y: locator.Coords[1]})
		if !ok || impl == nil || impl.Base() == nil || impl.Base().State() == facet.StateDisposed {
			return Match{}, nil, ErrUnresolved(locator)
		}
		nodes := VisibleTree(root, describe, children)
		var scope facet.FacetID
		if locator.Within != nil {
			parents := make(map[facet.FacetID]facet.FacetID, len(nodes))
			for _, node := range nodes {
				parents[node.Base.ID()] = node.ParentID
			}
			var scopeErr *Error
			scope, scopeErr = resolveScope(nodes, *locator.Within, parents)
			if scopeErr != nil {
				return Match{}, scopeErr.Candidates, scopeErr
			}
		}
		for _, n := range nodes {
			if n.Base.ID() == impl.Base().ID() {
				if scope != 0 {
					parents := make(map[facet.FacetID]facet.FacetID, len(nodes))
					for _, node := range nodes {
						parents[node.Base.ID()] = node.ParentID
					}
					if n.Base.ID() == scope || !isDescendant(parents, scope, n.Base.ID()) {
						return Match{}, nil, ErrUnresolved(locator)
					}
				}
				return Match{Impl: n.Impl, Base: n.Base, Path: n.Path}, nil, nil
			}
		}
		return Match{}, nil, ErrUnresolved(locator)
	}
	nodes := VisibleTree(root, describe, children)
	parents := make(map[facet.FacetID]facet.FacetID, len(nodes))
	for _, node := range nodes {
		parents[node.Base.ID()] = node.ParentID
	}
	var scope facet.FacetID
	if locator.Within != nil {
		var scopeErr *Error
		scope, scopeErr = resolveScope(nodes, *locator.Within, parents)
		if scopeErr != nil {
			return Match{}, scopeErr.Candidates, scopeErr
		}
	}
	var found []Node
	for _, node := range nodes {
		if scope != 0 && !isDescendant(parents, scope, node.Base.ID()) {
			continue
		}
		if scope != 0 && node.Base.ID() == scope {
			continue
		}
		if matches(locator, node) {
			found = append(found, node)
		}
	}
	if len(found) == 0 {
		return Match{}, nil, ErrUnresolved(locator)
	}
	if len(found) > 1 {
		limit := min(10, len(found))
		candidates := make([]Candidate, 0, limit)
		for _, node := range found[:limit] {
			candidates = append(candidates, Candidate{FacetPath: node.Path, TestID: node.Base.TestID(), Role: node.Meta.Role, Name: node.Meta.Name, Kind: node.Meta.Kind})
		}
		return Match{}, candidates, ErrAmbiguous(locator, candidates, len(found) > limit)
	}
	node := found[0]
	return Match{Impl: node.Impl, Base: node.Base, Path: node.Path}, nil, nil
}

func resolveScope(nodes []Node, locator Locator, parents map[facet.FacetID]facet.FacetID) (facet.FacetID, *Error) {
	var parentScope facet.FacetID
	if locator.Within != nil {
		var err *Error
		parentScope, err = resolveScope(nodes, *locator.Within, parents)
		if err != nil {
			return 0, err
		}
	}
	var found facet.FacetID
	var candidates []Candidate
	count := 0
	for _, node := range nodes {
		if parentScope != 0 && !isDescendant(parents, parentScope, node.Base.ID()) && node.Base.ID() != parentScope {
			continue
		}
		if matches(locator, node) {
			count++
			if found != 0 {
				if len(candidates) < maxCandidates {
					candidates = append(candidates, Candidate{FacetPath: node.Path, TestID: node.Base.TestID(), Role: node.Meta.Role, Name: node.Meta.Name, Kind: node.Meta.Kind})
				}
				continue
			}
			found = node.Base.ID()
			candidates = append(candidates, Candidate{FacetPath: node.Path, TestID: node.Base.TestID(), Role: node.Meta.Role, Name: node.Meta.Name, Kind: node.Meta.Kind})
		}
	}
	if found == 0 {
		return 0, ErrUnresolved(locator)
	}
	if count > 1 {
		return 0, ErrAmbiguous(locator, candidates, count > maxCandidates)
	}
	return found, nil
}

func isDescendant(parents map[facet.FacetID]facet.FacetID, ancestor, candidate facet.FacetID) bool {
	for id := candidate; id != 0; id = parents[id] {
		if parents[id] == ancestor {
			return true
		}
	}
	return false
}

func matches(locator Locator, node Node) bool {
	switch {
	case locator.TestID != "":
		return node.Base.TestID() == locator.TestID
	case locator.Role != "":
		return node.Meta.Role == locator.Role && node.Meta.Name == locator.Name
	case locator.Key != "":
		return node.Meta.Key == locator.Key
	case locator.Text != "":
		return node.Meta.Name == locator.Text
	case locator.Kind != "":
		return node.Meta.Kind == locator.Kind
	default:
		return false
	}
}

// PathFor is a single-facet diagnostic fallback. Tree paths should come from
// VisibleTree so sibling identity is included.
func PathFor(base *facet.Facet) string {
	if base == nil {
		return "facet"
	}
	if id := base.TestID(); id != "" {
		return id
	}
	return fmt.Sprintf("facet#%d", base.ID())
}
