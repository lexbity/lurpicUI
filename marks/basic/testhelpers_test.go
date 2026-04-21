package basic

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/projection"
)

func renderMark(t *testing.T, mark facet.FacetImpl) []gfx.Command {
	t.Helper()
	system := projection.NewSystem()
	out := system.Run(mark, projection.FrameInfo{})
	if len(out.RenderBatchs) == 0 {
		return nil
	}
	return out.RenderBatchs[0].Commands.Commands
}
