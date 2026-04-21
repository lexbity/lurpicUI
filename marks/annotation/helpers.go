package annotation

import (
	"fmt"
	"hash/fnv"
	"math"
	"unicode/utf8"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// AnchorSourceRef identifies a mark/anchor pair forwarded by annotation marks.
type AnchorSourceRef = structure.AnchorSourceRef

func registerAnnotationDescriptor(d marks.Descriptor) {
	marks.RegisterDescriptor(d)
}

func syncLayout(layoutRole *facet.LayoutRole, bounds gfx.Rect) {
	if layoutRole == nil {
		return
	}
	layoutRole.Arrange(bounds)
	layoutRole.MeasuredSize = gfx.Size{W: bounds.Width(), H: bounds.Height()}
}

func syncViewport(viewport *facet.ViewportRole, transform gfx.Transform) {
	if viewport == nil {
		return
	}
	viewport.Transform = transform
}

func attachSingleChild(parent *facet.Facet, child marks.Mark) {
	if parent == nil || child == nil {
		return
	}
	impl, ok := child.(facet.FacetImpl)
	if !ok {
		panic("marks/annotation: child mark does not implement facet.FacetImpl")
	}
	parent.AddChild(impl.Base())
}

func normalizeTransform(t gfx.Transform) gfx.Transform {
	if t == (gfx.Transform{}) {
		return gfx.Identity()
	}
	return t
}

func transformAnchors(tx gfx.Transform, anchors layout.AnchorSet) layout.AnchorSet {
	if len(anchors) == 0 {
		return nil
	}
	out := make(layout.AnchorSet, len(anchors))
	for id, pt := range anchors {
		out[id] = tx.TransformPoint(pt)
	}
	return out
}

func boundsAnchors(bounds gfx.Rect) layout.AnchorSet {
	if bounds.IsEmpty() {
		return nil
	}
	return layout.AnchorSet{
		"bounds-center": {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y + bounds.Height()/2},
		"top-left":      {X: bounds.Min.X, Y: bounds.Min.Y},
		"top-right":     {X: bounds.Max.X, Y: bounds.Min.Y},
		"bottom-right":  {X: bounds.Max.X, Y: bounds.Max.Y},
		"bottom-left":   {X: bounds.Min.X, Y: bounds.Max.Y},
	}
}

func pathBounds(path gfx.Path) gfx.Rect {
	if len(path.Segments) == 0 {
		return gfx.Rect{}
	}
	var minPt, maxPt gfx.Point
	havePoint := false
	visit := func(p gfx.Point) {
		if !havePoint {
			minPt = p
			maxPt = p
			havePoint = true
			return
		}
		if p.X < minPt.X {
			minPt.X = p.X
		}
		if p.Y < minPt.Y {
			minPt.Y = p.Y
		}
		if p.X > maxPt.X {
			maxPt.X = p.X
		}
		if p.Y > maxPt.Y {
			maxPt.Y = p.Y
		}
	}
	for _, seg := range path.Segments {
		for _, p := range seg.Pts {
			visit(p)
		}
	}
	if !havePoint {
		return gfx.Rect{}
	}
	return gfx.Rect{Min: minPt, Max: maxPt}
}

func pathAnchorSet(path gfx.Path) layout.AnchorSet {
	return boundsAnchors(pathBounds(path))
}

func pathFromPoints(pts []gfx.Point, closed bool) gfx.Path {
	if len(pts) == 0 {
		return gfx.Path{}
	}
	builder := gfx.NewPath().MoveTo(pts[0])
	for i := 1; i < len(pts); i++ {
		builder.LineTo(pts[i])
	}
	if closed {
		builder.Close()
	}
	return builder.Build()
}

func rotatePoint(p gfx.Point, radians float32) gfx.Point {
	tx := gfx.Rotation(radians)
	return tx.TransformPoint(p)
}

func shapeRect(center gfx.Point, size float32) gfx.Rect {
	half := size / 2
	return gfx.RectFromXYWH(center.X-half, center.Y-half, size, size)
}

func hashString(parts ...string) store.Version {
	h := fnv.New64a()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return store.Version(h.Sum64())
}

func anchorPoint(root *facet.Facet, ref AnchorSourceRef, fallback layout.AnchorID) (gfx.Point, bool) {
	if root == nil {
		return gfx.Point{}, false
	}
	target := findMarkFacet(root, ref.MarkID)
	if target == nil {
		return gfx.Point{}, false
	}
	exporter, ok := target.Impl().(layout.AnchorExporter)
	if !ok {
		return gfx.Point{}, false
	}
	anchors := exporter.ExportAnchors(layout.AnchorExportContext{})
	if len(anchors) == 0 {
		return gfx.Point{}, false
	}
	if ref.Anchor != "" {
		if pt, ok := anchors[layout.AnchorID(ref.Anchor)]; ok {
			return pt, true
		}
		return gfx.Point{}, false
	}
	if pt, ok := anchors[fallback]; ok {
		return pt, true
	}
	if pt, ok := anchors["bounds-center"]; ok {
		return pt, true
	}
	for _, pt := range anchors {
		return pt, true
	}
	return gfx.Point{}, false
}

func findMarkFacet(base *facet.Facet, markID string) *facet.Facet {
	if base == nil || markID == "" {
		return nil
	}
	stack := []*facet.Facet{base}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current == nil {
			continue
		}
		impl := current.Impl()
		if impl != nil {
			if authored, ok := impl.(interface{ AuthoredID() string }); ok && authored.AuthoredID() == markID {
				return current
			}
			if current.ID() != 0 && stringifyFacetID(current.ID()) == markID {
				return current
			}
		}
		children := current.Children()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, children[i])
		}
	}
	return nil
}

