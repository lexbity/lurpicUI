package gfx

type BrushKind uint8

const (
	BrushSolid BrushKind = iota
	BrushLinearGradient
)

type GradientStop struct {
	Offset float32
	Color  Color
}

type Brush struct {
	Kind          BrushKind
	Color         Color
	GradientStart Point
	GradientEnd   Point
	GradientStops []GradientStop
}

func SolidBrush(c Color) Brush {
	return Brush{
		Kind:  BrushSolid,
		Color: c,
	}
}

func LinearGradientBrush(start, end Point, stops []GradientStop) Brush {
	return Brush{
		Kind:          BrushLinearGradient,
		GradientStart: start,
		GradientEnd:   end,
		GradientStops: stops,
	}
}

type LineCap uint8

const (
	LineCapButt LineCap = iota
	LineCapRound
	LineCapSquare
)

type LineJoin uint8

const (
	LineJoinMiter LineJoin = iota
	LineJoinRound
	LineJoinBevel
)

type StrokeStyle struct {
	Width      float32
	Cap        LineCap
	Join       LineJoin
	MiterLimit float32
	Dash       []float32
	DashOffset float32
}

func DefaultStroke(width float32) StrokeStyle {
	return StrokeStyle{
		Width:      width,
		Cap:        LineCapButt,
		Join:       LineJoinMiter,
		MiterLimit: 10,
	}
}
