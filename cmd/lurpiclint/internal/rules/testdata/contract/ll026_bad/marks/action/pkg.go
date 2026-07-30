package action

type WidgetItem struct {
	ID   string
	Data string
}

// widgetItemCache has no version field — fires LL026.
type widgetItemCache struct {
	items []WidgetItem
}
