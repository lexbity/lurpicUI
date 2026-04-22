package uinav

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uinav"
)

// BreadcrumbItem describes one breadcrumb segment.
type BreadcrumbItem struct {
	Key   string
	Label string
}

// Breadcrumbs renders a breadcrumb trail.
type Breadcrumbs struct {
	ID      string
	Items   []BreadcrumbItem
	Current store.Binding[string]

	base         facet.Facet
	once         sync.Once
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
}

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyUINav,
		ConstructionClass: marks.ConstructionGenerated,
		Type:              marks.TypeName("uinav:breadcrumbs"),
		AnchorExporting:   false,
	})
}

func (b *Breadcrumbs) Base() *facet.Facet { b.ensureInit(); return &b.base }
func (b *Breadcrumbs) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUINav, ConstructionClass: marks.ConstructionGenerated, Type: marks.TypeName("uinav:breadcrumbs")}
}
func (b *Breadcrumbs) AuthoredID() string               { return b.ID }
func (b *Breadcrumbs) OnAttach(ctx facet.AttachContext) { b.syncRoles() }
func (b *Breadcrumbs) OnDetach()                        {}
func (b *Breadcrumbs) OnActivate()                      {}
func (b *Breadcrumbs) OnDeactivate()                    {}

func (b *Breadcrumbs) ensureInit() {
	b.once.Do(func() {
		ensureBase(&b.base)
		b.base.BindImpl(b)
		b.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			bounds := b.bounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		b.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		b.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return b.project(ctx) }}
		b.base.AddRole(b.layoutRole)
		b.base.AddRole(b.viewportRole)
		b.base.AddRole(b.projection)
		b.syncRoles()
	})
}

func (b *Breadcrumbs) syncRoles() {
	syncLayout(b.layoutRole, b.bounds())
	syncViewport(b.viewportRole, gfx.Identity())
}

func (b *Breadcrumbs) bounds() gfx.Rect {
	items := b.visibleItems(1000)
	width := float32(0)
	for _, item := range items {
		width += breadcrumbWidth(item.Label)
	}
	return gfx.RectFromXYWH(0, 0, width, 28)
}

func breadcrumbWidth(label string) float32 {
	return float32(len(label))*8 + 24
}

func (b *Breadcrumbs) visibleItems(maxWidth float32) []BreadcrumbItem {
	if len(b.Items) <= 2 {
		return append([]BreadcrumbItem(nil), b.Items...)
	}
	total := float32(0)
	for _, item := range b.Items {
		total += breadcrumbWidth(item.Label)
	}
	if total <= maxWidth {
		return append([]BreadcrumbItem(nil), b.Items...)
	}
	return []BreadcrumbItem{b.Items[0], BreadcrumbItem{Key: "...", Label: "..."}, b.Items[len(b.Items)-1]}
}

func (b *Breadcrumbs) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveBreadcrumbRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	var list gfx.CommandList
	x := float32(0)
	items := b.visibleItems(240)
	for i, item := range items {
		style := slots.Item.Resolve(theme.StateDefault, theme.DefaultTokens())
		if item.Key == b.Current.Get() || (i == len(items)-1 && len(items) > 0 && item.Key == b.Items[len(b.Items)-1].Key) {
			style = slots.Current.Resolve(theme.StateSelected, theme.DefaultTokens())
		}
		list.Add(gfx.FillRect{Rect: gfx.RectFromXYWH(x, 0, breadcrumbWidth(item.Label), 28), Brush: gfx.SolidBrush(fillColor(style, gfx.Color{R: 1, G: 1, B: 1, A: 1}))})
		x += breadcrumbWidth(item.Label)
		if i < len(items)-1 {
			list.Add(gfx.DrawPoints{Points: []gfx.Point{{X: x, Y: 14}}, Radius: 1, Brush: gfx.SolidBrush(fillColor(slots.Separator.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{A: 0.6}))})
			x += 12
		}
	}
	return &list
}
