package svg

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	. "codeburg.org/lexbit/lurpicui/gfx"
)

// SVGPaintKind describes the normalized paint state for an SVG fill or stroke.
type SVGPaintKind uint8

const (
	SVGPaintUnset SVGPaintKind = iota
	SVGPaintNone
	SVGPaintCurrentColor
	SVGPaintColor
	SVGPaintLinearGradient
)

// SVGFillRule describes the normalized fill rule for a shape.
type SVGFillRule uint8

const (
	SVGFillRuleNonZero SVGFillRule = iota
	SVGFillRuleEvenOdd
)

// SVGGradientUnits describes how gradient coordinates are interpreted.
type SVGGradientUnits uint8

const (
	SVGGradientUnitsObjectBoundingBox SVGGradientUnits = iota
	SVGGradientUnitsUserSpaceOnUse
)

// SVGClipPathUnits describes how clip-path coordinates are interpreted.
type SVGClipPathUnits uint8

const (
	SVGClipPathUnitsUserSpaceOnUse SVGClipPathUnits = iota
	SVGClipPathUnitsObjectBoundingBox
)

// SVGAspectRatioAlign describes preserveAspectRatio alignment modes.
type SVGAspectRatioAlign uint8

const (
	SVGAspectRatioAlignUnspecified SVGAspectRatioAlign = iota
	SVGAspectRatioAlignNone
	SVGAspectRatioAlignXMinYMin
	SVGAspectRatioAlignXMidYMin
	SVGAspectRatioAlignXMaxYMin
	SVGAspectRatioAlignXMinYMid
	SVGAspectRatioAlignXMidYMid
	SVGAspectRatioAlignXMaxYMid
	SVGAspectRatioAlignXMinYMax
	SVGAspectRatioAlignXMidYMax
	SVGAspectRatioAlignXMaxYMax
)

// SVGMeetOrSlice describes the preserveAspectRatio fit mode.
type SVGMeetOrSlice uint8

const (
	SVGMeetOrSliceMeet SVGMeetOrSlice = iota
	SVGMeetOrSliceSlice
)

// SVGPreserveAspectRatio is the canonical preserveAspectRatio contract.
type SVGPreserveAspectRatio struct {
	Align       SVGAspectRatioAlign
	MeetOrSlice SVGMeetOrSlice
}

// SVGGradient is a normalized linear gradient definition.
type SVGGradient struct {
	ID        string
	Units     SVGGradientUnits
	Transform Transform
	Start     Point
	End       Point
	Stops     []GradientStop
}

// SVGPaint is the normalized fill or stroke paint contract.
type SVGPaint struct {
	Kind     SVGPaintKind
	Color    Color
	Gradient *SVGGradient
	Opacity  float32
}

// SVGStroke is the normalized stroke contract for a path.
type SVGStroke struct {
	Paint      SVGPaint
	Width      float32
	Cap        LineCap
	Join       LineJoin
	MiterLimit float32
	Dash       []float32
	DashOffset float32
}

// SVGClipPath is a normalized clip-path definition.
type SVGClipPath struct {
	ID     string
	Units  SVGClipPathUnits
	Path   Path
	Bounds Rect
}

// SVGDefinitionKind identifies a normalized SVG definition.
type SVGDefinitionKind uint8

const (
	SVGDefinitionGradient SVGDefinitionKind = iota
	SVGDefinitionClipPath
)

// SVGDefinition is a deterministic definition entry used by normalized SVG data.
type SVGDefinition struct {
	ID       string
	Kind     SVGDefinitionKind
	Gradient *SVGGradient
	ClipPath *SVGClipPath
}

// SVGElement is a flattened normalized SVG drawing element.
type SVGElement struct {
	ID       string
	Path     Path
	Fill     SVGPaint
	Stroke   *SVGStroke
	Opacity  float32
	FillRule SVGFillRule
	ClipPath *SVGClipPath
	Bounds   Rect
}

