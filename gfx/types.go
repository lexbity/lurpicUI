package gfx

type Point struct {
	X, Y float32
}

type Size struct {
	W, H float32
}

type Rect struct {
	Min, Max Point
}

type Insets struct {
	Top, Right, Bottom, Left float32
}