func stringifyFacetID(id facet.FacetID) string {
	return fmt.Sprintf("%d", uint64(id))
}

func basicTextParagraph(t *basic.Text) (lineCount int, runeCount int, maxRunes int, lineSize float32) {
	if t == nil {
		return 0, 0, 0, 0
	}
	style := t.Style
	if style.Size <= 0 {
		style = text.DefaultStyle()
	}
	lineSize = style.Size * 1.2
	if lineSize <= 0 {
		lineSize = 14 * 1.2
	}
	if len(t.Paragraph.Spans) == 0 {
		return 1, 0, 0, lineSize
	}
	for _, span := range t.Paragraph.Spans {
		parts := splitLines(span.Text)
		runes := 0
		for _, line := range parts {
			n := utf8.RuneCountInString(line)
			runes += n
			if n > maxRunes {
				maxRunes = n
			}
		}
		runeCount += runes
		if len(parts) > lineCount {
			lineCount = len(parts)
		}
	}
	if lineCount == 0 {
		lineCount = 1
	}
	return lineCount, runeCount, maxRunes, lineSize
}

func splitLines(s string) []string {
	if s == "" {
		return []string{""}
	}
	out := []string{""}
	for _, r := range s {
		if r == '\n' {
			out = append(out, "")
			continue
		}
		out[len(out)-1] += string(r)
	}
	return out
}

func basicTextBounds(t *basic.Text) gfx.Rect {
	if t == nil {
		return gfx.Rect{}
	}
	lineCount, _, maxRunes, lineSize := basicTextParagraph(t)
	style := t.Style
	if style.Size <= 0 {
		style = text.DefaultStyle()
	}
	width := float32(maxRunes) * style.Size * 0.62
	if t.MaxWidth > 0 && width > t.MaxWidth {
		width = t.MaxWidth
	}
	if width < 0 {
		width = 0
	}
	height := float32(lineCount) * lineSize
	if height <= 0 {
		height = lineSize
	}
	return gfx.RectFromXYWH(0, 0, width, height)
}

func textMarkCommandList(t *basic.Text, tx gfx.Transform) *gfx.CommandList {
	if t == nil {
		return &gfx.CommandList{}
	}
	copy := cloneBasicText(t)
	copy.Tx.Transform = tx.Multiply(normalizeTransform(copy.Tx.Transform))
	role := copy.Base().ProjectionRole()
	if role == nil {
		return &gfx.CommandList{}
	}
	return role.Project(facet.ProjectionContext{})
}

func textMarkBounds(t *basic.Text, tx gfx.Transform) gfx.Rect {
	if t == nil {
		return gfx.Rect{}
	}
	copy := cloneBasicText(t)
	copy.Tx.Transform = tx.Multiply(normalizeTransform(copy.Tx.Transform))
	role := copy.Base().LayoutRole()
	if role != nil {
		size := role.Measure(facet.Constraints{})
		if size.W > 0 || size.H > 0 {
			return gfx.RectFromXYWH(0, 0, size.W, size.H)
		}
	}
	return basicTextBounds(copy)
}

