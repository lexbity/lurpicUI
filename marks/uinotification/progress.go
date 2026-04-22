package uinotification

import (
	"math"
	"sync"
	"time"

	"codeburg.org/lexbit/lurpicui/animation"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uinotification"
)

type ProgressMode uint8

const (
	ProgressDeterminate ProgressMode = iota
	ProgressIndeterminate
)

type ProgressShape uint8

const (
	ProgressLinear ProgressShape = iota
	ProgressCircular
)

type Progress struct {
	ID    string
	Mode  ProgressMode
	Shape ProgressShape
	Value store.Binding[float64]

	base         facet.Facet
	once         sync.Once
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	anim         *animation.AnimatedFloat32
	phase        float32
}

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyUINotification,
		ConstructionClass: marks.ConstructionGenerated,
		Type:              marks.TypeName("uinotification:progress"),
	})
}

func (p *Progress) Base() *facet.Facet { p.ensureInit(); return &p.base }
func (p *Progress) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUINotification, ConstructionClass: marks.ConstructionGenerated, Type: marks.TypeName("uinotification:progress")}
}
func (p *Progress) AuthoredID() string               { return p.ID }
func (p *Progress) OnAttach(ctx facet.AttachContext) { p.syncRoles() }
func (p *Progress) OnDetach()                        {}
func (p *Progress) OnActivate()                      {}
func (p *Progress) OnDeactivate()                    {}

func (p *Progress) ensureInit() {
	p.once.Do(func() {
		p.base.BindImpl(p)
		p.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			b := p.bounds()
			return gfx.Size{W: b.Width(), H: b.Height()}
		}}
		p.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		p.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return p.project(ctx) }}
		p.base.AddRole(p.layoutRole)
		p.base.AddRole(p.viewportRole)
		p.base.AddRole(p.projection)
		if p.Mode == ProgressIndeterminate {
			p.anim = animation.NewAnimatedValue(func() animation.Float32 {
				return animation.Float32(p.phase)
			}, animation.TransitionSpec{Duration: 300 * time.Millisecond, Easing: "linear"}, nil)
		}
		p.syncRoles()
	})
}

func (p *Progress) syncRoles() {
	syncLayout(p.layoutRole, p.bounds())
	syncViewport(p.viewportRole, gfx.Identity())
}

func (p *Progress) bounds() gfx.Rect {
	if p.Shape == ProgressCircular {
		size := progressCircularSize()
		return gfx.RectFromXYWH(0, 0, size, size)
	}
	return gfx.RectFromXYWH(0, 0, progressLinearWidth(), progressLinearHeight())
}

func (p *Progress) fraction() float64 {
	if p.Mode == ProgressIndeterminate {
		if p.anim == nil {
			return 0
		}
		return float64(p.anim.Current())
	}
	return clampFloat(p.Value.Get(), 0, 1)
}

func (p *Progress) linearFillRect() gfx.Rect {
	b := p.bounds()
	return gfx.RectFromXYWH(b.Min.X, b.Min.Y, b.Width()*float32(p.fraction()), b.Height())
}

func (p *Progress) circularPath() gfx.Path {
	b := p.bounds()
	center := gfx.Point{X: b.Min.X + b.Width()/2, Y: b.Min.Y + b.Height()/2}
	radius := min(b.Width(), b.Height()) / 2
	frac := float32(p.fraction())
	if frac <= 0 {
		return gfx.Path{}
	}
	steps := int(math.Max(4, math.Ceil(float64(24*frac))))
	var builder = gfx.NewPath()
	for i := 0; i <= steps; i++ {
		t := float32(i) / float32(steps)
		angle := -math.Pi/2 + float64(t*frac)*2*math.Pi
		pt := gfx.Point{
			X: center.X + radius*float32(math.Cos(angle)),
			Y: center.Y + radius*float32(math.Sin(angle)),
		}
		if i == 0 {
			builder.MoveTo(pt)
		} else {
			builder.LineTo(pt)
		}
	}
	return builder.Build()
}

func (p *Progress) Tick(dt time.Duration) bool {
	if p.Mode != ProgressIndeterminate {
		return false
	}
	p.ensureInit()
	if p.anim == nil {
		return false
	}
	if dt > 0 {
		p.phase += float32(dt.Seconds() * 0.5)
		if p.phase > 1 {
			p.phase = float32(math.Mod(float64(p.phase), 1))
		}
	}
	return p.anim.Tick(dt)
}

func (p *Progress) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveProgressRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	var list gfx.CommandList
	switch p.Shape {
	case ProgressCircular:
		path := p.circularPath()
		if len(path.Segments) > 0 {
			list.Add(gfx.StrokePath{Path: path, Stroke: gfx.DefaultStroke(4), Brush: gfx.SolidBrush(fillColor(slots.Indicator.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{R: 0.25, G: 0.45, B: 0.95, A: 1}))})
		}
	default:
		list.Add(gfx.FillRect{Rect: p.bounds(), Brush: gfx.SolidBrush(fillColor(slots.Track.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{A: 0.2}))})
		list.Add(gfx.FillRect{Rect: p.linearFillRect(), Brush: gfx.SolidBrush(fillColor(slots.Indicator.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{R: 0.25, G: 0.45, B: 0.95, A: 1}))})
	}
	return &list
}
