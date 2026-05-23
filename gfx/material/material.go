package material

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Commands converts a theme material into drawable commands for a path.
func Commands(path gfx.Path, material theme.Material) []gfx.Command {
	return theme.MaterialCommands(path, material)
}
