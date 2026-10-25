package marks

import "codeburg.org/lexbit/lurpicui/facet"

// HostedChildrenFor is the single capability probe for mark-owned visible
// children. Runtime and observation consumers use this adapter instead of
// declaring local structural interfaces.
func HostedChildrenFor(impl facet.FacetImpl) []facet.FacetImpl {
	if impl == nil {
		return nil
	}
	if host, ok := impl.(HostsChildren); ok {
		return host.HostedChildren()
	}
	return nil
}