// SVGDocument is the canonical normalized SVG form.
type SVGDocument struct {
	ViewBox             Rect
	Width               float32
	Height              float32
	PreserveAspectRatio SVGPreserveAspectRatio
	Definitions         []SVGDefinition
	Elements            []SVGElement
	Bounds              Rect
}

type svgNode struct {
	Name     string
	Attrs    map[string]string
	Children []*svgNode
}

type svgStyleState struct {
	Fill     SVGPaint
	Stroke   *SVGStroke
	Opacity  float32
	FillRule SVGFillRule
	ClipPath string
}

// ParseSVG parses and normalizes SVG source into the canonical SVG document form.
func ParseSVG(data []byte) (SVGDocument, error) {
	return NormalizeSVG(data)
}

// ParseSVGString parses and normalizes an SVG document from a string.
func ParseSVGString(src string) (SVGDocument, error) {
	return NormalizeSVG([]byte(src))
}

// NormalizeSVG parses and normalizes SVG source into the canonical SVG document form.
func NormalizeSVG(data []byte) (SVGDocument, error) {
	root, err := parseSVGTree(bytes.NewReader(data))
	if err != nil {
		return SVGDocument{}, err
	}
	if root == nil || root.Name != "svg" {
		return SVGDocument{}, errors.New("svg: root element must be <svg>")
	}

	index := make(map[string]*svgNode)
	indexSVGNodes(root, index)

	normalizer := svgNormalizer{
		root:        root,
		nodeIndex:   index,
		definitions: make(map[string]SVGDefinition),
		visiting:    make(map[string]bool),
	}

	doc := SVGDocument{}
	if vb, ok, err := parseViewBox(root.Attrs["viewBox"]); err != nil {
		return SVGDocument{}, err
	} else if ok {
		doc.ViewBox = vb
	}
	if width, ok, err := parseLength(root.Attrs["width"]); err != nil {
		return SVGDocument{}, err
	} else if ok {
		doc.Width = width
	}
	if height, ok, err := parseLength(root.Attrs["height"]); err != nil {
		return SVGDocument{}, err
	} else if ok {
		doc.Height = height
	}
	doc.PreserveAspectRatio, err = parsePreserveAspectRatio(root.Attrs["preserveAspectRatio"])
	if err != nil {
		return SVGDocument{}, err
	}
	if doc.ViewBox.IsEmpty() && doc.Width > 0 && doc.Height > 0 {
		doc.ViewBox = RectFromXYWH(0, 0, doc.Width, doc.Height)
	}

	rootState := defaultSVGStyleState()
	rootState, err = applyStyleAttrs(root.Attrs, rootState)
	if err != nil {
		return SVGDocument{}, err
	}
	rootTransform := parseTransformAttr(root.Attrs["transform"])
	if err := normalizer.collectDefinitions(root, rootState, rootTransform); err != nil {
		return SVGDocument{}, err
	}
	if err := normalizer.collectVisible(root, rootState, rootTransform, rootState.ClipPath, &doc); err != nil {
		return SVGDocument{}, err
	}

	defs := make([]SVGDefinition, 0, len(normalizer.definitions))
	for _, def := range normalizer.definitions {
		defs = append(defs, def)
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].ID < defs[j].ID
	})
	doc.Definitions = defs

	for _, el := range doc.Elements {
		doc.Bounds = doc.Bounds.Union(el.Bounds)
	}
	return doc, nil
}

type svgNormalizer struct {
	root        *svgNode
	nodeIndex   map[string]*svgNode
	definitions map[string]SVGDefinition
	visiting    map[string]bool
}

