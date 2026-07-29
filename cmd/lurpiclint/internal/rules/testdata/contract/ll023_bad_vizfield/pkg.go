package ll023_bad_vizfield

import (
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/store"
)

type Chart struct {
	StrokeWidth marks.Binding[float32]
}

func newChart(s *store.ValueStore[float32]) *Chart {
	c := &Chart{}
	c.StrokeWidth = marks.FromStore(s, 0)
	return c
}

func (c *Chart) handleToggle() {
	c.StrokeWidth = marks.Const(float32(2.0))
}
