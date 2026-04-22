package uinav

import (
	"fmt"
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uinav"
)

// Pagination renders a page window.
type Pagination struct {
	ID         string
	Page       store.Binding[int]
	TotalPages int
	WindowSize int

	base         facet.Facet
	once         sync.Once
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	inputRole    *facet.InputRole
}

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyUINav,
		ConstructionClass: marks.ConstructionGenerated,
		Type:              marks.TypeName("uinav:pagination"),
		HitTestable:       true,
	})
}

func (p *Pagination) Base() *facet.Facet { p.ensureInit(); return &p.base }
func (p *Pagination) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUINav, ConstructionClass: marks.ConstructionGenerated, Type: marks.TypeName("uinav:pagination"), HitTestable: true}
}
func (p *Pagination) AuthoredID() string               { return p.ID }
func (p *Pagination) OnAttach(ctx facet.AttachContext) { p.syncRoles() }
func (p *Pagination) OnDetach()                        {}
func (p *Pagination) OnActivate()                      {}
func (p *Pagination) OnDeactivate()                    {}

func (p *Pagination) ensureInit() {
	p.once.Do(func() {
		ensureBase(&p.base)
		p.base.BindImpl(p)
		p.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			bounds := p.bounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		p.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		p.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return p.project(ctx) }}
		p.hitRole = &facet.HitRole{OnHitTest: func(pt gfx.Point) facet.HitResult {
			if p.bounds().Contains(pt) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
			}
			return facet.HitResult{}
		}}
		p.inputRole = &facet.InputRole{
			OnPointer: func(e facet.PointerEvent) bool { return p.handlePointer(e) },
		}
		p.base.AddRole(p.layoutRole)
		p.base.AddRole(p.viewportRole)
		p.base.AddRole(p.projection)
		p.base.AddRole(p.hitRole)
		p.base.AddRole(p.inputRole)
		p.syncRoles()
	})
}

func (p *Pagination) syncRoles() {
	syncLayout(p.layoutRole, p.bounds())
	syncViewport(p.viewportRole, gfx.Identity())
}

func (p *Pagination) bounds() gfx.Rect {
	items := p.windowItems()
	return gfx.RectFromXYWH(0, 0, float32(len(items))*paginationItemSize(), menuRowHeight())
}

type paginationEntry struct {
	Key      string
	Page     int
	Ellipsis bool
}

func (p *Pagination) windowItems() []paginationEntry {
	total := p.TotalPages
	if total <= 0 {
		return nil
	}
	window := p.WindowSize
	if window <= 0 {
		window = 5
	}
	current := clampInt(p.Page.Get(), 1, total)
	if total <= window {
		out := make([]paginationEntry, 0, total)
		for i := 1; i <= total; i++ {
			out = append(out, paginationEntry{Key: itoa(i), Page: i})
		}
		return out
	}
	half := window / 2
	start := current - half
	if start < 1 {
		start = 1
	}
	end := start + window - 1
	if end > total {
		end = total
		start = end - window + 1
	}
	out := []paginationEntry{{Key: "1", Page: 1}}
	if start > 2 {
		out = append(out, paginationEntry{Key: "...", Ellipsis: true})
	}
	for i := start; i <= end; i++ {
		if i == 1 || i == total {
			continue
		}
		out = append(out, paginationEntry{Key: itoa(i), Page: i})
	}
	if end < total-1 {
		out = append(out, paginationEntry{Key: "...", Ellipsis: true})
	}
	if total > 1 {
		out = append(out, paginationEntry{Key: itoa(total), Page: total})
	}
	return out
}

func (p *Pagination) handlePointer(e facet.PointerEvent) bool {
	if e.Kind != platform.PointerPress {
		return false
	}
	items := p.windowItems()
	if len(items) == 0 {
		return false
	}
	idx := int(e.Position.X / 32)
	if idx < 0 || idx >= len(items) {
		return false
	}
	if items[idx].Ellipsis {
		if idx < len(items)/2 {
			p.Page.Set(clampInt(p.Page.Get()-p.WindowSize, 1, p.TotalPages))
		} else {
			p.Page.Set(clampInt(p.Page.Get()+p.WindowSize, 1, p.TotalPages))
		}
		return true
	}
	p.Page.Set(items[idx].Page)
	return true
}

func (p *Pagination) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolvePaginationRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	var list gfx.CommandList
	x := float32(0)
	for _, item := range p.windowItems() {
		rect := gfx.RectFromXYWH(x, 0, 28, 28)
		style := slots.Page.Resolve(theme.StateDefault, theme.DefaultTokens())
		if !item.Ellipsis && item.Page == p.Page.Get() {
			style = slots.Current.Resolve(theme.StateSelected, theme.DefaultTokens())
		}
		list.Add(gfx.FillRect{Rect: rect, Brush: gfx.SolidBrush(fillColor(style, gfx.Color{R: 1, G: 1, B: 1, A: 1}))})
		x += 32
	}
	return &list
}

func itoa(v int) string {
	return fmt.Sprintf("%d", v)
}
