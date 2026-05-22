package radial

import (
	"fmt"
	"math"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// Config configures a radial group policy.
type Config struct {
	DefaultRadius    float32
	StartAngle       float64
	WritingDirection facet.WritingDirection
}

// Child is the narrow view of a child facet participating in radial placement.
type Child struct {
	FacetID    facet.FacetID
	Attachment facet.Attachment
	Layout     *facet.LayoutRole
	Contract   facet.GroupChildContract
}

// ArrangedChild captures a child arranged by radial placement.
type ArrangedChild struct {
	FacetID   facet.FacetID
	Bounds    gfx.Rect
	Placement facet.RadialPlacement
	Angle     float64
	Radius    float32
	Center    gfx.Point
	ZPriority int32
	Contract  facet.GroupChildContract
}

// Policy arranges children on concentric radial tracks.
type Policy struct {
	cfg Config
}

// New constructs a radial policy.
func New(cfg Config) *Policy {
	return &Policy{cfg: cfg}
}

// Kind reports the local group layout contract kind.
func (p *Policy) Kind() facet.GroupLayoutKind {
	return facet.GroupLayoutRadial
}

// Measure computes the preferred size of the radial group.
func (p *Policy) Measure(ctx facet.MeasureContext, children []Child, constraints gfx.Size) (gfx.Size, error) {
	if p == nil || len(children) == 0 {
		return gfx.Size{}, nil
	}
	resolved, err := p.resolve(ctx, children, constraints)
	if err != nil {
		return gfx.Size{}, err
	}
	if len(resolved) == 0 {
		return gfx.Size{}, nil
	}
	minX, minY := float32(0), float32(0)
	maxX, maxY := float32(0), float32(0)
	first := true
	for _, child := range resolved {
		halfW := child.size.W / 2
		halfH := child.size.H / 2
		left := child.center.X - halfW
		right := child.center.X + halfW
		top := child.center.Y - halfH
		bottom := child.center.Y + halfH
		if first {
			minX, maxX = left, right
			minY, maxY = top, bottom
			first = false
			continue
		}
		if left < minX {
			minX = left
		}
		if right > maxX {
			maxX = right
		}
		if top < minY {
			minY = top
		}
		if bottom > maxY {
			maxY = bottom
		}
	}
	return gfx.Size{
		W: maxAbs(minX, maxX) * 2,
		H: maxAbs(minY, maxY) * 2,
	}, nil
}

// Arrange positions children around the configured center.
func (p *Policy) Arrange(ctx facet.ArrangeContext, children []Child, bounds gfx.Rect) ([]ArrangedChild, error) {
	if p == nil || len(children) == 0 {
		return nil, nil
	}
	resolved, err := p.resolve(facet.MeasureContext{
		Runtime:     ctx.Runtime,
		Theme:       ctx.Theme,
		Layer:       ctx.Layer,
		ParentGroup: ctx.ParentGroup,
		ChildGroup:  ctx.ChildGroup,
	}, children, gfx.Size{W: bounds.Width(), H: bounds.Height()})
	if err != nil {
		return nil, err
	}
	if len(resolved) == 0 {
		return nil, nil
	}
	centerX := bounds.Min.X + bounds.Width()/2
	centerY := bounds.Min.Y + bounds.Height()/2
	arranged := make([]ArrangedChild, 0, len(resolved))
	for _, child := range resolved {
		rect := gfx.RectFromXYWH(
			centerX+child.center.X-child.size.W/2,
			centerY+child.center.Y-child.size.H/2,
			child.size.W,
			child.size.H,
		)
		child.child.Layout.Arrange(facet.ArrangeContext{
			Runtime:     ctx.Runtime,
			Theme:       ctx.Theme,
			Layer:       ctx.Layer,
			ParentGroup: child.child.Layout.Parent,
			ChildGroup:  child.child.Layout.Child,
			Placement:   child.child.Attachment.Placement,
		}, rect)
		arranged = append(arranged, ArrangedChild{
			FacetID:   child.child.FacetID,
			Bounds:    rect,
			Placement: child.child.Attachment.Placement.Radial,
			Angle:     child.angle,
			Radius:    child.radius,
			Center:    gfx.Point{X: rect.Min.X + rect.Width()/2, Y: rect.Min.Y + rect.Height()/2},
			ZPriority: child.child.Attachment.ZPriority,
			Contract:  child.child.Contract,
		})
	}
	return arranged, nil
}

type resolvedChild struct {
	child     Child
	size      gfx.Size
	radius    float32
	angle     float64
	center    gfx.Point
	autoAngle bool
}

func (p *Policy) resolve(ctx facet.MeasureContext, children []Child, constraints gfx.Size) ([]resolvedChild, error) {
	resolved := make([]resolvedChild, 0, len(children))
	tracks := make(map[float32][]int)
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		if !child.Contract.SupportedPlacement.Has(facet.PlacementRadial) {
			panic(fmt.Sprintf("layout contract violation: facet %d; layer %d; placement radial; violated contract: unsupported placement mode; guidance: set SupportedPlacement to include radial placement", child.FacetID, child.Attachment.LayerID))
		}
		if child.Attachment.Placement.Mode != facet.PlacementRadial {
			return nil, fmt.Errorf("layout contract violation: facet %d; layer %d; placement radial; violated contract: unsupported placement mode; guidance: use facet.PlacementRadial for this child", child.FacetID, child.Attachment.LayerID)
		}
		size := measureChild(ctx, child, constraints)
		placement := child.Attachment.Placement.Radial
		radius := placement.RadiusTrack
		if radius < 0 {
			radius = p.cfg.DefaultRadius
		}
		radius += placement.RadiusOffset
		idx := len(resolved)
		resolved = append(resolved, resolvedChild{
			child:     child,
			size:      size,
			radius:    radius,
			angle:     placement.Angle,
			autoAngle: math.IsNaN(placement.Angle),
		})
		tracks[radius] = append(tracks[radius], idx)
	}
	direction := 1.0
	if p.cfg.WritingDirection == facet.WritingDirectionRTL {
		direction = -1.0
	}
	for _, indices := range tracks {
		auto := make([]int, 0, len(indices))
		for _, idx := range indices {
			if resolved[idx].autoAngle {
				auto = append(auto, idx)
			}
		}
		if len(auto) == 0 {
			continue
		}
		step := (2 * math.Pi) / float64(len(auto))
		for n, idx := range auto {
			resolved[idx].angle = p.cfg.StartAngle + direction*step*float64(n)
		}
	}
	for i := range resolved {
		angle := resolved[i].angle
		if math.IsNaN(angle) {
			angle = p.cfg.StartAngle
		}
		resolved[i].center = gfx.Point{
			X: float32(math.Cos(angle)) * resolved[i].radius,
			Y: float32(math.Sin(angle)) * resolved[i].radius,
		}
	}
	return resolved, nil
}

func measureChild(ctx facet.MeasureContext, child Child, constraints gfx.Size) gfx.Size {
	if child.Layout == nil {
		return gfx.Size{}
	}
	return child.Layout.Measure(ctx, facet.Constraints{
		MinSize: gfx.Size{},
		MaxSize: constraints,
	}).Size
}

func maxAbs(a, b float32) float32 {
	if abs(a) > abs(b) {
		return abs(a)
	}
	return abs(b)
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
