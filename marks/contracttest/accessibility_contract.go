package contracttest

import (
	"codeburg.org/lexbit/lurpicui/facet"
)

// AssertAccessible proves that a marks.Accessible mark:
//  1. implements AccessibilityRole() and AccessibleName(),
//  2. reports a non-empty AccessibilityRole,
//  3. if roleExpected is non-empty, the reported role matches it,
//  4. AccessibleName() returns the label passed to build.
//
// build: construct the mark, wiring label as the accessible name source
//
//	(e.g. NewList(label, ...)).
//
// roleExpected: the canonical role string the mark must report, or "" to
//
//	skip the role match assertion.
func AssertAccessible(
	t TB,
	build func(label string) facet.FacetImpl,
	roleExpected string,
) {
	t.Helper()
	ctx := facet.AttachContext{Runtime: contractRuntime{}}

	m := build("catalog-row")
	facet.Attach(m, ctx)
	defer facet.Dispose(m)

	acc, ok := m.(interface {
		AccessibilityRole() string
		AccessibleName() string
	})
	if !ok {
		t.Fatalf("AssertAccessible: mark does not implement marks.Accessible")
	}

	if acc.AccessibilityRole() == "" {
		t.Fatalf("AssertAccessible: AccessibilityRole is empty")
	}
	if roleExpected != "" && acc.AccessibilityRole() != roleExpected {
		t.Fatalf("AssertAccessible: AccessibilityRole=%q, want %q",
			acc.AccessibilityRole(), roleExpected)
	}
	if acc.AccessibleName() != "catalog-row" {
		t.Fatalf("AssertAccessible: AccessibleName=%q, want %q (label not propagated)",
			acc.AccessibleName(), "catalog-row")
	}
}

// AssertFocusable proves that a marks.Focusable mark:
//  1. Focusable() returns true when enabled, false when disabled,
//  2. OnFocusGained raises DirtyProjection on the facet.
//
// build: construct the mark with the given disabled state
//
//	(e.g. assign marks.Const(disabled) to the mark's Disabled binding).
func AssertFocusable(
	t TB,
	build func(disabled bool) facet.FacetImpl,
) {
	t.Helper()

	// (1) Enabled: Focusable() must be true.
	ctx := facet.AttachContext{Runtime: contractRuntime{}}

	m := build(false)
	facet.Attach(m, ctx)
	defer facet.Dispose(m)

	f, ok := m.(interface{ Focusable() bool })
	if !ok {
		t.Fatalf("AssertFocusable: mark does not implement marks.Focusable")
	}
	if !f.Focusable() {
		t.Fatalf("AssertFocusable: Focusable()==false when enabled")
	}

	// (2) Disabled: Focusable() must be false.
	m2 := build(true)
	facet.Attach(m2, ctx)
	defer facet.Dispose(m2)

	f2 := m2.(interface{ Focusable() bool })
	if f2.Focusable() {
		t.Fatalf("AssertFocusable: Focusable()==true when disabled")
	}

	// (3) Focus round-trip: OnFocusGained must raise DirtyProjection.
	// Trigger focus through the FocusRole, which is how marks that declare
	// a FocusRole.Focusable callback wire their focus handler.
	focusRole := m.Base().FocusRole()
	if focusRole == nil || focusRole.OnFocusGained == nil {
		return // focus role not configured; skip round-trip check
	}

	m.Base().ClearDirty(facet.DirtyAll)
	focusRole.OnFocusGained()
	if m.Base().DirtyFlags()&facet.DirtyProjection == 0 {
		t.Fatalf("AssertFocusable: OnFocusGained did not raise DirtyProjection")
	}
}
