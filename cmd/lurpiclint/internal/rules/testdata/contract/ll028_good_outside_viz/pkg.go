package ll028_good_outside

import (
	"codeburg.org/lexbit/lurpicui/gfx"
)

type Chart struct {
	Color gfx.Color
}

func NewChart() *Chart {
	return &Chart{
		Color: gfx.Color{R: 0.3, G: 0.3, B: 0.3, A: 1},
	}
}
