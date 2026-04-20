package stack

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// Axis selects the main axis for the stack policy.
type Axis uint8

const (
	Vertical Axis = iota
	Horizontal
)

// MainAlignment controls how extra main-axis space is distributed.
type MainAlignment uint8

const (
	MainStart MainAlignment = iota
	MainCenter
	MainEnd
	MainSpaceBetween
	MainSpaceAround
)

// Config is the stack policy configuration.
type Config struct {
	Axis       Axis
	Spacing    float32
	MainAlign  MainAlignment
	CrossAlign layout.Alignment
}

// Policy arranges children sequentially along a main axis.
type Policy struct {
	cfg Config
}

// New constructs a stack policy.
func New(cfg Config) *Policy {
	return &Policy{cfg: cfg}
}

// Measure computes the preferred size of the stack.
func (p *Policy) Measure(children []layout.ChildNode, constraints gfx.Size) gfx.Size {
	if p == nil {
		return gfx.Size{}
	}
	measured, _, _ := p.measure(children, constraints, false)
	return measured
}

// Arrange positions children within the resolved layer bounds.
func (p *Policy) Arrange(children []layout.ChildNode, layer layout.ResolvedLayer) {
	if p == nil || len(children) == 0 {
		return
	}
	p.arrange(children, layer)
}

func (p *Policy) measure(children []layout.ChildNode, constraints gfx.Size, arrangement bool) (gfx.Size, float32, float32) {
	if len(children) == 0 {
		return gfx.Size{}, 0, 0
	}
	horizontal := p.cfg.Axis == Horizontal
	mainAvail := axisSize(constraints, horizontal)
	count := len(children)
	baseGap := p.cfg.Spacing * float32(maxInt(0, count-1))

	var baseMainSum float32
	var crossMax float32
	var totalFlex float32
	for i := range children {
		childMain := childMainSize(children[i], horizontal)
		childMinMain := childMinMainSize(children[i], horizontal)
		if childMain < childMinMain {
			childMain = childMinMain
		}
		baseMainSum += childMain
		cross := childCrossSize(children[i], horizontal)
		if minCross := childMinCrossSize(children[i], horizontal); cross < minCross {
			cross = minCross
		}
		if cross > crossMax {
			crossMax = cross
		}
		if flex := children[i].Attachment.Placement.Flex; flex > 0 {
			totalFlex += flex
		}
	}

	totalMain := baseMainSum + baseGap
	if totalFlex > 0 && mainAvail > 0 {
		extra := mainAvail - totalMain
		if extra > 0 {
			totalMain += extra
		}
	}

	size := gfx.Size{}
	if horizontal {
		size.W = totalMain
		size.H = crossMax
	} else {
		size.W = crossMax
		size.H = totalMain
	}
	if arrangement {
		return size, baseMainSum, totalFlex
	}
	return size, baseMainSum, totalFlex
}

func (p *Policy) arrange(children []layout.ChildNode, layer layout.ResolvedLayer) {
	horizontal := p.cfg.Axis == Horizontal
	mainAvail := axisRectSize(layer.Bounds, horizontal)
	crossAvail := axisRectCrossSize(layer.Bounds, horizontal)
	count := len(children)
	baseGap := p.cfg.Spacing * float32(maxInt(0, count-1))

	var baseMainSum float32
	var totalFlex float32
	for i := range children {
		childMain := childMainSize(children[i], horizontal)
		if minMain := childMinMainSize(children[i], horizontal); childMain < minMain {
			childMain = minMain
		}
		baseMainSum += childMain
		if flex := children[i].Attachment.Placement.Flex; flex > 0 {
			totalFlex += flex
		}
	}

	extraMain := float32(0)
	if totalFlex > 0 && mainAvail > 0 {
		extraMain = mainAvail - (baseMainSum + baseGap)
		if extraMain < 0 {
			extraMain = 0
		}
	}

	startOffset, gap := p.mainAlignmentOffsets(count, mainAvail, baseMainSum, baseGap, extraMain)
	pos := startOffset
	for i := range children {
		child := children[i]
		main := childMainSize(child, horizontal)
		if minMain := childMinMainSize(child, horizontal); main < minMain {
			main = minMain
		}
		if flex := child.Attachment.Placement.Flex; flex > 0 && totalFlex > 0 {
			main += extraMain * (flex / totalFlex)
		}

		cross := childCrossSize(child, horizontal)
		if minCross := childMinCrossSize(child, horizontal); cross < minCross {
			cross = minCross
		}
		crossPos, crossSize := alignCross(p.cfg.CrossAlign, horizontal, layer.Bounds, crossAvail, cross)

		var rect gfx.Rect
		if horizontal {
			rect = gfx.RectFromXYWH(layer.Bounds.Min.X+pos, crossPos, main, crossSize)
		} else {
			rect = gfx.RectFromXYWH(crossPos, layer.Bounds.Min.Y+pos, crossSize, main)
		}
		children[i].SetArrangedBounds(rect)
		pos += main + gap
	}
}

