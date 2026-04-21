package annotation

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

// IconName identifies a reusable icon definition.
type IconName string

// IconDefinition describes a reusable icon.
type IconDefinition struct {
	Name      IconName
	BuildPath  func(size float32) gfx.Path
	Anchors    func(size float32) map[string]gfx.Point
}

// IconRegistry stores icon definitions.
type IconRegistry struct {
	mu    sync.RWMutex
	items map[IconName]IconDefinition
}

var defaultIconRegistry = &IconRegistry{items: make(map[IconName]IconDefinition)}

// RegisterIcon registers an icon definition.
func RegisterIcon(def IconDefinition) {
	defaultIconRegistry.Register(def)
}

// ResolveIcon resolves an icon definition.
func ResolveIcon(name IconName) (IconDefinition, bool) {
	return defaultIconRegistry.Resolve(name)
}

// Register adds a definition to the registry.
func (r *IconRegistry) Register(def IconDefinition) {
	if r == nil {
		return
	}
	if r.items == nil {
		r.items = make(map[IconName]IconDefinition)
	}
	if def.Name == "" {
		panic("marks/annotation: icon name must not be empty")
	}
	if def.BuildPath == nil {
		panic("marks/annotation: icon definition requires BuildPath")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[def.Name] = def
}

// Resolve looks up an icon definition.
func (r *IconRegistry) Resolve(name IconName) (IconDefinition, bool) {
	if r == nil {
		return IconDefinition{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.items[name]
	return def, ok
}

// Icon is a named icon instance.
type Icon struct {
	ID       string
	Name     IconName
	Position gfx.Point
	Size     float32

	base         facet.Facet
	once         sync.Once
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
}

func init() {
	registerAnnotationDescriptor(marks.Descriptor{
		Family:            marks.FamilyAnnotation,
		ConstructionClass: marks.ConstructionPrimitive,
		Type:              marks.TypeName("annotation:icon"),
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

func (i *Icon) Base() *facet.Facet { i.ensureInit(); return &i.base }

func (i *Icon) Descriptor() marks.Descriptor {
	return marks.Descriptor{
		Family:            marks.FamilyAnnotation,
		ConstructionClass: marks.ConstructionPrimitive,
		Type:              marks.TypeName("annotation:icon"),
		HitTestable:       true,
		AnchorExporting:   true,
	}
}

func (i *Icon) AuthoredID() string { return i.ID }
func (i *Icon) OnAttach(ctx facet.AttachContext) { i.syncRoles() }
func (i *Icon) OnDetach() {}
func (i *Icon) OnActivate() {}
func (i *Icon) OnDeactivate() {}

func (i *Icon) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	i.ensureInit()
	path, anchors := i.resolveGeometry()
	if len(anchors) == 0 {
		anchors = boundsAnchors(pathBounds(path))
	}
	transform := gfx.Translation(i.Position.X, i.Position.Y)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, anchors)
}

func (i *Icon) ensureInit() {
	i.once.Do(func() {
		i.base.BindImpl(i)
		i.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			bounds := i.localBounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		i.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		i.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return i.project(ctx) }}
		i.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if i.localBounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		i.base.AddRole(i.layoutRole)
		i.base.AddRole(i.viewportRole)
		i.base.AddRole(i.projection)
		i.base.AddRole(i.hitRole)
		i.syncRoles()
	})
}

func (i *Icon) syncRoles() {
	syncLayout(i.layoutRole, i.localBounds())
	syncViewport(i.viewportRole, gfx.Translation(i.Position.X, i.Position.Y))
}

func (i *Icon) resolveGeometry() (gfx.Path, layout.AnchorSet) {
	size := i.Size
	if size <= 0 {
		size = 16
	}
	if def, ok := ResolveIcon(i.Name); ok {
		path := def.BuildPath(size)
		var anchors layout.AnchorSet
		if def.Anchors != nil {
			anchors = make(layout.AnchorSet)
			for name, pt := range def.Anchors(size) {
				anchors[layout.AnchorID(name)] = pt
			}
		}
		return path, anchors
	}
	path := gfx.CirclePath(gfx.Point{}, size/2)
	return path, layout.AnchorSet{"center": {X: 0, Y: 0}}
}

func (i *Icon) localBounds() gfx.Rect {
	return pathBounds(i.resolveGeometryPath())
}

func (i *Icon) resolveGeometryPath() gfx.Path {
	path, _ := i.resolveGeometry()
	return path
}

func (i *Icon) project(ctx facet.ProjectionContext) *gfx.CommandList {
	path := i.resolveGeometryPath()
	if len(path.Segments) == 0 {
		return &gfx.CommandList{}
	}
	var list gfx.CommandList
	transform := gfx.Translation(i.Position.X, i.Position.Y)
	list.Add(gfx.PushTransform{Matrix: transform})
	list.Add(gfx.FillPath{Path: path, Brush: gfx.SolidBrush(gfx.Color{A: 1})})
	list.Add(gfx.PopTransform{})
	return &list
}
