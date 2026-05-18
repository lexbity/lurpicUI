package diagnostics

import (
	"fmt"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
)

// PanicContext captures enough state to format actionable contract-violation messages.
type PanicContext struct {
	FacetID    facet.FacetID
	MarkID     facet.MarkID
	LayerID    layout.LayerID
	LayerName  string
	LayerOrder int
	Placement  facet.PlacementMode
	HitPolicy  string
	ClipPolicy string
	Contract   string
	Guidance   string
}

// String formats the panic context as a single actionable message fragment.
func (c PanicContext) String() string {
	parts := make([]string, 0, 8)
	if c.FacetID != 0 {
		parts = append(parts, fmt.Sprintf("facet %d", c.FacetID))
	}
	if c.MarkID != 0 {
		parts = append(parts, fmt.Sprintf("mark %d", c.MarkID))
	}
	if c.LayerID != 0 {
		if c.LayerName != "" {
			parts = append(parts, fmt.Sprintf("layer %d (%s)", c.LayerID, c.LayerName))
		} else {
			parts = append(parts, fmt.Sprintf("layer %d", c.LayerID))
		}
	}
	if c.LayerOrder != 0 {
		parts = append(parts, fmt.Sprintf("order %d", c.LayerOrder))
	}
	if c.Placement != 0 {
		parts = append(parts, fmt.Sprintf("placement %d", c.Placement))
	}
	if c.HitPolicy != "" {
		parts = append(parts, "hit policy "+c.HitPolicy)
	}
	if c.ClipPolicy != "" {
		parts = append(parts, "clip policy "+c.ClipPolicy)
	}
	if c.Contract != "" && c.Contract != "layout contract violation" {
		parts = append(parts, c.Contract)
	}
	if c.Guidance != "" {
		parts = append(parts, c.Guidance)
	}
	return strings.Join(parts, "; ")
}

// ContractViolationMessage formats a full panic message for contract violations.
func ContractViolationMessage(ctx PanicContext) string {
	if ctx.Contract == "" || ctx.Contract == "layout contract violation" {
		ctx.Contract = "layout contract violation"
	}
	return ctx.Contract + ": " + ctx.String()
}