func (n *svgNormalizer) collectDefinitions(node *svgNode, state svgStyleState, transform Transform) error {
	if node == nil {
		return nil
	}
	type walkFrame struct {
		node      *svgNode
		state     svgStyleState
		transform Transform
	}
	stack := []walkFrame{{node: node, state: state, transform: transform}}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if frame.node == nil {
			continue
		}
		switch frame.node.Name {
		case "svg", "g":
			nextState := frame.state
			nextTransform := frame.transform
			if frame.node != n.root {
				var err error
				nextState, err = applyStyleAttrs(frame.node.Attrs, frame.state)
				if err != nil {
					return err
				}
				nextTransform = frame.transform.Multiply(parseTransformAttr(frame.node.Attrs["transform"]))
			}
			for i := len(frame.node.Children) - 1; i >= 0; i-- {
				stack = append(stack, walkFrame{
					node:      frame.node.Children[i],
					state:     nextState,
					transform: nextTransform,
				})
			}
		case "defs":
			for i := len(frame.node.Children) - 1; i >= 0; i-- {
				stack = append(stack, walkFrame{
					node:      frame.node.Children[i],
					state:     frame.state,
					transform: frame.transform,
				})
			}
		case "clipPath":
			def, err := n.normalizeClipPath(frame.node, frame.state, frame.transform)
			if err != nil {
				return err
			}
			n.definitions[def.ID] = def
		case "linearGradient":
			def, err := n.normalizeLinearGradient(frame.node, frame.transform)
			if err != nil {
				return err
			}
			n.definitions[def.ID] = def
		default:
			// Shapes and use nodes are not definitions, but they may carry ids.
			if id := frame.node.Attrs["id"]; id != "" && !isDefinitionOnlyNode(frame.node.Name) {
				// Keep the node indexed for <use> resolution.
			}
		}
	}
	return nil
}

func (n *svgNormalizer) collectDefinitionChildren(defs *svgNode, state svgStyleState, transform Transform) error {
	for _, child := range defs.Children {
		switch child.Name {
		case "clipPath":
			def, err := n.normalizeClipPath(child, state, transform)
			if err != nil {
				return err
			}
			n.definitions[def.ID] = def
		case "linearGradient":
			def, err := n.normalizeLinearGradient(child, transform)
			if err != nil {
				return err
			}
			n.definitions[def.ID] = def
		default:
			if err := n.collectDefinitions(child, state, transform); err != nil {
				return err
			}
		}
	}
	return nil
}

func (n *svgNormalizer) collectVisible(node *svgNode, state svgStyleState, transform Transform, clipRef string, doc *SVGDocument) error {
	if node == nil {
		return nil
	}
	type walkFrame struct {
		node      *svgNode
		state     svgStyleState
		transform Transform
		clipRef   string
		cleanup   string
	}
	stack := []walkFrame{{node: node, state: state, transform: transform, clipRef: clipRef}}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if frame.cleanup != "" {
			delete(n.visiting, frame.cleanup)
			continue
		}
		if frame.node == nil {
			continue
		}
		switch frame.node.Name {
		case "svg", "g":
			nextState := frame.state
			nextTransform := frame.transform
			nextClip := frame.clipRef
			if frame.node != n.root {
				var err error
				nextState, err = applyStyleAttrs(frame.node.Attrs, frame.state)
				if err != nil {
					return err
				}
				nextTransform = frame.transform.Multiply(parseTransformAttr(frame.node.Attrs["transform"]))
				if clip := frame.node.Attrs["clip-path"]; clip != "" {
					ref, err := parseURLReference(clip)
					if err != nil {
						return err
					}
					nextClip = ref
				}
			}
			for i := len(frame.node.Children) - 1; i >= 0; i-- {
				child := frame.node.Children[i]
				if child == nil || child.Name == "defs" {
					continue
				}
				stack = append(stack, walkFrame{
					node:      child,
					state:     nextState,
					transform: nextTransform,
					clipRef:   nextClip,
				})
			}
		case "defs", "clipPath", "linearGradient", "stop", "title", "desc":
			continue
		case "use":
			target, nextState, nextTransform, nextClip, ref, err := n.prepareUseExpansion(frame.node, frame.state, frame.transform, frame.clipRef)
			if err != nil {
				return err
			}
			stack = append(stack, walkFrame{cleanup: ref})
			stack = append(stack, walkFrame{
				node:      target,
				state:     nextState,
				transform: nextTransform,
				clipRef:   nextClip,
			})
		default:
			if isShapeNode(frame.node.Name) {
				el, err := n.normalizeShape(frame.node, frame.state, frame.transform, frame.clipRef)
				if err != nil {
					return err
				}
				doc.Elements = append(doc.Elements, el)
				continue
			}
			return fmt.Errorf("svg: unsupported element <%s>", frame.node.Name)
		}
	}
	return nil
}

