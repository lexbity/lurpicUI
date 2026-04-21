package uiinput

import (
	"math"
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
	"codeburg.org/lexbit/lurpicui/theme"
)

// SliderOrientation controls the slider axis.
type SliderOrientation uint8

const (
	SliderHorizontal SliderOrientation = iota
	SliderVertical
)

// SliderMode controls how values are interpreted.
type SliderMode uint8

const (
	SliderContinuous SliderMode = iota
	SliderDiscrete
	SliderRange
	SliderRestricted
)

// Slider is a value control with one or two thumbs.
type Slider struct {
	ID          string
	Orientation SliderOrientation
	Mode        SliderMode
	Value       store.Binding[float64]
	Range       *store.Binding[[2]float64]
	Min         float64
	Max         float64
	Step        float64
	Allowed     []float64
	Disabled    bool

	base         facet.Facet
	once         sync.Once
	state        controlState
	dragging     bool
	activeThumb  int
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	inputRole    *facet.InputRole
	focusRole    *facet.FocusRole
}

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyUIInput,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("uiinput:slider"),
		Focusable:         true,
		HitTestable:       true,
	})
}

func (s *Slider) Base() *facet.Facet { s.ensureInit(); return &s.base }
func (s *Slider) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUIInput, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("uiinput:slider"), Focusable: true, HitTestable: true}
}
func (s *Slider) AuthoredID() string { return s.ID }
func (s *Slider) OnAttach(ctx facet.AttachContext) { s.syncRoles() }
func (s *Slider) OnDetach() {}
func (s *Slider) OnActivate() {}
func (s *Slider) OnDeactivate() {}

func (s *Slider) ensureInit() {
	s.once.Do(func() {
		s.base.BindImpl(s)
		s.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			b := s.bounds()
			return gfx.Size{W: b.Width(), H: b.Height()}
		}}
		s.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		s.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return s.project(ctx) }}
		s.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if s.bounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
			}
			return facet.HitResult{}
		}}
		s.inputRole = &facet.InputRole{
			OnPointer: func(e facet.PointerEvent) bool { return s.handlePointer(e) },
			OnKey:     func(e facet.KeyEvent) bool { return s.handleKey(e) },
		}
		s.focusRole = &facet.FocusRole{
			Focusable: func() bool { return !s.Disabled },
			OnFocusGained: func() { s.state.focused = true },
			OnFocusLost:   func() { s.state.focused = false; s.dragging = false },
		}
		s.base.AddRole(s.layoutRole)
		s.base.AddRole(s.viewportRole)
		s.base.AddRole(s.projection)
		s.base.AddRole(s.hitRole)
		s.base.AddRole(s.inputRole)
		s.base.AddRole(s.focusRole)
		s.syncRoles()
	})
}

func (s *Slider) syncRoles() {
	s.state.disabled = s.Disabled
}

func (s *Slider) bounds() gfx.Rect {
	if s.Orientation == SliderVertical {
		return gfx.RectFromXYWH(0, 0, 28, 200)
	}
	return gfx.RectFromXYWH(0, 0, 240, 28)
}

func (s *Slider) handlePointer(e facet.PointerEvent) bool {
	if s.Disabled {
		return false
	}
	switch e.Kind {
	case platform.PointerPress:
		s.dragging = true
		s.activeThumb = s.chooseThumb(e.Position)
		s.applyPointer(e.Position)
		return true
	case platform.PointerMove:
		if s.dragging {
			s.applyPointer(e.Position)
			return true
		}
	case platform.PointerRelease:
		if s.dragging {
			s.applyPointer(e.Position)
			s.dragging = false
			return true
		}
	}
	return false
}

func (s *Slider) handleKey(e facet.KeyEvent) bool {
	if s.Disabled || !s.state.focused || e.Kind != platform.KeyPress {
		return false
	}
	step := s.effectiveStep()
	switch e.Key {
	case platform.KeyLeft, platform.KeyDown:
		s.setPrimaryValue(s.primaryValue() - step)
		return true
	case platform.KeyRight, platform.KeyUp:
		s.setPrimaryValue(s.primaryValue() + step)
		return true
	case platform.KeyHome:
		s.setPrimaryValue(s.Min)
		return true
	case platform.KeyEnd:
		s.setPrimaryValue(s.Max)
		return true
	}
	return false
}

