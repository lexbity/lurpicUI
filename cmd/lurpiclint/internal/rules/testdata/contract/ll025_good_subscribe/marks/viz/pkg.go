package viz

import (
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/signal"
)

type Chart struct {
	Subs   *signal.Subscriptions
	XScale *reactive.ReactiveScale
}

func (c *Chart) OnAttach() {
	signal.Track(c.Subs, &c.XScale.OnChange, func(signal.Unit) {})
}
