package viz

import (
	"codeburg.org/lexbit/lurpicui/gfx"
)

type Chart struct {
	LabelColor gfx.Color
}

func NewChart() *Chart {
	return &Chart{
		LabelColor: gfx.Color{R: 0.3, G: 0.3, B: 0.3, A: 1},
	}
}