func (n *svgNormalizer) normalizeClipPath(node *svgNode, state svgStyleState, transform Transform) (SVGDefinition, error) {
	id := node.Attrs["id"]
	if id == "" {
		return SVGDefinition{}, errors.New("svg: clipPath requires id")
	}
	units := SVGClipPathUnitsUserSpaceOnUse
	if s := node.Attrs["clipPathUnits"]; s != "" {
		switch s {
		case "userSpaceOnUse":
			units = SVGClipPathUnitsUserSpaceOnUse
		case "objectBoundingBox":
			units = SVGClipPathUnitsObjectBoundingBox
		default:
			return SVGDefinition{}, fmt.Errorf("svg: unsupported clipPathUnits %q", s)
		}
	}
	clip := &SVGClipPath{ID: id, Units: units}
	var bounds Rect
	for _, child := range node.Children {
		els, err := n.collectNodeElements(child, state, transform, "", false)
		if err != nil {
			return SVGDefinition{}, err
		}
		for _, el := range els {
			clip.Path.Segments = append(clip.Path.Segments, el.Path.Segments...)
			bounds = bounds.Union(el.Bounds)
		}
	}
	clip.Bounds = bounds
	return SVGDefinition{ID: id, Kind: SVGDefinitionClipPath, ClipPath: clip}, nil
}

func (n *svgNormalizer) normalizeLinearGradient(node *svgNode, transform Transform) (SVGDefinition, error) {
	id := node.Attrs["id"]
	if id == "" {
		return SVGDefinition{}, errors.New("svg: linearGradient requires id")
	}
	grad := &SVGGradient{ID: id, Units: SVGGradientUnitsObjectBoundingBox, Transform: transform}
	if s := node.Attrs["gradientUnits"]; s != "" {
		switch s {
		case "objectBoundingBox":
			grad.Units = SVGGradientUnitsObjectBoundingBox
		case "userSpaceOnUse":
			grad.Units = SVGGradientUnitsUserSpaceOnUse
		default:
			return SVGDefinition{}, fmt.Errorf("svg: unsupported gradientUnits %q", s)
		}
	}
	if x1, ok, err := parseLength(node.Attrs["x1"]); err != nil {
		return SVGDefinition{}, err
	} else if ok {
		grad.Start.X = x1
	}
	if y1, ok, err := parseLength(node.Attrs["y1"]); err != nil {
		return SVGDefinition{}, err
	} else if ok {
		grad.Start.Y = y1
	}
	if x2, ok, err := parseLength(node.Attrs["x2"]); err != nil {
		return SVGDefinition{}, err
	} else if ok {
		grad.End.X = x2
	}
	if y2, ok, err := parseLength(node.Attrs["y2"]); err != nil {
		return SVGDefinition{}, err
	} else if ok {
		grad.End.Y = y2
	}
	for _, child := range node.Children {
		if child.Name != "stop" {
			return SVGDefinition{}, fmt.Errorf("svg: unsupported child <%s> inside linearGradient", child.Name)
		}
		stop, err := parseGradientStop(child)
		if err != nil {
			return SVGDefinition{}, err
		}
		grad.Stops = append(grad.Stops, stop)
	}
	return SVGDefinition{ID: id, Kind: SVGDefinitionGradient, Gradient: grad}, nil
}

