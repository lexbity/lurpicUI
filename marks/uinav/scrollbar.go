package uinav

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uinav"
	"codeburg.org/lexbit/lurpicui/theme"
)

// ViewportBinding describes a scrollable viewport.
type ViewportBinding struct {
	Offset     store.Binding[float64]
	Extent     store.Binding[float64]
	ContentSize store.Binding[float64]
}

// Scrollbar is a scroll thumb control.
type Scrollbar struct {
	ID          string
	Orientation ScrollbarOrientation
	Viewport    ViewportBinding

	base         facet.Facet
	once         sync.Once
	state        controlState
	dragging     bool
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	inputRole    *facet.InputRole
}

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyUINav,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("uinav:scrollbar"),
		HitTestable:       true,
	})
}

func (s *Scrollbar) Base() *facet.Facet { s.ensureInit(); return &s.base }
func (s *Scrollbar) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUINav, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("uinav:scrollbar"), HitTestable: true}
}
func (s *Scrollbar) AuthoredID() string { return s.ID }
func (s *Scrollbar) OnAttach(ctx facet.AttachContext) { s.syncRoles() }
func (s *Scrollbar) OnDetach() {}
func (s *Scrollbar) OnActivate() {}
func (s *Scrollbar) OnDeactivate() {}

func (s *Scrollbar) ensureInit() {
	s.once.Do(func() {
		s.base.BindImpl(s)
		s.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			bounds := s.bounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		s.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		s.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return s.project(ctx) }}
		s.hitRole = &facet.HitRole{OnHitTest: func(pt gfx.Point) facet.HitResult {
			if s.bounds().Contains(pt) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
			}
			return facet.HitResult{}
		}}
		s.inputRole = &facet.InputRole{
			OnPointer: func(e facet.PointerEvent) bool { return s.handlePointer(e) },
		}
		s.base.AddRole(s.layoutRole)
		s.base.AddRole(s.viewportRole)
		s.base.AddRole(s.projection)
		s.base.AddRole(s.hitRole)
		s.base.AddRole(s.inputRole)
		s.syncRoles()
	})
}

func (s *Scrollbar) syncRoles() {
	syncLayout(s.layoutRole, s.bounds())
	syncViewport(s.viewportRole, gfx.Identity())
}

func (s *Scrollbar) bounds() gfx.Rect {
	if s.Orientation == ScrollbarVertical {
		return gfx.RectFromXYWH(0, 0, 12, 240)
	}
	return gfx.RectFromXYWH(0, 0, 240, 12)
}

func (s *Scrollbar) thumbRatio() float32 {
	extent := s.Viewport.Extent.Get()
	content := s.Viewport.ContentSize.Get()
	if extent <= 0 || content <= 0 {
		return 1
	}
	ratio := float32(extent / content)
	if ratio < 0.05 {
		return 0.05
	}
	if ratio > 1 {
		return 1
	}
	return ratio
}

func (s *Scrollbar) thumbRect() gfx.Rect {
	bounds := s.bounds()
	ratio := s.thumbRatio()
	if s.Orientation == ScrollbarVertical {
		h := bounds.Height() * ratio
		maxOffset := s.maxOffset()
		off := s.Viewport.Offset.Get()
		p := float32(0)
		if maxOffset > 0 {
			p = float32(clampFloat(off/maxOffset, 0, 1))
		}
		y := (bounds.Height() - h) * p
		return gfx.RectFromXYWH(0, y, bounds.Width(), h)
	}
	w := bounds.Width() * ratio
	maxOffset := s.maxOffset()
	off := s.Viewport.Offset.Get()
	p := float32(0)
	if maxOffset > 0 {
		p = float32(clampFloat(off/maxOffset, 0, 1))
	}
	x := (bounds.Width() - w) * p
	return gfx.RectFromXYWH(x, 0, w, bounds.Height())
}

func (s *Scrollbar) maxOffset() float64 {
	content := s.Viewport.ContentSize.Get()
	extent := s.Viewport.Extent.Get()
	if content <= extent {
		return 0
	}
	return content - extent
}

func (s *Scrollbar) handlePointer(e facet.PointerEvent) bool {
	if e.Kind != platform.PointerPress && e.Kind != platform.PointerMove && e.Kind != platform.PointerRelease {
		return false
	}
	bounds := s.bounds()
	thumb := s.thumbRect()
	switch e.Kind {
	case platform.PointerPress:
		if thumb.Contains(e.Position) {
			s.dragging = true
			return true
		}
		if bounds.Contains(e.Position) {
			s.pageByExtent(e.Position)
			return true
		}
	case platform.PointerMove:
		if s.dragging {
			s.dragTo(e.Position)
			return true
		}
	case platform.PointerRelease:
		if s.dragging {
			s.dragTo(e.Position)
			s.dragging = false
			return true
		}
	}
	return false
}

func (s *Scrollbar) dragTo(pos gfx.Point) {
	bounds := s.bounds()
	maxOffset := s.maxOffset()
	if maxOffset <= 0 {
		s.Viewport.Offset.Set(0)
		return
	}
	if s.Orientation == ScrollbarVertical {
		travel := bounds.Height() - s.thumbRect().Height()
		if travel <= 0 {
			s.Viewport.Offset.Set(0)
			return
		}
		y := clampFloat(float64(pos.Y)/float64(travel), 0, 1)
		s.Viewport.Offset.Set(y * maxOffset)
		return
	}
	travel := bounds.Width() - s.thumbRect().Width()
	if travel <= 0 {
		s.Viewport.Offset.Set(0)
		return
	}
	x := clampFloat(float64(pos.X)/float64(travel), 0, 1)
	s.Viewport.Offset.Set(x * maxOffset)
}

func (s *Scrollbar) pageByExtent(pos gfx.Point) {
	bounds := s.bounds()
	thumb := s.thumbRect()
	if s.Orientation == ScrollbarVertical {
		if pos.Y < thumb.Min.Y {
			s.Viewport.Offset.Set(clampFloat(s.Viewport.Offset.Get()-s.Viewport.Extent.Get(), 0, s.maxOffset()))
		} else {
			s.Viewport.Offset.Set(clampFloat(s.Viewport.Offset.Get()+s.Viewport.Extent.Get(), 0, s.maxOffset()))
		}
		return
	}
	if pos.X < thumb.Min.X {
		s.Viewport.Offset.Set(clampFloat(s.Viewport.Offset.Get()-s.Viewport.Extent.Get(), 0, s.maxOffset()))
	} else {
		s.Viewport.Offset.Set(clampFloat(s.Viewport.Offset.Get()+s.Viewport.Extent.Get(), 0, s.maxOffset()))
	}
	_ = bounds
}

func (s *Scrollbar) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveScrollbarRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	var list gfx.CommandList
	list.Add(gfx.FillRect{Rect: s.bounds(), Brush: gfx.SolidBrush(fillColor(slots.Track.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{R: 0.9, G: 0.9, B: 0.92, A: 1}))})
	list.Add(gfx.FillRect{Rect: s.thumbRect(), Brush: gfx.SolidBrush(fillColor(slots.Thumb.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{R: 0.25, G: 0.45, B: 0.95, A: 1}))})
	return &list
}
