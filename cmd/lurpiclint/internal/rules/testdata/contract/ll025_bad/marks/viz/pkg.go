package viz

import (
	"codeburg.org/lexbit/lurpicui/scale/reactive"
)

type Chart struct {
	XScale *reactive.ReactiveScale
}

func (c *Chart) OnAttach() {
	// Missing signal.Track — fires LL025
}