func parseGradientStop(node *svgNode) (GradientStop, error) {
	stop := GradientStop{}
	if s := node.Attrs["offset"]; s != "" {
		v, err := parsePercentOrNumber(s)
		if err != nil {
			return GradientStop{}, err
		}
		stop.Offset = v
	}
	colorValue := node.Attrs["stop-color"]
	if colorValue == "" {
		colorValue = "#000000"
	}
	paint, err := parsePaint(colorValue)
	if err != nil {
		return GradientStop{}, err
	}
	if paint.Kind == SVGPaintNone {
		paint.Kind = SVGPaintColor
		paint.Color = Color{}
	}
	if opacity := node.Attrs["stop-opacity"]; opacity != "" {
		v, err := parseOpacity(opacity)
		if err != nil {
			return GradientStop{}, err
		}
		paint.Opacity *= v
	}
	stop.Color = paintColorForGradient(paint)
	return stop, nil
}

func paintColorForGradient(p SVGPaint) Color {
	switch p.Kind {
	case SVGPaintCurrentColor:
		return Color{A: p.Opacity}
	case SVGPaintColor:
		return p.Color.WithAlpha(p.Color.A * p.Opacity)
	default:
		return Color{}
	}
}

func (n *svgNormalizer) normalizeShape(node *svgNode, state svgStyleState, transform Transform, clipRef string) (SVGElement, error) {
	nextState, err := applyStyleAttrs(node.Attrs, state)
	if err != nil {
		return SVGElement{}, err
	}
	localTransform := transform.Multiply(parseTransformAttr(node.Attrs["transform"]))
	if clip := node.Attrs["clip-path"]; clip != "" {
		ref, err := parseURLReference(clip)
		if err != nil {
			return SVGElement{}, err
		}
		clipRef = ref
	}
	id := node.Attrs["id"]
	el := SVGElement{
		ID:       id,
		Fill:     nextState.Fill,
		Stroke:   cloneStroke(nextState.Stroke),
		Opacity:  nextState.Opacity,
		FillRule: nextState.FillRule,
	}
	if clipRef != "" {
		clipDef, ok := n.definitions[clipRef]
		if !ok || clipDef.ClipPath == nil {
			return SVGElement{}, fmt.Errorf("svg: unknown clip-path %q", clipRef)
		}
		cp := *clipDef.ClipPath
		el.ClipPath = &cp
	}

	path, err := n.shapeToPath(node)
	if err != nil {
		return SVGElement{}, err
	}
	path = Transformed(path, localTransform)
	el.Path = path
	el.Bounds = Bounds(path)
	if el.Stroke != nil && el.Stroke.Width > 0 {
		el.Bounds = el.Bounds.Inset(-el.Stroke.Width/2, -el.Stroke.Width/2)
	}
	if el.ClipPath != nil && !el.ClipPath.Bounds.IsEmpty() {
		el.Bounds = intersectRects(el.Bounds, el.ClipPath.Bounds)
	}
	return el, nil
}

func (n *svgNormalizer) collectNodeElements(node *svgNode, state svgStyleState, transform Transform, clipRef string, emit bool) ([]SVGElement, error) {
	if node == nil {
		return nil, nil
	}
	type walkFrame struct {
		node      *svgNode
		state     svgStyleState
		transform Transform
		clipRef   string
		cleanup   string
	}
	out := make([]SVGElement, 0, 8)
	stack := []walkFrame{{node: node, state: state, transform: transform, clipRef: clipRef}}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if frame.cleanup != "" {
			delete(n.visiting, frame.cleanup)
			continue
		}
		if frame.node == nil {
			continue
		}
		switch frame.node.Name {
		case "svg", "g":
			nextState := frame.state
			nextTransform := frame.transform
			nextClip := frame.clipRef
			if frame.node != n.root {
				var err error
				nextState, err = applyStyleAttrs(frame.node.Attrs, frame.state)
				if err != nil {
					return nil, err
				}
				nextTransform = frame.transform.Multiply(parseTransformAttr(frame.node.Attrs["transform"]))
				if clip := frame.node.Attrs["clip-path"]; clip != "" {
					ref, err := parseURLReference(clip)
					if err != nil {
						return nil, err
					}
					nextClip = ref
				}
			}
			for i := len(frame.node.Children) - 1; i >= 0; i-- {
				child := frame.node.Children[i]
				if child == nil || child.Name == "defs" {
					continue
				}
				stack = append(stack, walkFrame{
					node:      child,
					state:     nextState,
					transform: nextTransform,
					clipRef:   nextClip,
				})
			}
		case "defs", "title", "desc", "clipPath", "linearGradient", "stop":
			continue
		case "use":
			target, nextState, nextTransform, nextClip, ref, err := n.prepareUseExpansion(frame.node, frame.state, frame.transform, frame.clipRef)
			if err != nil {
				return nil, err
			}
			stack = append(stack, walkFrame{cleanup: ref})
			stack = append(stack, walkFrame{
				node:      target,
				state:     nextState,
				transform: nextTransform,
				clipRef:   nextClip,
			})
		default:
			if isShapeNode(frame.node.Name) {
				el, err := n.normalizeShape(frame.node, frame.state, frame.transform, frame.clipRef)
				if err != nil {
					return nil, err
				}
				out = append(out, el)
				continue
			}
			return nil, fmt.Errorf("svg: unsupported element <%s>", frame.node.Name)
		}
	}
	return out, nil
}

