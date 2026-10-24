// Package locate implements the LDP locator grammar and resolution (§3.4):
// a pre-order walk of the live tree (the same order as the dirty-snapshot
// walk) matching author TestIDs, accessibility role/name, collection keys,
// descriptor kinds, or — last resort — hit-test coordinates.
//
// The package declares its view of mark metadata consumer-side (Principle
// 10): Describe is supplied by the caller, which adapts the marks package's
// canonical interfaces (Accessible, KeyedItem, Descriptor). This keeps
// locate free of the marks import (marks → runtime already exists).
package locate

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// Locator addresses facets without coordinates on the common path (§3.4).
// Exactly one field is honored: TestID, Role+Name, Key, Text, Kind, Coords —
// in that precedence order. Text matches AccessibleName only: canvas glyph
// runs have no text semantics and are deliberately unaddressable (interactive
// hosts MUST expose accessible names — the a11y pressure made explicit).
type Locator struct {
	TestID string      `json:"testid,omitempty"`
	Role   string      `json:"role,omitempty"`   // accessibility role
	Name   string      `json:"name,omitempty"`   // accessible name (with Role)
	Key    string      `json:"key,omitempty"`    // KeyedItem.ItemKey()
	Text   string      `json:"text,omitempty"`   // matches AccessibleName only
	Kind   string      `json:"kind,omitempty"`   // "family/typeName"
	Coords *[2]float32 `json:"coords,omitempty"` // hit-test fallback
}

// Candidate describes one matching facet for the ambiguity report.
type Candidate struct {
	FacetPath string `json:"path"`
	TestID    string `json:"testid,omitempty"`
	Role      string `json:"role,omitempty"`
	Name      string `json:"name,omitempty"`
	Kind      string `json:"kind,omitempty"`
}

// Match is a resolved locator: the facet, its implementation, and its path
// from the root.
type Match struct {
	Impl facet.FacetImpl
	Base *facet.Facet
	Path string
}

// Meta is the per-facet mark metadata the matcher consumes, produced by the
// caller's Describe adapter.
type Meta struct {
	Kind string // "family/typeName"
	Role string
	Name string
	Key  string
}

// DescribeFunc adapts a facet implementation to the matcher's Meta view.
type DescribeFunc func(impl facet.FacetImpl) Meta

// HitTestFunc resolves screen coordinates to a facet (the Coords fallback).
type HitTestFunc func(pt gfx.Point) (facet.FacetImpl, bool)

// maxCandidates bounds the ambiguity report (§3.4: up to 10 candidates).
const maxCandidates = 10

// Error is a typed locator failure: unresolved (zero matches) or ambiguous
// (more than one), carrying candidates in the ambiguous case.
type Error struct {
	Code       string // "locator.unresolved" | "locator.ambiguous"
	Detail     string
	Candidates []Candidate
}

func (e *Error) Error() string { return e.Code + ": " + e.Detail }

// ErrUnresolved constructs the zero-match error.
func ErrUnresolved(l Locator) *Error {
	return &Error{Code: "locator.unresolved", Detail: fmt.Sprintf("no facet matches %+v", l)}
}

// ErrAmbiguous constructs the multi-match error with candidates.
func ErrAmbiguous(l Locator, cands []Candidate) *Error {
	return &Error{
		Code:       "locator.ambiguous",
		Detail:     fmt.Sprintf("%d facets match %+v", len(cands), l),
		Candidates: cands,
	}
}

// node is one walked facet with its accumulated path and metadata.
type node struct {
	base *facet.Facet
	path string
	meta Meta
}