func (p *Policy) mainAlignmentOffsets(count int, mainAvail, baseMainSum, baseGap, extraMain float32) (startOffset, gap float32) {
	contentMain := baseMainSum + baseGap + extraMain
	residual := mainAvail - contentMain
	if residual < 0 {
		residual = 0
	}
	switch p.cfg.MainAlign {
	case MainCenter:
		startOffset = residual / 2
	case MainEnd:
		startOffset = residual
	case MainSpaceBetween:
		if count > 1 {
			gap = baseGap + residual/float32(count-1)
		} else {
			gap = baseGap
		}
	case MainSpaceAround:
		if count > 0 {
			gap = baseGap + residual/float32(count)
			startOffset = gap / 2
		}
	default:
		gap = baseGap
	}
	if p.cfg.MainAlign == MainStart || p.cfg.MainAlign == MainCenter || p.cfg.MainAlign == MainEnd {
		gap = baseGap
	}
	return startOffset, gap
}

func childMainSize(child layout.ChildNode, horizontal bool) float32 {
	if horizontal {
		return child.IntrinsicSize.W
	}
	return child.IntrinsicSize.H
}

func childCrossSize(child layout.ChildNode, horizontal bool) float32 {
	if horizontal {
		return child.IntrinsicSize.H
	}
	return child.IntrinsicSize.W
}

func childMinMainSize(child layout.ChildNode, horizontal bool) float32 {
	if horizontal {
		return child.MinSize.W
	}
	return child.MinSize.H
}

func childMinCrossSize(child layout.ChildNode, horizontal bool) float32 {
	if horizontal {
		return child.MinSize.H
	}
	return child.MinSize.W
}

func axisSize(size gfx.Size, horizontal bool) float32 {
	if horizontal {
		return size.W
	}
	return size.H
}

func axisRectSize(rect gfx.Rect, horizontal bool) float32 {
	if horizontal {
		return rect.Width()
	}
	return rect.Height()
}

func axisRectCrossSize(rect gfx.Rect, horizontal bool) float32 {
	if horizontal {
		return rect.Height()
	}
	return rect.Width()
}

func alignCross(a layout.Alignment, horizontal bool, bounds gfx.Rect, available, childSize float32) (pos, size float32) {
	switch crossDisposition(a, horizontal) {
	case crossEnd:
		if available > childSize {
			pos = crossOrigin(bounds, horizontal) + (available - childSize)
		} else {
			pos = crossOrigin(bounds, horizontal)
		}
		size = childSize
	case crossCenter:
		if available > childSize {
			pos = crossOrigin(bounds, horizontal) + (available-childSize)/2
		} else {
			pos = crossOrigin(bounds, horizontal)
		}
		size = childSize
	case crossStretch:
		pos = crossOrigin(bounds, horizontal)
		if available > childSize {
			size = available
		} else {
			size = childSize
		}
	default:
		pos = crossOrigin(bounds, horizontal)
		size = childSize
	}
	return pos, size
}

type crossMode uint8

const (
	crossStart crossMode = iota
	crossCenter
	crossEnd
	crossStretch
)

func crossDisposition(a layout.Alignment, horizontal bool) crossMode {
	switch a {
	case layout.AlignStretch:
		return crossStretch
	case layout.AlignBaseline:
		return crossStart
	}
	if horizontal {
		switch a {
		case layout.AlignCenter, layout.AlignCenterLeft, layout.AlignCenterRight:
			return crossCenter
		case layout.AlignEnd, layout.AlignBottomLeft, layout.AlignBottomCenter, layout.AlignBottomRight:
			return crossEnd
		default:
			return crossStart
		}
	}
	switch a {
	case layout.AlignCenter, layout.AlignTopCenter, layout.AlignBottomCenter:
		return crossCenter
	case layout.AlignEnd, layout.AlignTopRight, layout.AlignCenterRight, layout.AlignBottomRight:
		return crossEnd
	default:
		return crossStart
	}
}

func crossOrigin(bounds gfx.Rect, horizontal bool) float32 {
	if horizontal {
		return bounds.Min.Y
	}
	return bounds.Min.X
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
