package state

import "codeburg.org/lexbit/lurpicui/store"

// TrimToMax enforces the MaxRows retention cap by evicting the oldest rows
// (lowest monotonic ids) until the collection fits. It is idempotent at or
// below the cap and is the only removal path in the app, so minID always
// tracks the oldest live row. Callers must be on the runtime thread.
func (a *AppState) TrimToMax() {
	for a.Rows.Len() > a.MaxRows {
		a.Rows.Remove(store.ItemID(a.minID))
		a.minID++
	}
}
