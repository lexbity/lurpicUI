package annotation

import (
	"fmt"
	"math"
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/store"
)

// ConnectorMode selects the routing strategy for a connector.
type ConnectorMode uint8

const (
	ConnectorStraight ConnectorMode = iota
	ConnectorQuadratic
	ConnectorCubic
	ConnectorOrthogonal
)

// ConnectorEndpoint identifies one anchor endpoint and optional bias.
type ConnectorEndpoint struct {
	Source AnchorSourceRef
	Bias   gfx.Point
}

// Connector connects two anchors with a routed path.
type Connector struct {
	ID        string
	Mode      ConnectorMode
	From      ConnectorEndpoint
	To        ConnectorEndpoint
	ArrowHead bool
	Label     *Label

	base         facet.Facet
	once         sync.Once
	runtime      facet.RuntimeServices
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	cacheMu      sync.Mutex
	cacheKey     string
	cachePath    gfx.Path
	cacheBounds  gfx.Rect
	routeVersion store.VersionSource
	cacheHits    int
	builds       int
}

func init() {
	registerAnnotationDescriptor(marks.Descriptor{
		Family:            marks.FamilyAnnotation,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("annotation:connector"),
		HitTestable:       true,
		AnchorExporting:   true,
		ChildHosting:      true,
	})
}

func (c *Connector) Base() *facet.Facet { c.ensureInit(); return &c.base }

func (c *Connector) Descriptor() marks.Descriptor {
	return marks.Descriptor{
		Family:            marks.FamilyAnnotation,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("annotation:connector"),
		HitTestable:       true,
		AnchorExporting:   true,
		ChildHosting:      true,
	}
}

func (c *Connector) AuthoredID() string { return c.ID }
func (c *Connector) OnAttach(ctx facet.AttachContext) {
	c.runtime = ctx.Runtime
	c.syncRoles()
	c.requestRoute()
}
func (c *Connector) OnDetach()     {}
func (c *Connector) OnActivate()   {}
func (c *Connector) OnDeactivate() {}

func (c *Connector) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	c.ensureInit()
	path := c.routePath()
	if len(path.Segments) == 0 {
		return nil
	}
	transform := gfx.Identity()
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform
	}
	return transformAnchors(transform, boundsAnchors(pathBounds(path)))
}

func (c *Connector) ensureInit() {
	c.once.Do(func() {
		c.base.BindImpl(c)
		c.layoutRole = &facet.LayoutRole{OnMeasure: func(cn facet.Constraints) gfx.Size {
			bounds := c.routeBounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		c.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		c.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return c.project(ctx) }}
		c.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if c.hitTestLocal(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorCrosshair}
			}
			return facet.HitResult{}
		}}
		c.base.AddRole(c.layoutRole)
		c.base.AddRole(c.viewportRole)
		c.base.AddRole(c.projection)
		c.base.AddRole(c.hitRole)
		syncLayout(c.layoutRole, c.routeBounds())
		syncViewport(c.viewportRole, gfx.Identity())
	})
}

func (c *Connector) syncRoles() {
	syncLayout(c.layoutRole, c.routeBounds())
	syncViewport(c.viewportRole, gfx.Identity())
}

func (c *Connector) routeBounds() gfx.Rect {
	path := c.routePath()
	if len(path.Segments) == 0 {
		return gfx.Rect{}
	}
	return pathBounds(path)
}

func (c *Connector) routePath() gfx.Path {
	key := c.routeSignature()
	c.cacheMu.Lock()
	if key == c.cacheKey && len(c.cachePath.Segments) > 0 {
		c.cacheHits++
		path := c.cachePath
		c.cacheMu.Unlock()
		return path
	}
	c.cacheMu.Unlock()

	switch c.Mode {
	case ConnectorOrthogonal:
		c.requestRoute()
		c.cacheMu.Lock()
		path := c.cachePath
		c.cacheMu.Unlock()
		if len(path.Segments) == 0 {
			path = c.orthogonalPath()
			if len(path.Segments) > 0 {
				c.storeRoute(key, path)
			}
		}
		return path
	case ConnectorQuadratic:
		path := c.quadraticPath()
		if len(path.Segments) > 0 {
			c.storeRoute(key, path)
		}
		return path
	case ConnectorCubic:
		path := c.cubicPath()
		if len(path.Segments) > 0 {
			c.storeRoute(key, path)
		}
		return path
	default:
		path := c.straightPath()
		if len(path.Segments) > 0 {
			c.storeRoute(key, path)
		}
		return path
	}
}

