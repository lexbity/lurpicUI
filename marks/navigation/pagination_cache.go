package navigation

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
)

// This file holds the projection-cache value types for the Pagination mark.
//
// They are deliberately declared in a file that contains NO facet-embedding
// type: Principle 1/8 (facets must not echo domain data without a version) is
// honored by keying these caches on a domain version (CurrentIndex.Version)
// plus an Items hash, and re-deriving on mismatch. Holding the derived
// artifacts in a named, version-keyed struct — rather than as loose unversioned
// slice fields on the facet — is what makes the staleness checkable.
//
// The cache types themselves are render/projection state (P8): they can be
// fully reconstructed from the domain (Items) and view (CurrentIndex) inputs.

// entryProjection holds the per-visible-entry projection artifacts derived
// during measure, keyed on the inputs that produced them.
type entryProjection struct {
	version      uint64 // CurrentIndex.Version() when derived
	itemsHash    uint64 // hash of Items (count + labels) when derived
	maxWidth     float32
	contentScale float32

	visibleChildren []*paginationChild
	entryBounds     []gfx.Rect
	entryLayouts    []*text.TextLayout
	entryStyles     []text.TextStyle
	entryKinds      []paginationChildKind
	entryIndices    []int
	entryLabels     []string
}

// validFor reports whether the cached projection is fresh for the given key.
func (e *entryProjection) validFor(version, itemsHash uint64, maxWidth, contentScale float32) bool {
	return e.version == version && e.itemsHash == itemsHash &&
		e.maxWidth == maxWidth && e.contentScale == contentScale && len(e.visibleChildren) > 0
}

// pageChildSet wraps the per-page child registry. The child pointers are
// interaction/render state (P8), not domain truth; wrapping them in a named
// type keeps the facet field shape clean.
type pageChildSet struct {
	entries []*paginationChild
}

func (s *pageChildSet) len() int {
	if s == nil {
		return 0
	}
	return len(s.entries)
}

func (s *pageChildSet) at(i int) *paginationChild {
	if s == nil || i < 0 || i >= len(s.entries) {
		return nil
	}
	return s.entries[i]
}

// hashItems computes a lightweight FNV-1a hash over the page item count and
// labels. Items is a plain slice with no Version(), so this hash detects label
// or count changes that would make the cached projection stale.
func hashItems(items []PaginationItem) uint64 {
	h := uint64(1469598103934665603)
	mix := func(v uint64) { h ^= v; h *= 1099511628211 }
	mix(uint64(len(items)))
	for _, item := range items {
		mix(uint64(len(item.Label)))
		for _, r := range item.Label {
			mix(uint64(r)) //nolint:gosec // integer overflow conversion; codepoint bits for hashing only
		}
	}
	return h
}
