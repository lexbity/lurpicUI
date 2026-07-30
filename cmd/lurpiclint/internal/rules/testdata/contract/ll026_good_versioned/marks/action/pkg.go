package action

type WidgetItem struct {
	ID   string
	Data string
}

// widgetItemCache mirrors the optionRectCache pattern — versioned, no fire.
type widgetItemCache struct {
	version uint64
	items   []WidgetItem
}

func (c *widgetItemCache) validFor(v uint64) bool {
	return c.version == v
}
