package marks

import "codeburg.org/lexbit/lurpicui/facet"

// Mark is the unified authored contract.
// Every concrete mark type satisfies this interface.
// Most capability is optional and discovered by type assertion.
type Mark interface {
	facet.FacetImpl
	Descriptor() Descriptor
}