func (n *svgNormalizer) prepareUseExpansion(node *svgNode, state svgStyleState, transform Transform, clipRef string) (*svgNode, svgStyleState, Transform, string, string, error) {
	useRef, err := useReference(node)
	if err != nil {
		return nil, svgStyleState{}, Transform{}, "", "", err
	}
	target, ok := n.nodeIndex[useRef]
	if !ok {
		return nil, svgStyleState{}, Transform{}, "", "", fmt.Errorf("svg: unknown use reference %q", useRef)
	}
	if n.visiting[useRef] {
		return nil, svgStyleState{}, Transform{}, "", "", fmt.Errorf("svg: use reference cycle involving %q", useRef)
	}
	n.visiting[useRef] = true

	localTransform := transform.Multiply(parseTransformAttr(node.Attrs["transform"]))
	if x, ok, err := parseLength(node.Attrs["x"]); err != nil {
		delete(n.visiting, useRef)
		return nil, svgStyleState{}, Transform{}, "", "", err
	} else if ok {
		if y, ok, err := parseLength(node.Attrs["y"]); err != nil {
			delete(n.visiting, useRef)
			return nil, svgStyleState{}, Transform{}, "", "", err
		} else if ok {
			localTransform = localTransform.Multiply(Translation(x, y))
		} else {
			localTransform = localTransform.Multiply(Translation(x, 0))
		}
	} else if y, ok, err := parseLength(node.Attrs["y"]); err != nil {
		delete(n.visiting, useRef)
		return nil, svgStyleState{}, Transform{}, "", "", err
	} else if ok {
		localTransform = localTransform.Multiply(Translation(0, y))
	}

	nextState, err := applyStyleAttrs(node.Attrs, state)
	if err != nil {
		delete(n.visiting, useRef)
		return nil, svgStyleState{}, Transform{}, "", "", err
	}
	nextClip := clipRef
	if clip := node.Attrs["clip-path"]; clip != "" {
		clipRefValue, err := parseURLReference(clip)
		if err != nil {
			delete(n.visiting, useRef)
			return nil, svgStyleState{}, Transform{}, "", "", err
		}
		nextClip = clipRefValue
	}
	return target, nextState, localTransform, nextClip, useRef, nil
}