func cloneBasicText(src *basic.Text) *basic.Text {
	if src == nil {
		return nil
	}
	return &basic.Text{
		ID:         src.ID,
		Paragraph:  src.Paragraph,
		Style:      src.Style,
		MaxWidth:   src.MaxWidth,
		Align:      src.Align,
		Selectable: src.Selectable,
		Tx:         src.Tx,
	}
}

func transformRect(tx gfx.Transform, r gfx.Rect) gfx.Rect {
	return tx.TransformRect(r)
}

func pathStrokeHit(path gfx.Path, p gfx.Point, width float32) bool {
	if width <= 0 {
		return false
	}
	pts := flattenPath(path)
	if len(pts) == 0 {
		return false
	}
	tolerance := width / 2
	for _, contour := range pts {
		for i := 1; i < len(contour); i++ {
			if segmentDistance(p, contour[i-1], contour[i]) <= tolerance {
				return true
			}
		}
	}
	return false
}

func pathContains(path gfx.Path, p gfx.Point, evenOdd bool) bool {
	contours := flattenPath(path)
	if len(contours) == 0 {
		return false
	}
	if evenOdd {
		inside := false
		for _, contour := range contours {
			if pointInPolygon(p, contour, true) {
				inside = !inside
			}
		}
		return inside
	}
	winding := 0
	for _, contour := range contours {
		if pointInPolygon(p, contour, false) {
			winding++
		}
	}
	return winding != 0
}

func pointInPolygon(p gfx.Point, pts []gfx.Point, evenOdd bool) bool {
	if len(pts) < 3 {
		return false
	}
	inside := false
	winding := 0
	for i, j := 0, len(pts)-1; i < len(pts); j, i = i, i+1 {
		pi := pts[i]
		pj := pts[j]
		intersects := ((pi.Y > p.Y) != (pj.Y > p.Y)) &&
			(p.X < (pj.X-pi.X)*(p.Y-pi.Y)/(pj.Y-pi.Y+1e-12)+pi.X)
		if evenOdd && intersects {
			inside = !inside
		}
		if !evenOdd {
			if pi.Y <= p.Y {
				if pj.Y > p.Y && cross(pj, pi, p) > 0 {
					winding++
				}
			} else if pj.Y <= p.Y && cross(pj, pi, p) < 0 {
				winding--
			}
		}
	}
	if evenOdd {
		return inside
	}
	return winding != 0
}

func cross(a, b, c gfx.Point) float32 {
	return (a.X-b.X)*(c.Y-b.Y) - (a.Y-b.Y)*(c.X-b.X)
}

func flattenPath(path gfx.Path) [][]gfx.Point {
	if len(path.Segments) == 0 {
		return nil
	}
	var contours [][]gfx.Point
	var pts []gfx.Point
	var current gfx.Point
	var start gfx.Point
	haveStart := false
	flush := func() {
		if len(pts) > 0 {
			contours = append(contours, append([]gfx.Point(nil), pts...))
			pts = pts[:0]
		}
	}
	for _, seg := range path.Segments {
		switch seg.Verb {
		case gfx.PathMoveTo:
			flush()
			current = seg.Pts[0]
			start = current
			haveStart = true
			pts = append(pts, current)
		case gfx.PathLineTo:
			current = seg.Pts[0]
			pts = append(pts, current)
		case gfx.PathQuadTo:
			if !haveStart {
				continue
			}
			ctrl := seg.Pts[0]
			dest := seg.Pts[1]
			const steps = 8
			for i := 1; i <= steps; i++ {
				t := float32(i) / steps
				omt := 1 - t
				p := gfx.Point{
					X: omt*omt*current.X + 2*omt*t*ctrl.X + t*t*dest.X,
					Y: omt*omt*current.Y + 2*omt*t*ctrl.Y + t*t*dest.Y,
				}
				pts = append(pts, p)
			}
			current = dest
		case gfx.PathCubicTo:
			if !haveStart {
				continue
			}
			c1 := seg.Pts[0]
			c2 := seg.Pts[1]
			dest := seg.Pts[2]
			const steps = 12
			for i := 1; i <= steps; i++ {
				t := float32(i) / steps
				omt := 1 - t
				p := gfx.Point{
					X: omt*omt*omt*current.X + 3*omt*omt*t*c1.X + 3*omt*t*t*c2.X + t*t*t*dest.X,
					Y: omt*omt*omt*current.Y + 3*omt*omt*t*c1.Y + 3*omt*t*t*c2.Y + t*t*t*dest.Y,
				}
				pts = append(pts, p)
			}
			current = dest
		case gfx.PathClose:
			if haveStart {
				pts = append(pts, start)
				flush()
				current = start
				haveStart = false
			}
		}
	}
	flush()
	return contours
}

