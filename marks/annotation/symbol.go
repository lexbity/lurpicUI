package annotation

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

// SymbolName identifies a reusable symbol definition.
type SymbolName string

// SymbolDefinition describes how to build a reusable symbol.
type SymbolDefinition struct {
	Name      SymbolName
	BuildPath func(size float32) gfx.Path
	Anchors   func(size float32) map[string]gfx.Point
}

// SymbolRegistry stores reusable symbol definitions.
type SymbolRegistry struct {
	mu    sync.RWMutex
	items map[SymbolName]SymbolDefinition
}

var defaultSymbolRegistry = &SymbolRegistry{items: make(map[SymbolName]SymbolDefinition)}

// RegisterSymbol registers a reusable symbol definition.
func RegisterSymbol(def SymbolDefinition) {
	defaultSymbolRegistry.Register(def)
}

// ResolveSymbol resolves a symbol definition.
func ResolveSymbol(name SymbolName) (SymbolDefinition, bool) {
	return defaultSymbolRegistry.Resolve(name)
}

// Register registers a symbol definition on the registry.
func (r *SymbolRegistry) Register(def SymbolDefinition) {
	if r == nil {
		return
	}
	if r.items == nil {
		r.items = make(map[SymbolName]SymbolDefinition)
	}
	if def.Name == "" {
		panic("marks/annotation: symbol name must not be empty")
	}
	if def.BuildPath == nil {
		panic("marks/annotation: symbol definition requires BuildPath")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[def.Name] = def
}

// Resolve looks up a symbol definition.
func (r *SymbolRegistry) Resolve(name SymbolName) (SymbolDefinition, bool) {
	if r == nil {
		return SymbolDefinition{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.items[name]
	return def, ok
}

// SymbolInstance places a reusable symbol at a point.
type SymbolInstance struct {
	ID       string
	Symbol   SymbolName
	Position gfx.Point
	Size     float32
	Rotation float32
	Style    theme.MarkStyle

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
		Type:              marks.TypeName("annotation:symbol"),
		HitTestable:       true,
		AnchorExporting:   true,
	})
	RegisterSymbol(SymbolDefinition{
		Name: "placeholder",
		BuildPath: func(size float32) gfx.Path {
			half := size / 2
			return pathFromPoints([]gfx.Point{
				{X: -half, Y: 0},
				{X: 0, Y: half},
				{X: half, Y: 0},
				{X: 0, Y: -half},
			}, true)
		},
		Anchors: func(size float32) map[string]gfx.Point {
			half := size / 2
			return map[string]gfx.Point{
				"center": {X: 0, Y: 0},
				"left":   {X: -half, Y: 0},
				"right":  {X: half, Y: 0},
				"top":    {X: 0, Y: -half},
				"bottom": {X: 0, Y: half},
			}
		},
	})
}

func (s *SymbolInstance) Base() *facet.Facet { s.ensureInit(); return &s.base }

func (s *SymbolInstance) Descriptor() marks.Descriptor {
	return marks.Descriptor{
		Family:            marks.FamilyAnnotation,
		ConstructionClass: marks.ConstructionPrimitive,
		Type:              marks.TypeName("annotation:symbol"),
		HitTestable:       true,
		AnchorExporting:   true,
	}
}

func (s *SymbolInstance) AuthoredID() string { return s.ID }
func (s *SymbolInstance) OnAttach(ctx facet.AttachContext) { s.syncRoles() }
func (s *SymbolInstance) OnDetach() {}
func (s *SymbolInstance) OnActivate() {}
func (s *SymbolInstance) OnDeactivate() {}

func (s *SymbolInstance) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	s.ensureInit()
	path, anchors := s.resolveGeometry()
	if len(anchors) == 0 {
		anchors = pathAnchorSet(path)
	}
	transform := gfx.Translation(s.Position.X, s.Position.Y).Multiply(gfx.Rotation(s.Rotation))
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, anchors)
}

func (s *SymbolInstance) HitTest(world gfx.Point) bool {
	s.ensureInit()
	inv, ok := gfx.Translation(s.Position.X, s.Position.Y).Multiply(gfx.Rotation(s.Rotation)).Inverse()
	if !ok {
		return false
	}
	return pathStrokeHit(s.resolveGeometryPath(), inv.TransformPoint(world), max(0.5, s.Size/10))
}

func (s *SymbolInstance) ensureInit() {
	s.once.Do(func() {
		s.base.BindImpl(s)
		s.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			bounds := s.localBounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		s.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		s.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return s.project(ctx) }}
		s.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if s.hitTestLocal(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		s.base.AddRole(s.layoutRole)
		s.base.AddRole(s.viewportRole)
		s.base.AddRole(s.projection)
		s.base.AddRole(s.hitRole)
		s.syncRoles()
	})
}

func (s *SymbolInstance) syncRoles() {
	syncLayout(s.layoutRole, s.localBounds())
	syncViewport(s.viewportRole, gfx.Translation(s.Position.X, s.Position.Y))
}

func (s *SymbolInstance) resolveGeometry() (gfx.Path, layout.AnchorSet) {
	size := s.Size
	if size <= 0 {
		size = 16
	}
	if def, ok := ResolveSymbol(s.Symbol); ok {
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
	return path, layout.AnchorSet{
		"center": {X: 0, Y: 0},
	}
}

func (s *SymbolInstance) resolveGeometryPath() gfx.Path {
	path, _ := s.resolveGeometry()
	return path
}

func (s *SymbolInstance) localBounds() gfx.Rect {
	return pathBounds(s.resolveGeometryPath())
}

func (s *SymbolInstance) project(ctx facet.ProjectionContext) *gfx.CommandList {
	path := s.resolveGeometryPath()
	if len(path.Segments) == 0 {
		return &gfx.CommandList{}
	}
	var list gfx.CommandList
	transform := gfx.Translation(s.Position.X, s.Position.Y).Multiply(gfx.Rotation(s.Rotation))
	list.Add(gfx.PushTransform{Matrix: transform})
	material := s.Style.Resolve(theme.StateDefault, theme.DefaultTokens())
	for _, fill := range material.Fills {
		if fill.Type == theme.FillNone {
			continue
		}
		list.Add(gfx.FillPath{Path: path, Brush: gfx.SolidBrush(fill.Color)})
	}
	if len(material.Strokes) > 0 {
		stroke := material.Strokes[0]
		list.Add(gfx.StrokePath{Path: path, Stroke: strokeStyle(stroke), Brush: strokeBrushFromMaterial(stroke, 1)})
	}
	list.Add(gfx.PopTransform{})
	return &list
}

func (s *SymbolInstance) hitTestLocal(p gfx.Point) bool {
	size := s.Size
	if size <= 0 {
		size = 16
	}
	return pathStrokeHit(s.resolveGeometryPath(), p, max(0.5, size/8))
}

func (s *SymbolInstance) localBoundsSized() gfx.Rect {
	size := s.Size
	if size <= 0 {
		size = 16
	}
	return gfx.RectFromXYWH(-size/2, -size/2, size, size)
}
