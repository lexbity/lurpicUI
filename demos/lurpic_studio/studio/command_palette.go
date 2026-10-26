package studio

import (
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/store"
)

// ShellCommands builds the shell-wide command registry and the command palette
// mark bound to the shared CommandOpen store (FR-cmd). Registered commands
// switch exhibits, toggle the E1 feed, and open the narrow-mode sheets — each
// mutating observable state so running a command is visibly reactive.
func ShellCommands(scope *store.Scope, shell *ShellState, toggleLive func()) (*runtimepkg.CommandRegistry, *action.CommandPalette) {
	registry := runtimepkg.NewCommandRegistry()

	for _, e := range exhibitCatalog {
		entry := e
		registry.Register(runtimepkg.CommandEntry{
			ID:       "exhibit." + string(entry.id),
			Version:  1,
			Title:    "Open " + entry.title,
			Category: exhibitGroupCatalog,
			Keywords: []string{entry.group, string(entry.id)},
			Execute: func() {
				if shell.ActiveExhibit.Get() != entry.id {
					shell.ActiveExhibit.Set(entry.id)
				}
				shell.CommandOpen.Set(false)
			},
		})
	}
	registry.Register(runtimepkg.CommandEntry{
		ID:       "feed.toggle",
		Version:  1,
		Title:    "Toggle the streaming feed",
		Category: exhibitGroupRealtime,
		Keywords: []string{"live", "stream", "feed"},
		Execute: func() {
			if toggleLive != nil {
				toggleLive()
			}
			shell.CommandOpen.Set(false)
		},
	})
	registry.Register(runtimepkg.CommandEntry{
		ID:       "index.toggle",
		Version:  1,
		Title:    "Toggle the exhibit index",
		Category: exhibitGroupShell,
		Keywords: []string{"sources", "index", "drawer"},
		Execute: func() {
			shell.IndexOpen.Set(!shell.IndexOpen.Get())
			shell.CommandOpen.Set(false)
		},
	})
	registry.Register(runtimepkg.CommandEntry{
		ID:       "inspector.toggle",
		Version:  1,
		Title:    "Toggle the inspector",
		Category: exhibitGroupShell,
		Keywords: []string{"inspector", "details"},
		Execute: func() {
			shell.InspectorOpen.Set(!shell.InspectorOpen.Get())
			shell.CommandOpen.Set(false)
		},
	})

	palette := action.NewCommandPalette(scope, marks.Const("Lurpic Studio commands"), registry, shell.CommandOpen)
	return registry, palette
}
