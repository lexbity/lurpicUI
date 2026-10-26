package vulkan

import "codeburg.org/lexbit/lurpicui/gfx"

// encodeStrokeExpanded serializes the gfx-owned stroke expansion directly
// from its caller-owned scratch; command page payloads never escape.
func encodeStrokeExpanded(w *packetWriter, command gfx.CommandView, scratch *gfx.StrokeScratch) error {
	segments := command.ExpandStroke(scratch)
	w.writeU8(packetCmdFillPath)
	w.writePath(gfx.ReadPath(gfx.Path{Segments: segments}))
	return w.writeBrush(command.Brush())
}