func (c *Connector) project(ctx facet.ProjectionContext) *gfx.CommandList {
	path := c.routePath()
	if len(path.Segments) == 0 {
		return &gfx.CommandList{}
	}
	var list gfx.CommandList
	list.Add(gfx.StrokePath{
		Path:   path,
		Stroke: gfx.DefaultStroke(1.5),
		Brush:  gfx.SolidBrush(gfx.Color{A: 1}),
	})
	if c.ArrowHead {
		list.Commands = append(list.Commands, c.arrowCommands(path)...)
	}
	if c.Label != nil {
		mid := c.routeLabelPoint(path)
		label := *c.Label
		label.Placement = LabelFree
		label.Offset = mid
		if cmds := label.project(ctx); cmds != nil {
			list.Commands = append(list.Commands, cmds.Commands...)
		}
	}
	return &list
}

func (c *Connector) hitTestLocal(p gfx.Point) bool {
	path := c.routePath()
	if len(path.Segments) == 0 {
		return false
	}
	return pathStrokeHit(path, p, 10)
}

func (c *Connector) requestRoute() {
	if c.Mode != ConnectorOrthogonal || c.runtime == nil {
		path := c.orthogonalPath()
		if len(path.Segments) > 0 {
			c.storeRoute(c.routeSignature(), path)
		}
		return
	}
	signature := c.routeSignature()
	from, okFrom := c.resolvedEndpoint(c.From)
	to, okTo := c.resolvedEndpoint(c.To)
	if !okFrom || !okTo {
		return
	}
	req := routeRequest{
		Key:      signature,
		From:     from,
		To:       to,
		FromBias: c.From.Bias,
		ToBias:   c.To.Bias,
	}
	snap := job.NewSnapshot(req, hashString(signature))
	snap = job.BindCurrentVersions(snap, func() []store.Version { return []store.Version{hashString(c.routeSignature())} })
	j := job.Job[routeRequest, routeResult]{
		ID:       job.JobID(c.base.ID()),
		Priority: job.PriorityBackground,
		Snapshot: snap,
		Work: func(snap job.Snapshot[routeRequest], cancel *job.CancelToken) (routeResult, error) {
			return routeResult{Key: snap.Data.Key, Path: orthogonalPathFor(snap.Data.From, snap.Data.To, snap.Data.FromBias, snap.Data.ToBias)}, nil
		},
	}
	c.runtime.Schedule(job.BindJob(uint64(c.base.ID()), j, func(out routeResult) {
		if len(out.Path.Segments) > 0 {
			c.storeRoute(out.Key, out.Path)
		}
	}))
}

func (c *Connector) storeRoute(key string, path gfx.Path) {
	c.cacheMu.Lock()
	c.cacheKey = key
	c.cachePath = path
	c.cacheBounds = pathBounds(path)
	c.builds++
	c.cacheMu.Unlock()
}

func (c *Connector) routeSignature() string {
	from := c.endpointPoint(c.From)
	to := c.endpointPoint(c.To)
	return fmt.Sprintf("%d|%d|%g,%g|%g,%g|%g,%g|%g,%g|%t", c.Mode, c.base.ID(),
		from.X, from.Y,
		to.X, to.Y,
		c.From.Bias.X, c.From.Bias.Y, c.To.Bias.X, c.To.Bias.Y, c.ArrowHead)
}

func (c *Connector) endpointPoint(ep ConnectorEndpoint) gfx.Point {
	pt, ok := c.resolvedEndpoint(ep)
	if !ok {
		return gfx.Point{}
	}
	return pt
}

func (c *Connector) resolvedEndpoint(ep ConnectorEndpoint) (gfx.Point, bool) {
	if pt, ok := anchorPoint(c.base.Parent(), ep.Source, "bounds-center"); ok {
		return gfx.Point{X: pt.X + ep.Bias.X, Y: pt.Y + ep.Bias.Y}, true
	}
	return gfx.Point{}, false
}

func (c *Connector) straightPath() gfx.Path {
	from, okFrom := c.resolvedEndpoint(c.From)
	to, okTo := c.resolvedEndpoint(c.To)
	if !okFrom || !okTo {
		return gfx.Path{}
	}
	return pathFromPoints([]gfx.Point{from, to}, false)
}