// Resolve walks the live tree pre-order and returns exactly one match or a
// typed error. describe adapts each facet's mark; hitTest serves the Coords
// fallback (a hit, not a search — bypasses ambiguity).
func Resolve(root facet.FacetImpl, l Locator, describe DescribeFunc, hitTest HitTestFunc) (Match, []Candidate, *Error) {
	if root == nil || root.Base() == nil {
		return Match{}, nil, ErrUnresolved(l)
	}
	if l.Coords != nil {
		return resolveByCoords(l, hitTest)
	}

	// Pre-order walk (explicit stack, children visited in declaration order).
	stack := []node{{base: root.Base(), path: PathFor(root.Base())}}
	var matches []node
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n.base == nil || n.base.State() == facet.StateDisposed {
			continue
		}
		if describe != nil {
			n.meta = describe(n.base.Impl())
		}
		if locatorMatches(l, n) {
			matches = append(matches, n)
			if len(matches) > maxCandidates {
				cands := candidatesOf(matches[:maxCandidates])
				return Match{}, cands, ErrAmbiguous(l, cands)
			}
		}
		children := n.base.Children()
		for i := len(children) - 1; i >= 0; i-- {
			child := children[i]
			if child == nil {
				continue
			}
			stack = append(stack, node{
				base: child,
				path: n.path + "/" + PathFor(child),
			})
		}
		// Hosted (virtually-projected) children: group marks host rows the
		// facet tree never sees — walk them so keyed/TestID addressing
		// reaches every visible row.
		if impl := n.base.Impl(); impl != nil {
			if host, ok := impl.(interface{ HostedChildren() []facet.FacetImpl }); ok {
				hosted := host.HostedChildren()
				for i := len(hosted) - 1; i >= 0; i-- {
					h := hosted[i]
					if h == nil || h.Base() == nil {
						continue
					}
					stack = append(stack, node{
						base: h.Base(),
						path: n.path + "/" + PathFor(h.Base()),
					})
				}
			}
		}
	}
	switch len(matches) {
	case 0:
		return Match{}, nil, ErrUnresolved(l)
	case 1:
		m := matches[0]
		return Match{Impl: m.base.Impl(), Base: m.base, Path: m.path}, nil, nil
	default:
		cands := candidatesOf(matches)
		return Match{}, cands, ErrAmbiguous(l, cands)
	}
}

func resolveByCoords(l Locator, hitTest HitTestFunc) (Match, []Candidate, *Error) {
	if hitTest == nil {
		return Match{}, nil, &Error{Code: "locator.unresolved", Detail: "coords resolution requires a hit map (no frame projected yet)"}
	}
	pt := gfx.Point{X: l.Coords[0], Y: l.Coords[1]}
	impl, ok := hitTest(pt)
	if !ok || impl == nil || impl.Base() == nil {
		return Match{}, nil, ErrUnresolved(l)
	}
	return Match{Impl: impl, Base: impl.Base(), Path: PathFor(impl.Base())}, nil, nil
}

// PathFor renders one path segment for a facet: its TestID when set, else
// the fallback. Kind segments come from the caller's Describe adapter —
// this helper stays TestID-first by contract.
func PathFor(f *facet.Facet) string {
	if f == nil {
		return "facet"
	}
	if id := f.TestID(); id != "" {
		return id
	}
	return "facet"
}

func locatorMatches(l Locator, n node) bool {
	switch {
	case l.TestID != "":
		return n.base.TestID() == l.TestID
	case l.Kind != "":
		return n.meta.Kind == l.Kind
	case l.Key != "":
		// Exact match only — keys never fall back to substring matching.
		return n.meta.Key != "" && n.meta.Key == l.Key
	case l.Role != "":
		// Role may be paired with a Name; a bare Role matches any name.
		if n.meta.Role != l.Role {
			return false
		}
		return l.Name == "" || n.meta.Name == l.Name
	case l.Text != "":
		// AccessibleName only, exact match.
		return n.meta.Name == l.Text
	default:
		return false
	}
}

func candidatesOf(matches []node) []Candidate {
	out := make([]Candidate, 0, len(matches))
	for _, m := range matches {
		out = append(out, Candidate{
			FacetPath: m.path,
			TestID:    m.base.TestID(),
			Role:      m.meta.Role,
			Name:      m.meta.Name,
			Kind:      m.meta.Kind,
		})
	}
	return out
}
