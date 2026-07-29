package selection

import (
	"codeburg.org/lexbit/lurpicui/gfx"
)

// This file holds the option-rect cache value type for the DropdownSelect mark.
//
// It is deliberately declared in a file that contains NO facet-embedding type.
// Principle 1/8 (facets must not echo domain data without a version) is honored
// by keying this cache on the value store's Version plus the listbox bounds,
// option count, and scroll offset, re-deriving on any mismatch. Holding the
// derived rects in a named, version-keyed struct — rather than as a loose
// unversioned slice field on the facet — is what makes the staleness checkable.
//
// The cache is render state (P8): it is fully reconstructable from the value
// store, the Options binding, and the arranged listbox bounds.

// optionRectCache holds the arranged option rects keyed on the inputs that
// produced them. The scroll key closes the stale-rects-on-scroll gap: onScroll
// raises only DirtyProjection, so the rects must re-derive against the new
// scroll without waiting for a layout pass.
type optionRectCache struct {
	version uint64   // Value.Version() the rects were derived against
	bounds  gfx.Rect // cachedListboxBounds the rects were laid out within
	scroll  float32  // scrollOffset baked into the rects' Y positions
	count   int      // len(Options) the rects were laid out for
	rects   []gfx.Rect
}

// validFor reports whether the cache is fresh for the current inputs.
func (c *optionRectCache) validFor(v uint64, bounds gfx.Rect, scroll float32, count int) bool {
	return c.version == v && c.bounds == bounds && c.scroll == scroll && c.count == count
}