func (c *Connector) quadraticPath() gfx.Path {
	from, okFrom := c.resolvedEndpoint(c.From)
	to, okTo := c.resolvedEndpoint(c.To)
	if !okFrom || !okTo {
		return gfx.Path{}
	}
	mid := gfx.Point{X: (from.X + to.X) / 2, Y: (from.Y + to.Y) / 2}
	ctrl := gfx.Point{
		X: mid.X + (c.From.Bias.X+c.To.Bias.X)/2,
		Y: mid.Y + (c.From.Bias.Y+c.To.Bias.Y)/2,
	}
	return gfx.NewPath().MoveTo(from).QuadTo(ctrl, to).Build()
}

func (c *Connector) cubicPath() gfx.Path {
	from, okFrom := c.resolvedEndpoint(c.From)
	to, okTo := c.resolvedEndpoint(c.To)
	if !okFrom || !okTo {
		return gfx.Path{}
	}
	dx := to.X - from.X
	dy := to.Y - from.Y
	c1 := gfx.Point{X: from.X + dx/3 + c.From.Bias.X, Y: from.Y + dy/3 + c.From.Bias.Y}
	c2 := gfx.Point{X: to.X - dx/3 + c.To.Bias.X, Y: to.Y - dy/3 + c.To.Bias.Y}
	return gfx.NewPath().MoveTo(from).CubicTo(c1, c2, to).Build()
}

func (c *Connector) orthogonalPath() gfx.Path {
	from, okFrom := c.resolvedEndpoint(c.From)
	to, okTo := c.resolvedEndpoint(c.To)
	if !okFrom || !okTo {
		return gfx.Path{}
	}
	return orthogonalPathFor(from, to, c.From.Bias, c.To.Bias)
}

func orthogonalPathFor(from, to, fromBias, toBias gfx.Point) gfx.Path {
	if from == to {
		return pathFromPoints([]gfx.Point{from, {X: from.X + 1, Y: from.Y + 1}}, false)
	}
	dx := to.X - from.X
	dy := to.Y - from.Y
	if math.Abs(float64(dx)) >= math.Abs(float64(dy)) {
		midX := from.X + dx/2 + (fromBias.X+toBias.X)/2
		return pathFromPoints([]gfx.Point{
			from,
			{X: midX, Y: from.Y + fromBias.Y},
			{X: midX, Y: to.Y + toBias.Y},
			to,
		}, false)
	}
	midY := from.Y + dy/2 + (fromBias.Y+toBias.Y)/2
	return pathFromPoints([]gfx.Point{
		from,
		{X: from.X + fromBias.X, Y: midY},
		{X: to.X + toBias.X, Y: midY},
		to,
	}, false)
}

type routeRequest struct {
	Key      string
	From     gfx.Point
	To       gfx.Point
	FromBias gfx.Point
	ToBias   gfx.Point
}

type routeResult struct {
	Key  string
	Path gfx.Path
}

func (c *Connector) routeLabelPoint(path gfx.Path) gfx.Point {
	if len(path.Segments) == 0 {
		return gfx.Point{}
	}
	if len(path.Segments) == 1 && len(path.Segments[0].Pts) >= 2 {
		a := path.Segments[0].Pts[0]
		b := path.Segments[0].Pts[1]
		return gfx.Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
	}
	bounds := pathBounds(path)
	return gfx.Point{X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y + bounds.Height()/2}
}

func (c *Connector) arrowCommands(path gfx.Path) []gfx.Command {
	pts := flattenPath(path)
	if len(pts) == 0 {
		return nil
	}
	last := pts[len(pts)-1]
	if len(last) < 2 {
		return nil
	}
	end := last[len(last)-1]
	prev := last[len(last)-2]
	dx := end.X - prev.X
	dy := end.Y - prev.Y
	l := float32(math.Hypot(float64(dx), float64(dy)))
	if l == 0 {
		return nil
	}
	ux := dx / l
	uy := dy / l
	left := gfx.Point{X: end.X - ux*8 - uy*4, Y: end.Y - uy*8 + ux*4}
	right := gfx.Point{X: end.X - ux*8 + uy*4, Y: end.Y - uy*8 - ux*4}
	return []gfx.Command{gfx.FillPath{Path: pathFromPoints([]gfx.Point{end, left, right}, true), Brush: gfx.SolidBrush(gfx.Color{A: 1})}}
}