func (s *Slider) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveSliderRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, uirecipe.SliderStandard)
	var list gfx.CommandList
	bounds := s.bounds()
	state := s.state.interactionState()
	track := slots.Track.Resolve(state, theme.DefaultTokens())
	fill := slots.Fill.Resolve(state, theme.DefaultTokens())
	thumbStyle := slots.Thumb.Resolve(state, theme.DefaultTokens())
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(fillColor(track, gfx.Color{R: 0.82, G: 0.82, B: 0.84, A: 1}))})
	if s.Mode == SliderRange && s.Range != nil {
		vals := s.Range.Get()
		s.normRange(&vals)
		r0 := s.valueRect(vals[0], bounds)
		r1 := s.valueRect(vals[1], bounds)
		if s.Orientation == SliderVertical {
			list.Add(gfx.FillRect{Rect: gfx.RectFromXYWH(bounds.Min.X, r1.Min.Y, bounds.Width(), r0.Max.Y-r1.Min.Y), Brush: gfx.SolidBrush(fillColor(fill, gfx.Color{R: 0.3, G: 0.5, B: 1, A: 1}))})
		} else {
			list.Add(gfx.FillRect{Rect: gfx.RectFromXYWH(r0.Max.X, bounds.Min.Y, r1.Min.X-r0.Max.X, bounds.Height()), Brush: gfx.SolidBrush(fillColor(fill, gfx.Color{R: 0.3, G: 0.5, B: 1, A: 1}))})
		}
		list.Add(gfx.FillRect{Rect: s.thumbRect(vals[0], bounds), Brush: gfx.SolidBrush(fillColor(thumbStyle, gfx.Color{R: 0.25, G: 0.45, B: 0.95, A: 1}))})
		list.Add(gfx.FillRect{Rect: s.thumbRect(vals[1], bounds), Brush: gfx.SolidBrush(fillColor(thumbStyle, gfx.Color{R: 0.25, G: 0.45, B: 0.95, A: 1}))})
	} else {
		value := s.primaryValue()
		tr := s.valueRect(value, bounds)
		if s.Orientation == SliderVertical {
			list.Add(gfx.FillRect{Rect: gfx.RectFromXYWH(bounds.Min.X, tr.Min.Y, bounds.Width(), bounds.Max.Y-tr.Min.Y), Brush: gfx.SolidBrush(fillColor(fill, gfx.Color{R: 0.3, G: 0.5, B: 1, A: 1}))})
		} else {
			list.Add(gfx.FillRect{Rect: gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, tr.Min.X-bounds.Min.X, bounds.Height()), Brush: gfx.SolidBrush(fillColor(fill, gfx.Color{R: 0.3, G: 0.5, B: 1, A: 1}))})
		}
		list.Add(gfx.FillRect{Rect: s.thumbRect(value, bounds), Brush: gfx.SolidBrush(fillColor(thumbStyle, gfx.Color{R: 0.25, G: 0.45, B: 0.95, A: 1}))})
	}
	if s.state.focused {
		focus := slots.FocusRing.Resolve(theme.StateFocused, theme.DefaultTokens())
		if len(focus.Strokes) > 0 {
			list.Add(gfx.StrokeRect{Rect: bounds.Inset(-2, -2), Stroke: strokeStyle(focus.Strokes[0]), Brush: gfx.SolidBrush(strokeColor(focus, gfx.Color{R: 0.3, G: 0.5, B: 1, A: 1}))})
		}
	}
	return &list
}

func (s *Slider) effectiveStep() float64 {
	if s.Step > 0 {
		return s.Step
	}
	if len(s.Allowed) > 1 {
		return (s.Max - s.Min) / float64(len(s.Allowed)-1)
	}
	return (s.Max - s.Min) / 10
}

func (s *Slider) primaryValue() float64 {
	if s.Mode == SliderRange && s.Range != nil {
		vals := s.Range.Get()
		s.normRange(&vals)
		return vals[s.activeThumb]
	}
	return s.Value.Get()
}

func (s *Slider) setPrimaryValue(v float64) {
	v = s.clampAndSnap(v)
	if s.Mode == SliderRange && s.Range != nil {
		vals := s.Range.Get()
		s.normRange(&vals)
		vals[s.activeThumb] = v
		s.normRange(&vals)
		s.Range.Set(vals)
		return
	}
	s.Value.Set(v)
}

