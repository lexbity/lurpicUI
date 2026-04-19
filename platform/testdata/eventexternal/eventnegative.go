//go:build platformnegative

package eventexternal

import "codeburg.org/lexbit/lurpicui/platform"

type externalEvent struct{}

func (externalEvent) isEvent() {}

var _ platform.Event = externalEvent{}
