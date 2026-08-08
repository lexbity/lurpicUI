package studio

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/store"
)

// ShellState carries the stores the gallery shell shares across its layout
// arrangements. The responsive contract (FR-resp, F-resp) is that every shell
// sub-tree — the wide 3-pane split and the narrow full-width + overlay tree —
// binds these SAME stores, so a breakpoint crossing preserves store-version
// continuity and value equality without re-parenting a single mark.
type ShellState struct {
	// ActiveExhibit is the exhibit the stage shows. The index pane (nav_rail
	// + tree_navigator in wide, nav_drawer + bottom rail in narrow) writes it;
	// the stage reads it.
	ActiveExhibit *store.ValueStore[ExhibitID]
	// CommandOpen gates the command palette (Ctrl+K / the chrome ⌘K button).
	CommandOpen *store.ValueStore[bool]
	// IndexOpen gates the narrow-mode nav_drawer (the Sources re-host).
	IndexOpen *store.ValueStore[bool]
	// InspectorOpen gates the narrow-mode inspector bottom sheet.
	InspectorOpen *store.ValueStore[bool]
	// Connection reflects the streaming feed's liveness (FR-status).
	Connection *store.ValueStore[bool]
	// RowCount is the badge's text — the selected source's live row count
	// (FR-status), derived from the collection so it stays lock-step.
	RowCount *store.Derived[string]
	// Compact toggles the shell's compact density (the chrome's theme button
	// flips it); the shell's bespoke hosts re-lay out with tighter padding.
	Compact *store.ValueStore[bool]

	// AppState is the flagship's shared store topology (rows, live window).
	AppState *state.AppState
}

// NewShellState builds the shell's shared stores over the app state.
func NewShellState(appState *state.AppState) *ShellState {
	rowCount := store.NewDerived(func() string {
		return fmt.Sprintf("%d rows", appState.Rows.Len())
	}, appState.Rows)
	return &ShellState{
		ActiveExhibit: store.NewValueStore(ExhibitRealtime),
		CommandOpen:   store.NewValueStore(false),
		IndexOpen:     store.NewValueStore(false),
		InspectorOpen: store.NewValueStore(false),
		Connection:    store.NewValueStore(true),
		RowCount:      rowCount,
		Compact:       store.NewValueStore(false),
		AppState:      appState,
	}
}
