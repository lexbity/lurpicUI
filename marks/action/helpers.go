package action

import (
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func minFloat(a, b float32) float32 {
	return mathutil.Min(a, b)
}

func maxFloat(a, b float32) float32 {
	return mathutil.Max(a, b)
}

func materialCommands(path gfx.Path, material theme.Material) []gfx.Command {
	return theme.MaterialCommands(path, material)
}

func materialColor(material theme.Material) gfx.Color {
	return theme.MaterialColor(material)
}

func isTransparentMaterial(material theme.Material) bool {
	return theme.IsTransparentMaterial(material)
}
