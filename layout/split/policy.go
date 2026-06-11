package split

import (
	"math"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// Axis selects the split direction.
type Axis uint8

const (
	Horizontal Axis = iota
	Vertical
)

// PaneSizing describes how a pane gets its main-axis size.
type PaneSizing uint8

const (
	PaneFixed PaneSizing = iota
	PaneWeighted
	PaneIntrinsic
)

// Config is the split policy configuration.
type Config struct {
	Axis        Axis
	DividerSize float32
}

// Policy divides a region into ordered panes.
type Policy struct {
	cfg Config
}

// New constructs a split policy.
func New(cfg Config) *Policy {
	return &Policy{cfg: cfg}
}

// Measure computes the preferred size of the split.
func (p *Policy) Measure(children []layout.ChildNode, constraints gfx.Size) gfx.Size {
	if p == nil || len(children) == 0 {
		return gfx.Size{}
	}

	horizontal := p.cfg.Axis == Horizontal
	var mainBase float32
	var crossMax float32
	var weightedMinTotal float32
	var weightedCount int

	for i := range children {
		child := children[i]
		cross := crossSize(child, horizontal)
		if min := minCrossSize(child, horizontal); cross < min {
			cross = min
		}
		if cross > crossMax {
			crossMax = cross
		}
		switch paneSizing(child) {
		case PaneWeighted:
			weightedCount++
			weightedMinTotal += minMainSize(child, horizontal)
		default:
			mainBase += declaredMainSize(child, horizontal)
		}
	}

	mainBase += p.dividerTotal(len(children))
	mainTotal := mainBase + weightedMinTotal
	mainAvail := axisSize(constraints, horizontal)
	if weightedCount > 0 && mainAvail > mainTotal {
		mainTotal = mainAvail
	}

	if horizontal {
		return gfx.Size{W: mainTotal, H: crossMax}
	}
	return gfx.Size{W: crossMax, H: mainTotal}
}

// Arrange positions panes within the resolved layer.
func (p *Policy) Arrange(children []layout.ChildNode, layer layout.ResolvedLayer) {
	if p == nil || len(children) == 0 {
		return
	}

	horizontal := p.cfg.Axis == Horizontal
	mainAvail := axisRectSize(layer.Bounds, horizontal)
	remaining := mainAvail - p.dividerTotal(len(children))
	if remaining < 0 {
		remaining = 0
	}

	var fixedTotal float32
	var weightedMinTotal float32
	var weightedWeightTotal float32
	var weightedCount int

	for i := range children {
		child := children[i]
		switch paneSizing(child) {
		case PaneWeighted:
			weightedCount++
			weightedWeightTotal += weightOf(child)
			weightedMinTotal += minMainSize(child, horizontal)
		default:
			size := declaredMainSize(child, horizontal)
			if min := minMainSize(child, horizontal); size < min {
				size = min
			}
			fixedTotal += size
		}
	}

	remaining -= fixedTotal
	if remaining < 0 {
		remaining = 0
	}

	weightedScale := float32(0)
	if weightedCount > 0 {
		if remaining <= weightedMinTotal || weightedWeightTotal <= 0 {
			weightedScale = -1
		} else {
			weightedScale = solveWeightedScale(children, horizontal, remaining)
		}
	}

	pos := axisOrigin(layer.Bounds, horizontal)
	for i := range children {
		child := children[i]
		main := declaredMainSize(child, horizontal)
		switch paneSizing(child) {
		case PaneFixed:
			if min := minMainSize(child, horizontal); main < min {
				main = min
			}
		case PaneWeighted:
			if weightedScale < 0 {
				main = minMainSize(child, horizontal)
			} else {
				main = weightOf(child) * weightedScale
				if min := minMainSize(child, horizontal); main < min {
					main = min
				}
			}
		case PaneIntrinsic:
			if min := minMainSize(child, horizontal); main < min {
				main = min
			}
		}

		cross := crossSize(child, horizontal)
		if min := minCrossSize(child, horizontal); cross < min {
			cross = min
		}

		var rect gfx.Rect
		if horizontal {
			rect = gfx.RectFromXYWH(pos, layer.Bounds.Min.Y, main, cross)
		} else {
			rect = gfx.RectFromXYWH(layer.Bounds.Min.X, pos, cross, main)
		}
		children[i].SetArrangedBounds(rect) //nolint:gosec // slice bounds verified upstream

		pos += main
		if i < len(children)-1 {
			pos += p.cfg.DividerSize
		}
	}
}

func solveWeightedScale(children []layout.ChildNode, horizontal bool, remaining float32) float32 {
	low := float32(0)
	high := remaining
	for i := range children {
		if paneSizing(children[i]) == PaneWeighted {
			if w := weightOf(children[i]); w > 0 {
				if candidate := remaining / w; candidate > high {
					high = candidate
				}
			}
		}
	}
	if high <= 0 {
		return 0
	}
	for iter := 0; iter < 32; iter++ {
		mid := (low + high) / 2
		sum := weightedSizeSum(children, horizontal, mid)
		if sum > remaining {
			high = mid
		} else {
			low = mid
		}
	}
	return low
}

func weightedSizeSum(children []layout.ChildNode, horizontal bool, scale float32) float32 {
	sum := float32(0)
	for i := range children {
		child := children[i]
		if paneSizing(child) != PaneWeighted {
			continue
		}
		size := weightOf(child) * scale
		if min := minMainSize(child, horizontal); size < min {
			size = min
		}
		sum += size
	}
	return sum
}

func paneSizing(child layout.ChildNode) PaneSizing {
	if child.Attachment.Placement.Flex > 0 {
		return PaneWeighted
	}
	if child.Attachment.Placement.Offset.X != 0 || child.Attachment.Placement.Offset.Y != 0 {
		return PaneFixed
	}
	return PaneIntrinsic
}

func declaredMainSize(child layout.ChildNode, horizontal bool) float32 {
	if paneSizing(child) == PaneFixed {
		if horizontal {
			return abs(child.Attachment.Placement.Offset.X)
		}
		return abs(child.Attachment.Placement.Offset.Y)
	}
	return mainSize(child, horizontal)
}

func weightOf(child layout.ChildNode) float32 {
	if child.Attachment.Placement.Flex <= 0 {
		return 0
	}
	return child.Attachment.Placement.Flex
}

func mainSize(child layout.ChildNode, horizontal bool) float32 {
	if horizontal {
		return child.IntrinsicSize.W
	}
	return child.IntrinsicSize.H
}

func crossSize(child layout.ChildNode, horizontal bool) float32 {
	if horizontal {
		return child.IntrinsicSize.H
	}
	return child.IntrinsicSize.W
}

func minMainSize(child layout.ChildNode, horizontal bool) float32 {
	if horizontal {
		return child.MinSize.W
	}
	return child.MinSize.H
}

func minCrossSize(child layout.ChildNode, horizontal bool) float32 {
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

func axisOrigin(rect gfx.Rect, horizontal bool) float32 {
	if horizontal {
		return rect.Min.X
	}
	return rect.Min.Y
}

func (p *Policy) dividerTotal(count int) float32 {
	if count <= 1 {
		return 0
	}
	return p.cfg.DividerSize * float32(count-1)
}

func abs(v float32) float32 {
	return float32(math.Abs(float64(v)))
}
