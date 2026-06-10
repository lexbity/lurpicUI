package structure

import (
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func maxFloat(a, b float32) float32 {
	return mathutil.Max(a, b)
}

func materialCommands(path gfx.Path, material theme.Material) []gfx.Command {
	return theme.MaterialCommands(path, material)
}

func isTransparentMaterial(material theme.Material) bool {
	return theme.IsTransparentMaterial(material)
}
