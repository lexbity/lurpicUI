package store

import (
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/ui_catalog/model"
)

// SelectionStore holds the currently selected entry ID.
var SelectionStore = store.NewValueStore[string]("")

// SelectedEntry returns the currently selected catalog entry, if any.
func SelectedEntry(catalog *model.Catalog) (*model.CatalogEntry, bool) {
	id := SelectionStore.Get()
	if id == "" || catalog == nil {
		return nil, false
	}
	return catalog.GetEntry(id)
}

// SelectEntry sets the selected entry by ID.
func SelectEntry(id string) {
	SelectionStore.Set(id)
}

// ClearSelection clears the current selection.
func ClearSelection() {
	SelectionStore.Set("")
}
