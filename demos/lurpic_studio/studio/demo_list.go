package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

// demoList is a bespoke vertical scroll host for the E6 playground (Finding
// F-scroll-content). The standard scroll_region draws its content children by
// self-projection without attaching them to the facet tree, so content inside a
// scroll_region is never hit-testable by the runtime — interactive marks cannot
// live in one. This host attaches its cards as real facet-tree children (so
// they are projected and hit-tested like every other demo host's children) and
// scrolls them with the wheel/arrow keys, clipping only by arranging cards
// inside the viewport. It follows the E1 grid's row-visibility pattern.
type demoList struct {
	facet.Facet
	layout facet.LayoutRole
	hit    facet.HitRole
	input  facet.InputRole

	items  []facet.FacetImpl //lurpiclint:ignore LL012 -- the hosted card facets are composition structure, not domain state (F-lint-hosts)
	gap    float32
	scroll float32

	rt facet.RuntimeServices
}

// newDemoList builds a scrollable vertical host over the given card facets.
func newDemoList(gap float32, items ...facet.FacetImpl) *demoList {
	l := &demoList{gap: gap}
	l.items = append([]facet.FacetImpl(nil), items...)
	l.Facet = facet.NewFacet()
	for _, it := range l.items {
		if it != nil && it.Base() != nil {
			l.AddChild(it.Base()) //lurpiclint:ignore LL021 -- E6 hosts playground cards as regular children, not overlays (LL021 over-fires)
		}
	}

	l.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke scrollable list host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return l.measure(ctx, c)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			l.arrange(ctx, bounds)
		},
	}
	l.layout.Child = linearChildContract(facet.StretchPolicy{
		Width:  facet.StretchAlways,
		Height: facet.StretchAlways,
	})
	l.hit = facet.HitRole{
		OnHitTest: func(p gfx.Point) facet.HitResult {
			b := l.layout.ArrangedBounds
			if b.IsEmpty() || !b.Contains(p) {
				return facet.HitResult{}
			}
			return facet.HitResult{Hit: true}
		},
	}
	l.input = facet.InputRole{
		OnScroll: func(e facet.ScrollEvent) bool { return l.onScroll(e) },
		OnKey:    func(e facet.KeyEvent) bool { return l.onKey(e) },
	}
	l.AddRole(&l.layout)
	l.AddRole(&l.hit)
	l.AddRole(&l.input)
	return l
}

// measure sizes the host to fill the available space; cards are measured with
// an unbounded height so each reports its content height (the Card idiom).
func (l *demoList) measure(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
	avail := gfx.Size{W: c.MaxSize.W}
	for _, it := range l.items {
		if it == nil || it.Base() == nil || it.Base().LayoutRole() == nil {
			continue
		}
		it.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: avail})
	}
	return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
}

// arrange places the visible cards vertically below the viewport top, offset by
// the scroll amount. Cards outside the viewport are arranged to zero bounds so
// they project and hit-test nothing.
func (l *demoList) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if bounds.IsEmpty() {
		for _, it := range l.items {
			if it != nil && it.Base() != nil && it.Base().LayoutRole() != nil {
				it.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
			}
		}
		return
	}
	total := l.contentHeight()
	maxScroll := total - bounds.Height()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if l.scroll > maxScroll {
		l.scroll = maxScroll
	}
	if l.scroll < 0 {
		l.scroll = 0
	}
	cursorY := bounds.Min.Y - l.scroll
	for _, it := range l.items {
		if it == nil || it.Base() == nil || it.Base().LayoutRole() == nil {
			continue
		}
		role := it.Base().LayoutRole()
		h := role.MeasuredSize.H
		if h <= 0 {
			h = role.MeasuredResult.Size.H
		}
		top := cursorY
		bottom := top + h
		cursorY = bottom + l.gap
		if bottom < bounds.Min.Y || top > bounds.Max.Y {
			role.Arrange(ctx, gfx.Rect{})
			continue
		}
		role.Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, top, bounds.Width(), h))
	}
}

// contentHeight returns the total stacked height of the items plus gaps.
func (l *demoList) contentHeight() float32 {
	total := float32(0)
	for _, it := range l.items {
		if it == nil || it.Base() == nil || it.Base().LayoutRole() == nil {
			continue
		}
		role := it.Base().LayoutRole()
		h := role.MeasuredSize.H
		if h <= 0 {
			h = role.MeasuredResult.Size.H
		}
		if h > 0 {
			total += h + l.gap
		}
	}
	if total > 0 && l.gap > 0 {
		total -= l.gap
	}
	return total
}

// scrollHeight returns the maximum scrollable extent for the current bounds.
func (l *demoList) onScroll(e facet.ScrollEvent) bool {
	if e.DeltaX == 0 && e.DeltaY == 0 {
		return false
	}
	next := l.scroll - e.DeltaY
	max := l.contentHeight() - l.layout.ArrangedBounds.Height()
	if max < 0 {
		max = 0
	}
	if next > max {
		next = max
	}
	if next < 0 {
		next = 0
	}
	if next == l.scroll {
		return false
	}
	l.scroll = next
	invalidateLayout(l, l.rt, "demoList.onScroll")
	return true
}

func (l *demoList) onKey(e facet.KeyEvent) bool {
	if e.Kind != platform.KeyPress && e.Kind != platform.KeyRepeat {
		return false
	}
	var delta float32
	switch e.Key {
	case platform.KeyUp:
		delta = -48
	case platform.KeyDown:
		delta = 48
	case platform.KeyPageUp:
		delta = -l.layout.ArrangedBounds.Height()
	case platform.KeyPageDown:
		delta = l.layout.ArrangedBounds.Height()
	default:
		return false
	}
	next := l.scroll + delta
	max := l.contentHeight() - l.layout.ArrangedBounds.Height()
	if max < 0 {
		max = 0
	}
	if next > max {
		next = max
	}
	if next < 0 {
		next = 0
	}
	if next == l.scroll {
		return false
	}
	l.scroll = next
	invalidateLayout(l, l.rt, "demoList.onKey")
	return true
}

// Items returns the hosted cards.
func (l *demoList) Items() []facet.FacetImpl { return append([]facet.FacetImpl(nil), l.items...) }

// ScrollOffset returns the current pixel scroll offset.
func (l *demoList) ScrollOffset() float32 { return l.scroll }

func (l *demoList) Base() *facet.Facet { l.BindImpl(l); return &l.Facet }
func (l *demoList) OnAttach(ctx facet.AttachContext) {
	l.rt = ctx.Runtime
}
func (l *demoList) OnDetach()     {}
func (l *demoList) OnActivate()   {}
func (l *demoList) OnDeactivate() {}
