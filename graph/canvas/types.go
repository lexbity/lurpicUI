package canvas

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/store"
)

// GraphNode is a single node in the graph.
type GraphNode struct {
	ID     store.ItemID
	Bounds gfx.Rect
	Label  string
}

// GraphEdge is a directed edge between two nodes.
type GraphEdge struct {
	ID    store.ItemID
	SrcID store.ItemID
	DstID store.ItemID
}

// ViewportState holds the current pan and zoom.
type ViewportState struct {
	Pan  gfx.Point
	Zoom float32
}
