package runtime

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/layout"
	anchorpolicy "codeburg.org/lexbit/lurpicui/layout/anchor"
	freepolicy "codeburg.org/lexbit/lurpicui/layout/free"
	gridpolicy "codeburg.org/lexbit/lurpicui/layout/grid"
	projectedpolicy "codeburg.org/lexbit/lurpicui/layout/projected"
	splitpolicy "codeburg.org/lexbit/lurpicui/layout/split"
	stackpolicy "codeburg.org/lexbit/lurpicui/layout/stack"
)

// LayerComposer is implemented by facets that declare runtime layer specs.
type LayerComposer interface {
	OnLayerSpecs() []layout.LayerSpec
}

// PolicyRegistry maps placement modes to placement policies.
type PolicyRegistry struct {
	policies map[layout.PlacementMode]layout.PlacementPolicy
}

// DefaultRegistry returns the built-in placement policy registry.
func DefaultRegistry() *PolicyRegistry {
	return &PolicyRegistry{
		policies: map[layout.PlacementMode]layout.PlacementPolicy{
			layout.PlacementStack:     stackpolicy.New(stackpolicy.Config{}),
			layout.PlacementSplit:     splitpolicy.New(splitpolicy.Config{}),
			layout.PlacementGrid:      gridpolicy.New(gridpolicy.Config{}),
			layout.PlacementFree:      freepolicy.New(),
			layout.PlacementAnchor:    anchorpolicy.New(),
			layout.PlacementProjected: projectedpolicy.New(),
		},
	}
}

func (r *PolicyRegistry) Policy(mode layout.PlacementMode) (layout.PlacementPolicy, bool) {
	if r == nil || len(r.policies) == 0 {
		return nil, false
	}
	p, ok := r.policies[mode]
	return p, ok
}

func (r *PolicyRegistry) MustPolicy(mode layout.PlacementMode) layout.PlacementPolicy {
	p, ok := r.Policy(mode)
	if !ok || p == nil {
		panic(fmt.Sprintf("runtime: no placement policy registered for mode %d", mode))
	}
	return p
}

type resolvedLayerSet struct {
	specs       []layout.LayerSpec
	layers      []layout.ResolvedLayer
	childCounts []int
	dirty       bool
}

func (s *resolvedLayerSet) resolvedLayer(id layout.LayerID) (layout.ResolvedLayer, bool) {
	if s == nil {
		return layout.ResolvedLayer{}, false
	}
	for i := range s.specs {
		if s.specs[i].ID == id {
			if i < len(s.layers) {
				return s.layers[i], true
			}
			return layout.ResolvedLayer{}, false
		}
	}
	return layout.ResolvedLayer{}, false
}
