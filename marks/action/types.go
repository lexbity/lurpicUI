package action

// MarkAction carries the key and provenance for action mark Activated signals.
type MarkAction struct {
	Key         string
	Source      string
	ZoomPercent int // populated for zoom events; 0 otherwise
}