func segmentDistance(p, a, b gfx.Point) float32 {
	ax := float64(a.X)
	ay := float64(a.Y)
	bx := float64(b.X)
	by := float64(b.Y)
	px := float64(p.X)
	py := float64(p.Y)
	dx := bx - ax
	dy := by - ay
	if dx == 0 && dy == 0 {
		return float32(math.Hypot(px-ax, py-ay))
	}
	t := ((px-ax)*dx + (py-ay)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	x := ax + t*dx
	y := ay + t*dy
	return float32(math.Hypot(px-x, py-y))
}

func strokeBrushFromMaterial(stroke theme.MaterialStroke, opacity float32) gfx.Brush {
	color := stroke.Paint.Color
	color = color.WithAlpha(color.A * stroke.Paint.Opacity * opacity)
	return gfx.SolidBrush(color)
}

func strokeStyle(stroke theme.MaterialStroke) gfx.StrokeStyle {
	style := gfx.DefaultStroke(stroke.Width)
	switch stroke.Cap {
	case theme.CapRound:
		style.Cap = gfx.LineCapRound
	case theme.CapSquare:
		style.Cap = gfx.LineCapSquare
	default:
		style.Cap = gfx.LineCapButt
	}
	switch stroke.Join {
	case theme.JoinRound:
		style.Join = gfx.LineJoinRound
	case theme.JoinBevel:
		style.Join = gfx.LineJoinBevel
	default:
		style.Join = gfx.LineJoinMiter
	}
	style.Dash = append([]float32(nil), stroke.Dash...)
	style.DashOffset = stroke.DashOffset
	return style
}

func projectMarkAt(mark marks.Mark, pos gfx.Point, ctx facet.ProjectionContext) *gfx.CommandList {
	switch m := mark.(type) {
	case *Label:
		if m == nil {
			return &gfx.CommandList{}
		}
		oldPlacement := m.Placement
		oldOffset := m.Offset
		m.Placement = LabelFree
		m.Offset = pos
		defer func() {
			m.Placement = oldPlacement
			m.Offset = oldOffset
		}()
		return m.project(ctx)
	case *basic.Text:
		if m == nil {
			return &gfx.CommandList{}
		}
		oldTx := m.Tx
		m.Tx.Transform = gfx.Translation(pos.X, pos.Y).Multiply(normalizeTransform(m.Tx.Transform))
		defer func() {
			m.Tx = oldTx
		}()
		role := m.Base().ProjectionRole()
		if role == nil {
			return &gfx.CommandList{}
		}
		return role.Project(ctx)
	case *SymbolInstance:
		if m == nil {
			return &gfx.CommandList{}
		}
		oldPos := m.Position
		m.Position = pos
		defer func() {
			m.Position = oldPos
		}()
		return m.project(ctx)
	case *Icon:
		if m == nil {
			return &gfx.CommandList{}
		}
		oldPos := m.Position
		m.Position = pos
		defer func() {
			m.Position = oldPos
		}()
		return m.project(ctx)
	case *Handle:
		if m == nil {
			return &gfx.CommandList{}
		}
		oldPos := m.Position
		m.Position = pos
		defer func() {
			m.Position = oldPos
		}()
		return m.project(ctx)
	default:
		if impl, ok := mark.(facet.FacetImpl); ok {
			role := impl.Base().ProjectionRole()
			if role != nil {
				return role.Project(ctx)
			}
		}
	}
	return &gfx.CommandList{}
}

func projectCommandsAt(cmds *gfx.CommandList, pos gfx.Point) *gfx.CommandList {
	if cmds == nil {
		return &gfx.CommandList{}
	}
	out := &gfx.CommandList{}
	if pos != (gfx.Point{}) {
		out.Add(gfx.PushTransform{Matrix: gfx.Translation(pos.X, pos.Y)})
	}
	out.Commands = append(out.Commands, cmds.Commands...)
	if pos != (gfx.Point{}) {
		out.Add(gfx.PopTransform{})
	}
	return out
}