func (n *svgNormalizer) shapeToPath(node *svgNode) (Path, error) {
	switch node.Name {
	case "path":
		d := node.Attrs["d"]
		if d == "" {
			return Path{}, errors.New("svg: path requires d")
		}
		return parsePathData(d)
	case "rect":
		x := float32(0)
		y := float32(0)
		if v, ok, err := parseLength(node.Attrs["x"]); err != nil {
			return Path{}, err
		} else if ok {
			x = v
		}
		if v, ok, err := parseLength(node.Attrs["y"]); err != nil {
			return Path{}, err
		} else if ok {
			y = v
		}
		w, ok, err := parseLength(node.Attrs["width"])
		if err != nil {
			return Path{}, err
		}
		if !ok || w <= 0 {
			return Path{}, errors.New("svg: rect requires positive width")
		}
		h, ok, err := parseLength(node.Attrs["height"])
		if err != nil {
			return Path{}, err
		}
		if !ok || h <= 0 {
			return Path{}, errors.New("svg: rect requires positive height")
		}
		rx, _, err := parseLength(node.Attrs["rx"])
		if err != nil {
			return Path{}, err
		}
		ry, _, err := parseLength(node.Attrs["ry"])
		if err != nil {
			return Path{}, err
		}
		if rx == 0 && ry == 0 {
			return RectPath(RectFromXYWH(x, y, w, h)), nil
		}
		if rx == 0 {
			rx = ry
		}
		if ry == 0 {
			ry = rx
		}
		if rx > w/2 {
			rx = w / 2
		}
		if ry > h/2 {
			ry = h / 2
		}
		return roundedRectPath(RectFromXYWH(x, y, w, h), rx, ry), nil
	case "circle":
		cx := float32(0)
		cy := float32(0)
		r, ok, err := parseLength(node.Attrs["r"])
		if err != nil {
			return Path{}, err
		}
		if !ok || r <= 0 {
			return Path{}, errors.New("svg: circle requires positive r")
		}
		if v, ok, err := parseLength(node.Attrs["cx"]); err != nil {
			return Path{}, err
		} else if ok {
			cx = v
		}
		if v, ok, err := parseLength(node.Attrs["cy"]); err != nil {
			return Path{}, err
		} else if ok {
			cy = v
		}
		return ellipsePath(Point{X: cx, Y: cy}, r, r), nil
	case "ellipse":
		cx := float32(0)
		cy := float32(0)
		rx, ok, err := parseLength(node.Attrs["rx"])
		if err != nil {
			return Path{}, err
		}
		if !ok || rx <= 0 {
			return Path{}, errors.New("svg: ellipse requires positive rx")
		}
		ry, ok, err := parseLength(node.Attrs["ry"])
		if err != nil {
			return Path{}, err
		}
		if !ok || ry <= 0 {
			return Path{}, errors.New("svg: ellipse requires positive ry")
		}
		if v, ok, err := parseLength(node.Attrs["cx"]); err != nil {
			return Path{}, err
		} else if ok {
			cx = v
		}
		if v, ok, err := parseLength(node.Attrs["cy"]); err != nil {
			return Path{}, err
		} else if ok {
			cy = v
		}
		return ellipsePath(Point{X: cx, Y: cy}, rx, ry), nil
	case "line":
		x1, ok, err := parseLength(node.Attrs["x1"])
		if err != nil {
			return Path{}, err
		}
		if !ok {
			return Path{}, errors.New("svg: line requires x1")
		}
		y1, ok, err := parseLength(node.Attrs["y1"])
		if err != nil {
			return Path{}, err
		}
		if !ok {
			return Path{}, errors.New("svg: line requires y1")
		}
		x2, ok, err := parseLength(node.Attrs["x2"])
		if err != nil {
			return Path{}, err
		}
		if !ok {
			return Path{}, errors.New("svg: line requires x2")
		}
		y2, ok, err := parseLength(node.Attrs["y2"])
		if err != nil {
			return Path{}, err
		}
		if !ok {
			return Path{}, errors.New("svg: line requires y2")
		}
		return NewPath().MoveTo(Point{X: x1, Y: y1}).LineTo(Point{X: x2, Y: y2}).Build(), nil
	case "polyline", "polygon":
		pts, err := parsePoints(node.Attrs["points"])
		if err != nil {
			return Path{}, err
		}
		if node.Name == "polygon" {
			return PolylinePath(pts, true), nil
		}
		return PolylinePath(pts, false), nil
	default:
		return Path{}, fmt.Errorf("svg: unsupported geometry element <%s>", node.Name)
	}
}