func (s *Slider) chooseThumb(pos gfx.Point) int {
	if s.Mode != SliderRange || s.Range == nil {
		return 0
	}
	vals := s.Range.Get()
	s.normRange(&vals)
	left := s.thumbCenter(vals[0], s.bounds())
	right := s.thumbCenter(vals[1], s.bounds())
	if distanceSquared(pos, left) <= distanceSquared(pos, right) {
		return 0
	}
	return 1
}

func (s *Slider) applyPointer(pos gfx.Point) {
	if s.Mode == SliderRange && s.Range != nil {
		vals := s.Range.Get()
		s.normRange(&vals)
		vals[s.activeThumb] = s.valueFromPoint(pos)
		s.normRange(&vals)
		s.Range.Set(vals)
		return
	}
	s.Value.Set(s.valueFromPoint(pos))
}

func (s *Slider) valueFromPoint(pos gfx.Point) float64 {
	bounds := s.bounds()
	if s.Orientation == SliderVertical {
		t := 1 - clamp01(float64((pos.Y-bounds.Min.Y)/bounds.Height()))
		return s.clampAndSnap(s.Min + t*(s.Max-s.Min))
	}
	t := clamp01(float64((pos.X - bounds.Min.X) / bounds.Width()))
	return s.clampAndSnap(s.Min + t*(s.Max-s.Min))
}

func (s *Slider) valueRect(value float64, bounds gfx.Rect) gfx.Rect {
	t := float32((s.clampAndSnap(value) - s.Min) / (s.Max - s.Min))
	if s.Orientation == SliderVertical {
		y := bounds.Max.Y - t*bounds.Height()
		return gfx.RectFromXYWH(bounds.Min.X, y, bounds.Width(), 0)
	}
	x := bounds.Min.X + t*bounds.Width()
	return gfx.RectFromXYWH(x, bounds.Min.Y, 0, bounds.Height())
}

func (s *Slider) thumbCenter(value float64, bounds gfx.Rect) gfx.Point {
	tr := s.thumbRect(value, bounds)
	return gfx.Point{X: (tr.Min.X + tr.Max.X) / 2, Y: (tr.Min.Y + tr.Max.Y) / 2}
}

func (s *Slider) thumbRect(value float64, bounds gfx.Rect) gfx.Rect {
	const size = 14
	if s.Orientation == SliderVertical {
		t := float32((s.clampAndSnap(value) - s.Min) / (s.Max - s.Min))
		y := bounds.Max.Y - t*bounds.Height()
		return gfx.RectFromXYWH(bounds.Min.X+(bounds.Width()-size)/2, y-size/2, size, size)
	}
	t := float32((s.clampAndSnap(value) - s.Min) / (s.Max - s.Min))
	x := bounds.Min.X + t*bounds.Width()
	return gfx.RectFromXYWH(x-size/2, bounds.Min.Y+(bounds.Height()-size)/2, size, size)
}

func (s *Slider) clampAndSnap(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return s.Min
	}
	if s.Max < s.Min {
		s.Min, s.Max = s.Max, s.Min
	}
	if v < s.Min {
		v = s.Min
	}
	if v > s.Max {
		v = s.Max
	}
	switch s.Mode {
	case SliderDiscrete:
		step := s.effectiveStep()
		if step > 0 {
			v = math.Round((v-s.Min)/step)*step + s.Min
		}
	case SliderRestricted:
		if len(s.Allowed) == 0 {
			return v
		}
		best := s.Allowed[0]
		bestDist := math.Abs(v - best)
		for _, cand := range s.Allowed[1:] {
			if dist := math.Abs(v - cand); dist < bestDist {
				best = cand
				bestDist = dist
			}
		}
		v = best
	}
	return v
}

func (s *Slider) normRange(vals *[2]float64) {
	if vals == nil {
		return
	}
	vals[0] = s.clampAndSnap(vals[0])
	vals[1] = s.clampAndSnap(vals[1])
	if vals[0] > vals[1] {
		vals[0], vals[1] = vals[1], vals[0]
	}
}

func distanceSquared(a, b gfx.Point) float32 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return dx*dx + dy*dy
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
